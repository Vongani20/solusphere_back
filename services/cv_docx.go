package services

import (
	"archive/zip"
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"io"
	"log"
	"strings"

	"solusphere_backend/models"
)

// The SoluGrowth CV Master Template (docs/CV Master Template.docx). The Word
// download fills this template in place so the output matches the official
// branded layout exactly (header logo, icons, table layout, footer).
//
//go:embed assets/cv_master_template.docx
var cvMasterTemplateDocx []byte

// Aspect ratio (cx/cy in EMU) of the photo frame in the master template.
const cvDocxPhotoRatio = 1098550.0 / 1359535.0

// GenerateWord fills the embedded CV Master Template with the profile data
// and returns the resulting .docx bytes.
func (s *CVPDFService) GenerateWord(profile *models.CVProfile) ([]byte, error) {
	if profile == nil {
		return nil, fmt.Errorf("profile is required")
	}
	profile = FitCVProfileForTemplate(profile)

	reader, err := zip.NewReader(bytes.NewReader(cvMasterTemplateDocx), int64(len(cvMasterTemplateDocx)))
	if err != nil {
		return nil, fmt.Errorf("open CV template: %w", err)
	}

	var docXML string
	for _, f := range reader.File {
		if f.Name != "word/document.xml" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("open template document.xml: %w", err)
		}
		raw, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("read template document.xml: %w", err)
		}
		docXML = string(raw)
	}
	if docXML == "" {
		return nil, fmt.Errorf("CV template is missing word/document.xml")
	}

	docXML, err = fillCVDocumentXML(docXML, profile)
	if err != nil {
		return nil, err
	}

	photoPNG := cvDocxPhotoPNG(profile.ProfilePhotoURL)

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range reader.File {
		header := f.FileHeader
		w, err := zw.CreateHeader(&header)
		if err != nil {
			return nil, fmt.Errorf("write docx entry %s: %w", f.Name, err)
		}
		switch {
		case f.Name == "word/document.xml":
			if _, err := w.Write([]byte(docXML)); err != nil {
				return nil, fmt.Errorf("write document.xml: %w", err)
			}
		case f.Name == "word/media/image7.png" && photoPNG != nil:
			if _, err := w.Write(photoPNG); err != nil {
				return nil, fmt.Errorf("write photo: %w", err)
			}
		default:
			rc, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("copy docx entry %s: %w", f.Name, err)
			}
			if _, err := io.Copy(w, rc); err != nil {
				rc.Close()
				return nil, fmt.Errorf("copy docx entry %s: %w", f.Name, err)
			}
			rc.Close()
		}
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("finalize docx: %w", err)
	}
	return buf.Bytes(), nil
}

// fillCVDocumentXML performs the placeholder substitutions inside the
// template's document.xml. Anchors are the literal run texts of the template.
//
// Edits are applied bottom-up (experience first, gender last). All user
// content is therefore always inserted at a position AFTER any anchor still
// to be resolved, so anchor searches (which take the first occurrence from
// the top) can never be hijacked by user-provided text that happens to equal
// a template placeholder.
func fillCVDocumentXML(doc string, profile *models.CVProfile) (string, error) {
	// Item paragraph templates, extracted from the pristine document.
	bulletTemplate, err := extractParagraph(doc, ">Microsoft Office<")
	if err != nil {
		return "", err
	}
	plainTemplate, err := extractParagraph(doc, ">English<")
	if err != nil {
		return "", err
	}
	bulletItem := func(text string) string {
		return stripParaIDs(strings.Replace(bulletTemplate, ">Microsoft Office<", ">"+xmlEscapeCV(text)+"<", 1))
	}
	plainItem := func(text string) string {
		return stripParaIDs(strings.Replace(plainTemplate, ">English<", ">"+xmlEscapeCV(text)+"<", 1))
	}
	detailItem := func(text string) string {
		item := plainItem("- " + text)
		return strings.Replace(item, `<w:ind w:firstLine="29"/>`, `<w:ind w:left="602"/>`, 1)
	}

	// ---- Experience (bottom of the document) ----
	doc, err = fillCVExperience(doc, profile.Experience)
	if err != nil {
		return "", err
	}

	// ---- Profile + value proposition (left column, page 1) ----
	headStart, headEnd, err := paragraphBounds(doc, ">PROFILE<", 0)
	if err != nil {
		return "", err
	}
	bodyStart, bodyEnd, err := nextParagraph(doc, headEnd)
	if err != nil {
		return "", err
	}
	headingPara := doc[headStart:headEnd]
	emptyBodyPara := doc[bodyStart:bodyEnd]

	profilePara := injectRunBeforeClose(emptyBodyPara,
		cvBodyRun(orCVPlaceholder(profile.ProfileText, "[Insert profile]")))
	vpHeading := stripParaIDs(strings.Replace(headingPara, ">PROFILE<", ">VALUE PROPOSITION<", 1))
	vpPara := stripParaIDs(injectRunBeforeClose(emptyBodyPara,
		cvBodyRun(orCVPlaceholder(profile.ValueProposition, "[Insert value proposition]"))))
	doc = doc[:bodyStart] + profilePara + vpHeading + vpPara + doc[bodyEnd:]

	// ---- Candidate name ----
	name := strings.TrimSpace(profile.FirstName + " " + profile.LastName)
	if name == "" {
		name = "[First Name] [Surname]"
	}
	doc = strings.Replace(doc, ">Name of Candidate<", ">"+xmlEscapeCV(name)+"<", 1)

	// ---- Languages (template already has one entry: English) ----
	languages := profile.Languages
	if len(languages) == 0 {
		languages = []string{"[Insert language(s)]"}
	}
	langStart, langEnd, err := paragraphBounds(doc, ">English<", 0)
	if err != nil {
		return "", err
	}
	var langParas strings.Builder
	for _, item := range languages {
		langParas.WriteString(plainItem(item))
	}
	doc = doc[:langStart] + langParas.String() + doc[langEnd:]

	// ---- Professional membership ----
	doc, err = insertAfterParagraph(doc, ">PROFESSIONAL MEMBERSHIP",
		bulletItems(bulletItem, profile.ProfessionalMemberships, "[Insert memberships]"))
	if err != nil {
		return "", err
	}

	// ---- Computer skills (template already has one bullet: Microsoft Office) ----
	computerSkills := profile.ComputerSkills
	if len(computerSkills) == 0 {
		computerSkills = []string{"[Insert skills]"}
	}
	csStart, csEnd, err := paragraphBounds(doc, ">Microsoft Office<", 0)
	if err != nil {
		return "", err
	}
	var csParas strings.Builder
	for _, item := range computerSkills {
		csParas.WriteString(bulletItem(item))
	}
	doc = doc[:csStart] + csParas.String() + doc[csEnd:]

	// ---- Qualifications ----
	doc, err = insertAfterParagraph(doc, ">QUALIFICATIONS AND TRAINING",
		bulletItems(bulletItem, profile.Qualifications, "[Insert qualification]"))
	if err != nil {
		return "", err
	}

	// ---- Professional skills ----
	var skillParas strings.Builder
	if len(profile.ProfessionalSkills) == 0 {
		skillParas.WriteString(bulletItem("[Insert skills]"))
	}
	for _, skill := range profile.ProfessionalSkills {
		skillParas.WriteString(bulletItem(orCVPlaceholder(skill.Skill, "[Insert skill]")))
		for _, detail := range skill.Details {
			skillParas.WriteString(detailItem(detail))
		}
	}
	doc, err = insertAfterParagraph(doc, ">PROFESSIONAL SKILLS<", skillParas.String())
	if err != nil {
		return "", err
	}

	// ---- Personal details: nationality + date of birth ----
	natStart, natEnd, err := paragraphBounds(doc, "Nationality:", 0)
	if err != nil {
		return "", err
	}
	natPara := strings.Replace(doc[natStart:natEnd], ">South African<",
		">"+xmlEscapeCV(orCVPlaceholder(profile.Nationality, "[Insert nationality]"))+"<", 1)
	dobPara := strings.Replace(natPara, ">Nationality:<", ">Date of Birth:<", 1)
	dobPara = replaceLastRunText(dobPara, xmlEscapeCV(orCVPlaceholder(formatCVDate(profile.DateOfBirth), "dd Month yyyy")))
	doc = doc[:natStart] + natPara + stripParaIDs(dobPara) + doc[natEnd:]

	// ---- Personal details: gender (value split across two runs: "M"+"ale") ----
	return replaceWithinParagraph(doc, "Gender:", func(para string) string {
		gender := xmlEscapeCV(orCVPlaceholder(profile.Gender, "[Insert gender]"))
		para = strings.Replace(para, ">M</w:t>", ">"+gender+"</w:t>", 1)
		para = strings.Replace(para, ">ale</w:t>", "></w:t>", 1)
		return para
	})
}

// fillCVExperience replaces the template's two example experience blocks with
// one generated block per experience entry.
func fillCVExperience(doc string, entries []models.CVExperience) (string, error) {
	companyT, err := extractParagraph(doc, ">Company:<")
	if err != nil {
		return "", err
	}
	positionT, err := extractParagraph(doc, ">Position: <")
	if err != nil {
		return "", err
	}
	periodT, err := extractParagraph(doc, ">Period: <")
	if err != nil {
		return "", err
	}
	scopeHeadT, err := extractParagraph(doc, ">Scope of work:<")
	if err != nil {
		return "", err
	}

	// Spacer between period and scope heading, and the empty list paragraph
	// used for scope-of-work lines.
	_, periodEnd, err := paragraphBounds(doc, ">Period: <", 0)
	if err != nil {
		return "", err
	}
	spacerStart, spacerEnd, err := nextParagraph(doc, periodEnd)
	if err != nil {
		return "", err
	}
	spacerT := doc[spacerStart:spacerEnd]

	_, scopeHeadEnd, err := paragraphBounds(doc, ">Scope of work:<", 0)
	if err != nil {
		return "", err
	}
	scopeItemStart, scopeItemEnd, err := nextParagraph(doc, scopeHeadEnd)
	if err != nil {
		return "", err
	}
	scopeItemT := doc[scopeItemStart:scopeItemEnd]

	// Replacement range: from the first Company paragraph through the trailing
	// empty paragraphs after the last example block.
	blockStart, _, err := paragraphBounds(doc, ">Company:<", 0)
	if err != nil {
		return "", err
	}
	lastScope := strings.LastIndex(doc, ">Scope of work:<")
	_, rangeEnd, err := paragraphBounds(doc, ">Scope of work:<", lastScope-1)
	if err != nil {
		return "", err
	}
	for {
		start, end, err := nextParagraph(doc, rangeEnd)
		if err != nil || strings.TrimSpace(doc[rangeEnd:start]) != "" {
			break
		}
		para := doc[start:end]
		if hasVisibleText(para) {
			break
		}
		rangeEnd = end
	}

	if len(entries) == 0 {
		entries = []models.CVExperience{{}}
	}

	var blocks strings.Builder
	for i, exp := range entries {
		company := orCVPlaceholder(exp.Company, "[Insert company]")
		position := orCVPlaceholder(exp.Position, "[Insert position]")
		period := orCVPlaceholder(formatCVPeriod(exp.PeriodStart, exp.PeriodEnd), "[Month yyyy - Month yyyy]")

		blocks.WriteString(stripParaIDs(strings.Replace(companyT, ">Name<", ">"+xmlEscapeCV(company)+"<", 1)))
		blocks.WriteString(stripParaIDs(strings.Replace(positionT, ">Job Title<", ">"+xmlEscapeCV(position)+"<", 1)))
		blocks.WriteString(stripParaIDs(strings.Replace(periodT, ">April 2025 - Current<", ">"+xmlEscapeCV(period)+"<", 1)))
		blocks.WriteString(stripParaIDs(spacerT))
		blocks.WriteString(stripParaIDs(scopeHeadT))

		scopes := exp.ScopeOfWork
		if len(scopes) == 0 {
			scopes = []string{"[Insert scope of work]"}
		}
		for _, scope := range scopes {
			blocks.WriteString(stripParaIDs(injectRunBeforeClose(scopeItemT, cvScopeRun("- "+scope))))
		}
		if i < len(entries)-1 {
			blocks.WriteString(stripParaIDs(spacerT))
		}
	}

	return doc[:blockStart] + blocks.String() + doc[rangeEnd:], nil
}

// ---------- photo ----------

// cvDocxPhotoPNG downloads the profile photo and re-encodes it as a PNG
// center-cropped to the template photo frame's aspect ratio. Returns nil when
// no usable photo is available so the template placeholder stays in place.
func cvDocxPhotoPNG(photoURL string) []byte {
	if photoURL == "" {
		return nil
	}
	data, _, err := downloadImage(photoURL)
	if err != nil {
		log.Printf("CV Word: failed to load profile photo %q: %v", photoURL, err)
		return nil
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		log.Printf("CV Word: failed to decode profile photo: %v", err)
		return nil
	}

	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	cropW, cropH := w, h
	if float64(w)/float64(h) > cvDocxPhotoRatio {
		cropW = int(float64(h) * cvDocxPhotoRatio)
	} else {
		cropH = int(float64(w) / cvDocxPhotoRatio)
	}
	x0 := bounds.Min.X + (w-cropW)/2
	y0 := bounds.Min.Y + (h-cropH)/2

	cropped := image.NewRGBA(image.Rect(0, 0, cropW, cropH))
	draw.Draw(cropped, cropped.Bounds(), img, image.Pt(x0, y0), draw.Src)

	var buf bytes.Buffer
	if err := png.Encode(&buf, cropped); err != nil {
		log.Printf("CV Word: failed to encode profile photo: %v", err)
		return nil
	}
	return buf.Bytes()
}

// ---------- XML helpers ----------

// paragraphBounds returns the [start, end) bounds of the <w:p> element that
// contains the first occurrence of marker at or after from.
func paragraphBounds(doc, marker string, from int) (int, int, error) {
	if from < 0 {
		from = 0
	}
	idx := strings.Index(doc[from:], marker)
	if idx < 0 {
		return 0, 0, fmt.Errorf("CV template marker %q not found", marker)
	}
	idx += from

	start := strings.LastIndex(doc[:idx], "<w:p ")
	if alt := strings.LastIndex(doc[:idx], "<w:p>"); alt > start {
		start = alt
	}
	if start < 0 {
		return 0, 0, fmt.Errorf("CV template: no paragraph start before %q", marker)
	}
	end := strings.Index(doc[idx:], "</w:p>")
	if end < 0 {
		return 0, 0, fmt.Errorf("CV template: no paragraph end after %q", marker)
	}
	return start, idx + end + len("</w:p>"), nil
}

// nextParagraph returns the bounds of the first <w:p> element starting at or
// after pos.
func nextParagraph(doc string, pos int) (int, int, error) {
	start := strings.Index(doc[pos:], "<w:p ")
	if alt := strings.Index(doc[pos:], "<w:p>"); alt >= 0 && (start < 0 || alt < start) {
		start = alt
	}
	if start < 0 {
		return 0, 0, fmt.Errorf("CV template: no paragraph after position %d", pos)
	}
	start += pos
	end := strings.Index(doc[start:], "</w:p>")
	if end < 0 {
		return 0, 0, fmt.Errorf("CV template: unterminated paragraph at %d", start)
	}
	return start, start + end + len("</w:p>"), nil
}

func extractParagraph(doc, marker string) (string, error) {
	start, end, err := paragraphBounds(doc, marker, 0)
	if err != nil {
		return "", err
	}
	return doc[start:end], nil
}

func replaceWithinParagraph(doc, marker string, transform func(string) string) (string, error) {
	start, end, err := paragraphBounds(doc, marker, 0)
	if err != nil {
		return "", err
	}
	return doc[:start] + transform(doc[start:end]) + doc[end:], nil
}

func insertAfterParagraph(doc, marker, content string) (string, error) {
	_, end, err := paragraphBounds(doc, marker, 0)
	if err != nil {
		return "", err
	}
	return doc[:end] + content + doc[end:], nil
}

// replaceLastRunText replaces the text of the last <w:t> run in the paragraph.
func replaceLastRunText(para, newText string) string {
	open := strings.LastIndex(para, "<w:t")
	if open < 0 {
		return para
	}
	tagEnd := strings.Index(para[open:], ">")
	close := strings.Index(para[open:], "</w:t>")
	if tagEnd < 0 || close < 0 {
		return para
	}
	return para[:open+tagEnd+1] + newText + para[open+close:]
}

// injectRunBeforeClose appends a run to a paragraph, right before </w:p>.
func injectRunBeforeClose(para, runXML string) string {
	idx := strings.LastIndex(para, "</w:p>")
	if idx < 0 {
		return para
	}
	return para[:idx] + runXML + para[idx:]
}

func cvBodyRun(text string) string {
	return `<w:r><w:rPr><w:rFonts w:ascii="Arial" w:hAnsi="Arial" w:cs="Arial"/><w:sz w:val="20"/></w:rPr>` +
		cvRunTexts(text) + `</w:r>`
}

func cvScopeRun(text string) string {
	return `<w:r><w:rPr><w:rFonts w:ascii="Arial" w:hAnsi="Arial" w:cs="Arial"/><w:sz w:val="16"/><w:szCs w:val="16"/></w:rPr>` +
		cvRunTexts(text) + `</w:r>`
}

// cvRunTexts renders text as <w:t> elements, converting newlines to <w:br/>.
func cvRunTexts(text string) string {
	lines := strings.Split(text, "\n")
	var b strings.Builder
	for i, line := range lines {
		if i > 0 {
			b.WriteString("<w:br/>")
		}
		b.WriteString(`<w:t xml:space="preserve">`)
		b.WriteString(xmlEscapeCV(strings.TrimRight(line, "\r")))
		b.WriteString(`</w:t>`)
	}
	return b.String()
}

func bulletItems(bulletItem func(string) string, items []string, placeholder string) string {
	if len(items) == 0 {
		items = []string{placeholder}
	}
	var b strings.Builder
	for _, item := range items {
		b.WriteString(bulletItem(item))
	}
	return b.String()
}

func hasVisibleText(para string) bool {
	rest := para
	for {
		open := strings.Index(rest, "<w:t")
		if open < 0 {
			return false
		}
		rest = rest[open:]
		tagEnd := strings.Index(rest, ">")
		close := strings.Index(rest, "</w:t>")
		if tagEnd < 0 || close < 0 {
			return false
		}
		if strings.TrimSpace(rest[tagEnd+1:close]) != "" {
			return true
		}
		rest = rest[close+len("</w:t>"):]
	}
}

// stripParaIDs removes w14:paraId/w14:textId attributes from cloned
// paragraphs so the document does not contain duplicate paragraph IDs.
func stripParaIDs(para string) string {
	for _, attr := range []string{` w14:paraId="`, ` w14:textId="`} {
		for {
			start := strings.Index(para, attr)
			if start < 0 {
				break
			}
			end := strings.Index(para[start+len(attr):], `"`)
			if end < 0 {
				break
			}
			para = para[:start] + para[start+len(attr)+end+1:]
		}
	}
	return para
}

var cvXMLEscaper = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
	`"`, "&quot;",
	"'", "&apos;",
)

func xmlEscapeCV(s string) string {
	return cvXMLEscaper.Replace(s)
}

func orCVPlaceholder(value, placeholder string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return placeholder
	}
	return value
}
