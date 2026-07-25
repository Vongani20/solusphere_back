package services

import (
	"archive/zip"
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"image/color"
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
	doc = pinCVSidebarTable(doc)

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
		return stripParaIDs(strings.Replace(bulletTemplate, ">Microsoft Office<", ">"+xmlEscapeCV(stripLeadingBullet(text))+"<", 1))
	}
	plainItem := func(text string) string {
		return stripParaIDs(strings.Replace(plainTemplate, ">English<", ">"+xmlEscapeCV(stripLeadingBullet(text))+"<", 1))
	}
	// Skill details stay plain (no Word bullet and no "-" prefix) — only the
	// skill title itself uses a list point, matching the master template.
	detailItem := func(text string) string {
		item := plainItem(text)
		return strings.Replace(item, `<w:ind w:firstLine="29"/>`, `<w:ind w:left="602"/>`, 1)
	}

	// ---- Experience (bottom of the document) ----
	doc, err = fillCVExperience(doc, profile.Experience)
	if err != nil {
		return "", err
	}

	// ---- Profile + value proposition (left column, page 1) ----
	// PROFILE / VALUE PROPOSITION / EXPERIENCE share left indent 567 in the
	// master template. Replace the empty body zone between PROFILE and
	// EXPERIENCE so section headings stay on that same left margin.
	_, headEnd, err := paragraphBounds(doc, ">PROFILE<", 0)
	if err != nil {
		return "", err
	}
	bodyStart, _, err := nextParagraph(doc, headEnd)
	if err != nil {
		return "", err
	}
	expStart, _, err := paragraphBounds(doc, ">EXPERIENCE<", 0)
	if err != nil {
		return "", err
	}
	headingPara, err := extractParagraph(doc, ">PROFILE<")
	if err != nil {
		return "", err
	}
	alignedBody, err := cvAlignedBodyParagraph(doc, bodyStart, expStart)
	if err != nil {
		return "", err
	}

	profilePara := injectRunBeforeClose(alignedBody,
		cvBodyRun(orCVPlaceholder(profile.ProfileText, "[Insert profile]")))
	vpHeading := stripParaIDs(strings.Replace(headingPara, ">PROFILE<", ">VALUE PROPOSITION<", 1))
	vpPara := stripParaIDs(injectRunBeforeClose(alignedBody,
		cvBodyRun(orCVPlaceholder(profile.ValueProposition, "[Insert value proposition]"))))
	doc = doc[:bodyStart] + profilePara + vpHeading + vpPara + doc[expStart:]

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

// pinCVSidebarTable anchors the personal-details sidebar table to the page
// instead of the text flow. With the template's text anchoring, a sidebar tall
// enough not to fit below its anchor point gets pushed down the page (or onto
// page 2), so PERSONAL DETAILS no longer starts at the top. Page anchoring
// keeps the table's top fixed; overflowing rows continue on the next page.
// 3593 twips = top margin (1021) + the template's original text offset (2572).
func pinCVSidebarTable(doc string) string {
	const floating = `w:vertAnchor="text" w:horzAnchor="page" w:tblpX="6751" w:tblpY="2572"`
	const pinned = `w:vertAnchor="page" w:horzAnchor="page" w:tblpX="6751" w:tblpY="3593"`
	return strings.Replace(doc, floating, pinned, 1)
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
	scopeItemT, err := cvScopeItemParagraph(doc, scopeHeadEnd)
	if err != nil {
		return "", err
	}

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
			// Scope paragraphs already carry Word list numbering — do not add "-".
			blocks.WriteString(stripParaIDs(injectRunBeforeClose(scopeItemT, cvScopeRun(stripLeadingBullet(scope)))))
		}
		if i < len(entries)-1 {
			blocks.WriteString(stripParaIDs(spacerT))
		}
	}

	return doc[:blockStart] + blocks.String() + doc[rangeEnd:], nil
}

// cvAlignedBodyParagraph returns a left-column body paragraph template whose
// left indent matches PROFILE / EXPERIENCE (567 twips). Falls back to forcing
// that indent onto the first empty body paragraph in the zone.
func cvAlignedBodyParagraph(doc string, from, to int) (string, error) {
	pos := from
	for pos < to {
		start, end, err := nextParagraph(doc, pos)
		if err != nil || start >= to {
			break
		}
		para := doc[start:end]
		if strings.Contains(para, `w:ind w:left="567"`) && !hasVisibleText(para) {
			return para, nil
		}
		pos = end
	}
	start, end, err := nextParagraph(doc, from)
	if err != nil {
		return "", err
	}
	para := doc[start:end]
	if strings.Contains(para, `<w:ind `) {
		para = strings.Replace(para, `<w:ind `, `<w:ind w:left="567" `, 1)
	} else if strings.Contains(para, `<w:pPr>`) {
		para = strings.Replace(para, `<w:pPr>`, `<w:pPr><w:ind w:left="567"/>`, 1)
	}
	return para, nil
}

// cvScopeItemParagraph returns the first scope-of-work list paragraph after
// from that has a real numbering definition. The master template leaves a
// numId=0 stub immediately under "Scope of work:" — using that stub drops
// the bullet indent and pushes the text onto the wrong left margin.
func cvScopeItemParagraph(doc string, from int) (string, error) {
	pos := from
	for i := 0; i < 6; i++ {
		start, end, err := nextParagraph(doc, pos)
		if err != nil {
			return "", err
		}
		para := doc[start:end]
		if strings.Contains(para, `<w:numPr>`) &&
			!strings.Contains(para, `w:numId w:val="0"`) &&
			!hasVisibleText(para) {
			return para, nil
		}
		pos = end
	}
	return "", fmt.Errorf("CV template: no numbered scope-of-work paragraph found")
}

// ---------- photo ----------

// cvDocxPhotoPNG downloads the profile photo and re-encodes it as a PNG that
// fits the template photo frame without cropping: the whole image is centred
// on a white canvas matching the frame's aspect ratio. Returns nil when no
// usable photo is available so the template placeholder stays in place.
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
	if w <= 0 || h <= 0 {
		return nil
	}

	// Grow the canvas (never crop) so its aspect ratio matches the frame.
	canvasW, canvasH := w, h
	if float64(w)/float64(h) > cvDocxPhotoRatio {
		canvasH = int(float64(w)/cvDocxPhotoRatio + 0.5)
	} else {
		canvasW = int(float64(h)*cvDocxPhotoRatio + 0.5)
	}

	canvas := image.NewRGBA(image.Rect(0, 0, canvasW, canvasH))
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)

	offset := image.Rect((canvasW-w)/2, (canvasH-h)/2, (canvasW-w)/2+w, (canvasH-h)/2+h)
	draw.Draw(canvas, offset, img, bounds.Min, draw.Src)

	var buf bytes.Buffer
	if err := png.Encode(&buf, canvas); err != nil {
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

// stripLeadingBullet removes manual bullet/dash prefixes so Word list
// numbering is the only point marker used in list sections.
func stripLeadingBullet(text string) string {
	text = strings.TrimSpace(text)
	for {
		trimmed := text
		for _, prefix := range []string{"•", "●", "·", "▪", "◦", "-", "–", "—", "*"} {
			if strings.HasPrefix(trimmed, prefix) {
				trimmed = strings.TrimSpace(trimmed[len(prefix):])
			}
		}
		if trimmed == text {
			return text
		}
		text = trimmed
	}
}
