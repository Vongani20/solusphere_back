package services

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
	"solusphere_backend/models"
)

// Page layout constants (mm, A4 = 210 x 297)
const (
	cvPageW = 210.0
	cvPageH = 297.0
	cvML    = 12.0
	cvMR    = 12.0
	cvMT    = 10.0
	cvCol1X = cvML
	cvCol1W = 63.0
	cvGap   = 5.0
	cvCol2X = cvCol1X + cvCol1W + cvGap
	cvCol2W = cvPageW - cvMR - cvCol2X
)

// Brand colours aligned with the SoluGrowth CV template.
const (
	cvTealR   = 0
	cvTealG   = 151
	cvTealB   = 167
	cvPurpleR = 75
	cvPurpleG = 0
	cvPurpleB = 130
	cvIconR   = 51
	cvIconG   = 102
	cvIconB   = 153
	cvSideR   = 236
	cvSideG   = 236
	cvSideB   = 236
)

// CVPDFService generates branded SoluGrowth CVs.
type CVPDFService struct{}

func NewCVPDFService() *CVPDFService {
	return &CVPDFService{}
}

func (s *CVPDFService) GeneratePDF(profile *models.CVProfile) (out []byte, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			log.Printf("CV PDF panic: %v", rec)
			out = nil
			err = fmt.Errorf("PDF generation panicked: %v", rec)
		}
	}()

	if profile == nil {
		return nil, fmt.Errorf("profile is required")
	}
	// Export the full CV — do not truncate fields for template fit.
	models.SanitizeCVProfile(profile)
	// gofpdf Helvetica is WinAnsi/Latin-1 only; Unicode (curly quotes, etc.) panics.
	pdfSafeProfile(profile)

	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(cvML, cvMT, cvMR)
	pdf.SetAutoPageBreak(false, 0)
	pdf.AliasNbPages("")

	pdf.AddPage()
	cursor := s.drawPage1(pdf, profile)
	for safety := 0; cursor.section != cvSectionDone && safety < 40; safety++ {
		pdf.AddPage()
		cursor = s.drawSidebarContinuation(pdf, profile, cursor)
	}

	pdf.AddPage()
	s.drawExperiencePages(pdf, profile)

	total := pdf.PageCount()
	for page := 1; page <= total; page++ {
		pdf.SetPage(page)
		s.drawPageFooterLine(pdf)
		s.drawFooter(pdf, page, total)
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("PDF generation failed: %w", err)
	}
	return buf.Bytes(), nil
}

const cvContentBottom = cvPageH - 16.0

type cvSidebarSection int

const (
	cvSectionSkills cvSidebarSection = iota
	cvSectionQualifications
	cvSectionComputer
	cvSectionMemberships
	cvSectionLanguages
	cvSectionDone
)

type cvSidebarCursor struct {
	section cvSidebarSection
	index   int
}

func (s *CVPDFService) drawPage1(pdf *fpdf.Fpdf, profile *models.CVProfile) cvSidebarCursor {
	logoW, logoH := 52.0, 18.0
	logoX := cvPageW - cvMR - logoW
	s.drawBrandHeader(pdf, logoX, cvMT, logoW, logoH)

	pdf.SetXY(cvML, cvMT)
	pdf.SetFont("Helvetica", "B", 17)
	setCV_Purple(pdf)
	name := strings.TrimSpace(profile.FirstName + " " + profile.LastName)
	if name == "" {
		name = "[First Name] [Surname]"
	}
	availW := logoX - cvML - 2
	pdf.CellFormat(availW, 8, name, "", 1, "L", false, 0, "")
	pdf.SetX(cvML)
	pdf.SetFont("Helvetica", "", 8)
	setCV_Teal(pdf)
	pdf.CellFormat(availW, 5, "CURRICULUM VITAE", "", 1, "L", false, 0, "")
	setCV_Black(pdf)

	contentTopY := cvMT + 16
	leftY := contentTopY

	photoH := 52.0
	s.drawPhoto(pdf, cvCol1X, leftY, cvCol1W, photoH, profile.ProfilePhotoURL)
	leftY += photoH + 5

	leftY = s.drawLeftWrappedSection(pdf, "PROFILE", profile.ProfileText, "[Insert profile summary].", cvCol1X, leftY, cvCol1W, cvContentBottom-8)
	leftY += 3
	s.drawLeftWrappedSection(pdf, "VALUE PROPOSITION", profile.ValueProposition, "[Insert value proposition. Describe how you apply your skills to produce outcomes or solve problems.]", cvCol1X, leftY, cvCol1W, cvContentBottom-8)

	boxTop := contentTopY
	boxH := cvContentBottom - boxTop - 2
	pdf.SetFillColor(cvSideR, cvSideG, cvSideB)
	pdf.Rect(cvCol2X, boxTop, cvCol2W, boxH, "F")

	innerX := cvCol2X + 4
	innerW := cvCol2W - 8
	rightY := boxTop + 4
	maxY := boxTop + boxH - 2

	rightY = s.drawRightHeading(pdf, "PERSONAL DETAILS", "P", innerX, rightY, innerW)
	rightY = s.drawPersonalDetails(pdf, profile, innerX, rightY, innerW)
	rightY += 3

	return s.drawSidebarFrom(pdf, profile, cvSidebarCursor{section: cvSectionSkills}, innerX, rightY, innerW, maxY)
}

func (s *CVPDFService) drawSidebarContinuation(pdf *fpdf.Fpdf, profile *models.CVProfile, start cvSidebarCursor) cvSidebarCursor {
	logoW, logoH := 52.0, 18.0
	logoX := cvPageW - cvMR - logoW
	s.drawBrandHeader(pdf, logoX, cvMT, logoW, logoH)

	boxTop := cvMT + 8
	boxH := cvContentBottom - boxTop - 2
	pdf.SetFillColor(cvSideR, cvSideG, cvSideB)
	pdf.Rect(cvCol2X, boxTop, cvCol2W, boxH, "F")

	innerX := cvCol2X + 4
	innerW := cvCol2W - 8
	rightY := boxTop + 4
	maxY := boxTop + boxH - 2

	pdf.SetXY(innerX, rightY)
	pdf.SetFont("Helvetica", "I", 7.5)
	setCV_Gray(pdf)
	pdf.CellFormat(innerW, 4, "Continued...", "", 1, "L", false, 0, "")
	setCV_Black(pdf)
	rightY += 5

	return s.drawSidebarFrom(pdf, profile, start, innerX, rightY, innerW, maxY)
}

func (s *CVPDFService) drawSidebarFrom(pdf *fpdf.Fpdf, profile *models.CVProfile, start cvSidebarCursor, x, y, w, maxY float64) cvSidebarCursor {
	type sectionDef struct {
		id    cvSidebarSection
		title string
		icon  string
		draw  func(startIdx int, yy float64) (float64, int, bool)
	}

	sections := []sectionDef{
		{cvSectionSkills, "PROFESSIONAL SKILLS", "S", func(startIdx int, yy float64) (float64, int, bool) {
			return s.drawSkillsList(pdf, profile.ProfessionalSkills, x, yy, w, maxY, startIdx)
		}},
		{cvSectionQualifications, "QUALIFICATIONS AND TRAINING", "Q", func(startIdx int, yy float64) (float64, int, bool) {
			return s.drawBulletList(pdf, profile.Qualifications, "[Insert qualification]", x, yy, w, maxY, startIdx)
		}},
		{cvSectionComputer, "COMPUTER SKILLS", "C", func(startIdx int, yy float64) (float64, int, bool) {
			return s.drawBulletList(pdf, profile.ComputerSkills, "[Insert skills]", x, yy, w, maxY, startIdx)
		}},
		{cvSectionMemberships, "PROFESSIONAL MEMBERSHIP", "M", func(startIdx int, yy float64) (float64, int, bool) {
			return s.drawBulletList(pdf, profile.ProfessionalMemberships, "[Insert memberships]", x, yy, w, maxY, startIdx)
		}},
		{cvSectionLanguages, "LANGUAGES", "L", func(startIdx int, yy float64) (float64, int, bool) {
			return s.drawBulletList(pdf, profile.Languages, "[Insert language(s)]", x, yy, w, maxY, startIdx)
		}},
	}

	active := false
	for _, section := range sections {
		if section.id == start.section {
			active = true
		}
		if !active {
			continue
		}

		startIdx := 0
		if section.id == start.section {
			startIdx = start.index
		}
		if y+8 > maxY {
			return cvSidebarCursor{section: section.id, index: startIdx}
		}

		// Only redraw heading when starting the section (or continuing with remaining items).
		if startIdx == 0 {
			y = s.drawRightHeading(pdf, section.title, section.icon, x, y, w)
		} else {
			pdf.SetXY(x, y)
			pdf.SetFont("Helvetica", "I", 7)
			setCV_Gray(pdf)
			pdf.CellFormat(w, 4, section.title+" (continued)", "", 1, "L", false, 0, "")
			setCV_Black(pdf)
			y += 4
		}

		nextY, nextIdx, done := section.draw(startIdx, y)
		if !done {
			return cvSidebarCursor{section: section.id, index: nextIdx}
		}
		y = nextY + 3
		if y > maxY {
			next := section.id + 1
			if next >= cvSectionDone {
				return cvSidebarCursor{section: cvSectionDone}
			}
			return cvSidebarCursor{section: next, index: 0}
		}
	}
	return cvSidebarCursor{section: cvSectionDone}
}

func (s *CVPDFService) drawLeftWrappedSection(pdf *fpdf.Fpdf, title, text, placeholder string, x, y, w, maxY float64) float64 {
	pdf.SetXY(x, y)
	s.drawLeftHeading(pdf, title, w)
	y = pdf.GetY() + 1
	if strings.TrimSpace(text) == "" {
		text = placeholder
	}
	pdf.SetFont("Helvetica", "", 7.5)
	setCV_Black(pdf)
	lines := pdf.SplitText(pdfSafeText(text), w)
	lineH := 4.0
	for _, line := range lines {
		if y+lineH > maxY {
			break
		}
		pdf.SetXY(x, y)
		pdf.CellFormat(w, lineH, line, "", 1, "L", false, 0, "")
		y += lineH
	}
	return y
}

func (s *CVPDFService) drawExperiencePages(pdf *fpdf.Fpdf, profile *models.CVProfile) {
	s.drawExperienceHeader(pdf)
	y := cvMT + 9
	setCV_Black(pdf)

	if len(profile.Experience) == 0 {
		pdf.SetXY(cvML, y)
		pdf.SetFont("Helvetica", "", 8)
		setCV_Gray(pdf)
		pdf.CellFormat(180, 5, "[No experience entries added yet]", "", 1, "L", false, 0, "")
		setCV_Black(pdf)
		return
	}

	for i, exp := range profile.Experience {
		needed := 20.0
		if y+needed > cvContentBottom {
			pdf.AddPage()
			s.drawExperienceHeader(pdf)
			y = cvMT + 9
		}

		company := exp.Company
		if company == "" {
			company = "[Insert company]"
		}
		position := exp.Position
		if position == "" {
			position = "[Insert position]"
		}
		period := formatCVPeriod(exp.PeriodStart, exp.PeriodEnd)
		if period == "" {
			period = "[Month yyyy - Month yyyy]"
		}

		y = s.drawLabeledWrapped(pdf, "Company:", company, cvML, y, 28, cvPageW-cvML-cvMR-28)
		y = s.drawLabeledWrapped(pdf, "Position:", position, cvML, y, 28, cvPageW-cvML-cvMR-28)
		y = s.drawLabeledWrapped(pdf, "Period:", period, cvML, y, 28, cvPageW-cvML-cvMR-28)

		scopes := exp.ScopeOfWork
		if len(scopes) == 0 {
			scopes = []string{"[Insert concise points that describe what the user did in the role]"}
		}

		if y+6 > cvContentBottom {
			pdf.AddPage()
			s.drawExperienceHeader(pdf)
			y = cvMT + 9
		}
		pdf.SetXY(cvML, y)
		pdf.SetFont("Helvetica", "B", 8.5)
		setCV_Teal(pdf)
		pdf.CellFormat(180, 5, "Scope of work:", "", 1, "L", false, 0, "")
		y += 5
		setCV_Black(pdf)

		for _, scope := range scopes {
			scope = strings.TrimSpace(scope)
			if scope == "" {
				continue
			}
			text := pdfSafeText("- " + scope)
			pdf.SetFont("Helvetica", "", 8)
			lines := pdf.SplitText(text, 172)
			need := float64(len(lines)) * 4.5
			if y+need > cvContentBottom {
				pdf.AddPage()
				s.drawExperienceHeader(pdf)
				y = cvMT + 9
				pdf.SetFont("Helvetica", "I", 7.5)
				setCV_Gray(pdf)
				pdf.SetXY(cvML, y)
				pdf.CellFormat(180, 4, "Scope of work (continued):", "", 1, "L", false, 0, "")
				y += 5
				setCV_Black(pdf)
				pdf.SetFont("Helvetica", "", 8)
			}
			for _, line := range lines {
				pdf.SetXY(cvML+4, y)
				pdf.CellFormat(172, 4.5, line, "", 1, "L", false, 0, "")
				y += 4.5
			}
		}

		if i < len(profile.Experience)-1 {
			y += 2
			if y+4 > cvContentBottom {
				pdf.AddPage()
				s.drawExperienceHeader(pdf)
				y = cvMT + 9
			} else {
				pdf.SetDrawColor(180, 180, 180)
				pdf.SetLineWidth(0.2)
				pdf.Line(cvML, y, cvPageW-cvMR, y)
				y += 4
			}
		} else {
			y += 4
		}
	}
}

func (s *CVPDFService) drawExperienceHeader(pdf *fpdf.Fpdf) {
	logoW, logoH := 52.0, 18.0
	logoX := cvPageW - cvMR - logoW
	s.drawBrandHeader(pdf, logoX, cvMT, logoW, logoH)

	pdf.SetXY(cvML, cvMT)
	pdf.SetFont("Helvetica", "B", 11)
	setCV_Teal(pdf)
	pdf.CellFormat(180, 7, "EXPERIENCE", "", 1, "L", false, 0, "")
	setCV_Black(pdf)
}

func (s *CVPDFService) drawLabeledWrapped(pdf *fpdf.Fpdf, label, value string, x, y, labelW, valueW float64) float64 {
	pdf.SetXY(x, y)
	pdf.SetFont("Helvetica", "B", 8.5)
	setCV_Black(pdf)
	pdf.CellFormat(labelW, 5, label, "", 0, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 8.5)
	lines := pdf.SplitText(pdfSafeText(value), valueW)
	if len(lines) == 0 {
		lines = []string{""}
	}
	for i, line := range lines {
		if i == 0 {
			pdf.SetXY(x+labelW, y)
		} else {
			y += 5
			if y+5 > cvContentBottom {
				pdf.AddPage()
				s.drawExperienceHeader(pdf)
				y = cvMT + 9
			}
			pdf.SetXY(x+labelW, y)
		}
		pdf.CellFormat(valueW, 5, line, "", 1, "L", false, 0, "")
	}
	return y + 5
}

func (s *CVPDFService) drawPageFooterLine(pdf *fpdf.Fpdf) {
	pdf.SetDrawColor(0, 0, 0)
	pdf.SetLineWidth(0.3)
	pdf.Line(cvML, cvPageH-12, cvPageW-cvMR, cvPageH-12)
}

func (s *CVPDFService) drawBrandHeader(pdf *fpdf.Fpdf, x, y, w, h float64) {
	if len(soluGrowthLogoPNG) > 0 {
		opts := fpdf.ImageOptions{ImageType: "PNG", ReadDpi: false}
		name := fmt.Sprintf("solugrowth-logo-%d", time.Now().UnixNano())
		pdf.RegisterImageOptionsReader(name, opts, bytes.NewReader(soluGrowthLogoPNG))
		pdf.ImageOptions(name, x, y, w, h, false, opts, 0, "")
		return
	}

	pdf.SetFont("Helvetica", "B", 10)
	setCV_Teal(pdf)
	pdf.SetXY(x, y+2)
	pdf.CellFormat(w, 5, "SoluGrowth", "", 1, "R", false, 0, "")
	pdf.SetX(x)
	pdf.SetFont("Helvetica", "", 6.5)
	setCV_Gray(pdf)
	pdf.CellFormat(w, 4, "BPO - ITO - KPO", "", 0, "R", false, 0, "")
	setCV_Black(pdf)
}

func (s *CVPDFService) drawPhoto(pdf *fpdf.Fpdf, x, y, w, h float64, photoURL string) {
	shadowOffset := 1.5
	pdf.SetFillColor(210, 210, 210)
	pdf.Rect(x+shadowOffset, y+shadowOffset, w, h, "F")

	drawn := false
	if photoURL != "" {
		func() {
			defer func() {
				if rec := recover(); rec != nil {
					log.Printf("CV PDF: photo render panic for %q: %v", photoURL, rec)
					drawn = false
				}
			}()
			imgData, imgType, err := downloadImage(photoURL)
			if err != nil {
				log.Printf("CV PDF: failed to load profile photo %q: %v", photoURL, err)
				return
			}
			name := fmt.Sprintf("cv-photo-%d", time.Now().UnixNano())
			opts := fpdf.ImageOptions{ImageType: imgType}
			info := pdf.RegisterImageOptionsReader(name, opts, bytes.NewReader(imgData))

			// Fit the whole photo inside the frame without cropping or stretching.
			drawX, drawY, drawW, drawH := x, y, w, h
			if info != nil && info.Width() > 0 && info.Height() > 0 {
				scale := math.Min(w/info.Width(), h/info.Height())
				drawW = info.Width() * scale
				drawH = info.Height() * scale
				drawX = x + (w-drawW)/2
				drawY = y + (h-drawH)/2
			}

			pdf.SetFillColor(255, 255, 255)
			pdf.Rect(x, y, w, h, "F")
			pdf.ImageOptions(name, drawX, drawY, drawW, drawH, false, opts, 0, "")
			pdf.SetDrawColor(220, 220, 220)
			pdf.SetLineWidth(0.2)
			pdf.Rect(x, y, w, h, "D")
			drawn = true
		}()
	}
	if drawn {
		return
	}

	pdf.SetDrawColor(200, 200, 200)
	pdf.SetFillColor(245, 245, 245)
	pdf.Rect(x, y, w, h, "FD")
	pdf.SetFont("Helvetica", "", 7)
	setCV_Gray(pdf)
	pdf.SetXY(x, y+h/2-2)
	pdf.CellFormat(w, 4, "Insert user's image", "", 0, "C", false, 0, "")
	setCV_Black(pdf)
	pdf.SetFillColor(255, 255, 255)
}

func (s *CVPDFService) drawLeftHeading(pdf *fpdf.Fpdf, title string, w float64) {
	pdf.SetFont("Helvetica", "B", 9)
	setCV_Teal(pdf)
	pdf.CellFormat(w, 5, title, "", 1, "L", false, 0, "")
	setCV_Black(pdf)
}

func (s *CVPDFService) drawRightHeading(pdf *fpdf.Fpdf, title, icon string, x, y, w float64) float64 {
	iconSize := 4.5
	pdf.SetFillColor(cvIconR, cvIconG, cvIconB)
	pdf.Circle(x+iconSize/2, y+iconSize/2, iconSize/2, "F")
	pdf.SetFont("Helvetica", "B", 5.5)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetXY(x, y+0.6)
	pdf.CellFormat(iconSize, iconSize, icon, "", 0, "C", false, 0, "")

	textX := x + iconSize + 2
	pdf.SetXY(textX, y)
	pdf.SetFont("Helvetica", "B", 8)
	setCV_Teal(pdf)
	pdf.CellFormat(w-iconSize-2, 5, title, "", 1, "L", false, 0, "")
	setCV_Black(pdf)
	return y + 5
}

func (s *CVPDFService) drawPersonalDetails(pdf *fpdf.Fpdf, profile *models.CVProfile, x, y, w float64) float64 {
	type row struct{ label, value string }
	var rows []row
	if profile.Gender != "" {
		rows = append(rows, row{"Gender:", profile.Gender})
	}
	if profile.Nationality != "" {
		rows = append(rows, row{"Nationality:", profile.Nationality})
	}
	if profile.DateOfBirth != "" {
		rows = append(rows, row{"Date of birth:", formatCVDate(profile.DateOfBirth)})
	}
	if len(rows) == 0 {
		rows = []row{
			{"Gender:", "[Insert gender]"},
			{"Nationality:", "[Insert nationality]"},
			{"Date of birth:", "dd Month yyyy"},
		}
	}

	lineH := 4.5
	labelW := 26.0
	for _, r := range rows {
		pdf.SetXY(x, y)
		pdf.SetFont("Helvetica", "", 7.5)
	setCV_Gray(pdf)
	pdf.CellFormat(labelW, lineH, pdfSafeText(r.label), "", 0, "L", false, 0, "")
	setCV_Black(pdf)
	pdf.CellFormat(w-labelW, lineH, pdfSafeText(r.value), "", 1, "L", false, 0, "")
		y += lineH
	}
	return y
}

func (s *CVPDFService) drawSkillsList(pdf *fpdf.Fpdf, skills []models.ProfessionalSkill, x, y, w, maxY float64, startIdx int) (float64, int, bool) {
	lineH := 4.0
	if len(skills) == 0 {
		if startIdx > 0 {
			return y, 0, true
		}
		nextY := s.drawWrappedLines(pdf, x+2, y, w-2, lineH, "- [Insert skills]", maxY)
		if nextY < 0 {
			return y, 0, false
		}
		return nextY, 0, true
	}
	if startIdx < 0 {
		startIdx = 0
	}
	for i := startIdx; i < len(skills); i++ {
		skill := skills[i]
		label := skill.Skill
		if label == "" {
			label = "[Insert skills]"
		}
		pdf.SetFont("Helvetica", "B", 7.5)
		setCV_Black(pdf)
		nextY := s.drawWrappedLines(pdf, x+2, y, w-2, lineH, "- "+label+":", maxY)
		if nextY < 0 {
			// If the page is still mostly empty, skip this oversized item to avoid a hang.
			if y < maxY-40 {
				continue
			}
			return y, i, false
		}
		y = nextY

		details := skill.Details
		if len(details) == 0 {
			details = []string{"[skill detail point if necessary]"}
		}
		for _, d := range details {
			if strings.TrimSpace(d) == "" {
				continue
			}
			pdf.SetFont("Helvetica", "", 7.5)
			nextY = s.drawWrappedLines(pdf, x+7, y, w-7, lineH, "* "+d, maxY)
			if nextY < 0 {
				return y, i, false
			}
			y = nextY
		}
	}
	return y, len(skills), true
}

func (s *CVPDFService) drawBulletList(pdf *fpdf.Fpdf, items []string, placeholder string, x, y, w, maxY float64, startIdx int) (float64, int, bool) {
	lineH := 4.0
	pdf.SetFont("Helvetica", "", 7.5)
	setCV_Black(pdf)

	var nonEmpty []string
	for _, item := range items {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			nonEmpty = append(nonEmpty, trimmed)
		}
	}
	if len(nonEmpty) == 0 {
		if startIdx > 0 {
			return y, 0, true
		}
		nextY := s.drawWrappedLines(pdf, x+2, y, w-2, lineH, "- "+placeholder, maxY)
		if nextY < 0 {
			return y, 0, false
		}
		return nextY, 0, true
	}
	if startIdx < 0 {
		startIdx = 0
	}
	for i := startIdx; i < len(nonEmpty); i++ {
		nextY := s.drawWrappedLines(pdf, x+2, y, w-2, lineH, "- "+nonEmpty[i], maxY)
		if nextY < 0 {
			if y < maxY-40 {
				continue
			}
			return y, i, false
		}
		y = nextY
	}
	return y, len(nonEmpty), true
}

func (s *CVPDFService) drawWrappedLines(pdf *fpdf.Fpdf, x, y, w, lineH float64, text string, maxY float64) float64 {
	lines := pdf.SplitText(pdfSafeText(text), w)
	if len(lines) == 0 {
		return y
	}
	for _, line := range lines {
		if y+lineH > maxY {
			return -1
		}
		pdf.SetXY(x, y)
		pdf.CellFormat(w, lineH, line, "", 1, "L", false, 0, "")
		y += lineH
	}
	return y
}

func (s *CVPDFService) drawFooter(pdf *fpdf.Fpdf, pageNum, total int) {
	y := cvPageH - 8
	pdf.SetFont("Helvetica", "I", 7)
	setCV_Gray(pdf)
	pdf.SetXY(cvML, y)
	pdf.CellFormat(90, 4, "Private and Confidential", "", 0, "L", false, 0, "")
	pdf.CellFormat(cvPageW-cvML-cvMR-90, 4, fmt.Sprintf("Page %d of %d", pageNum, total), "", 0, "R", false, 0, "")
	setCV_Black(pdf)
}

func setCV_Teal(pdf *fpdf.Fpdf)   { pdf.SetTextColor(cvTealR, cvTealG, cvTealB) }
func setCV_Purple(pdf *fpdf.Fpdf) { pdf.SetTextColor(cvPurpleR, cvPurpleG, cvPurpleB) }
func setCV_Gray(pdf *fpdf.Fpdf)   { pdf.SetTextColor(100, 100, 100) }
func setCV_Black(pdf *fpdf.Fpdf)  { pdf.SetTextColor(0, 0, 0) }

func downloadImage(rawURL string) ([]byte, string, error) {
	if key, ok := models.S3KeyFromObjectURL(rawURL); ok {
		data, contentType, err := models.DownloadFromS3(key)
		if err != nil {
			return nil, "", err
		}
		return data, imageTypeFromSource(rawURL, contentType), nil
	}

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Get(rawURL)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("HTTP %d fetching image", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, "", err
	}
	return data, imageTypeFromSource(rawURL, strings.ToLower(resp.Header.Get("Content-Type"))), nil
}

func imageTypeFromSource(rawURL, contentType string) string {
	if strings.Contains(contentType, "png") {
		return "PNG"
	}
	if strings.Contains(contentType, "jpeg") || strings.Contains(contentType, "jpg") {
		return "JPEG"
	}
	urlLower := strings.ToLower(rawURL)
	if strings.HasSuffix(urlLower, ".png") {
		return "PNG"
	}
	return "JPEG"
}

// pdfSafeText maps text to WinAnsi/Latin-1 so gofpdf Helvetica does not panic on
// Unicode code points (e.g. curly apostrophe U+2019 -> index out of range).
func pdfSafeText(s string) string {
	if s == "" {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case '\n', '\r', '\t':
			b.WriteRune(r)
		case '\u00a0':
			b.WriteByte(' ')
		case '\u2018', '\u2019', '\u201a', '\u2032', '`', '´':
			b.WriteByte('\'')
		case '\u201c', '\u201d', '\u201e', '\u201f', '\u2033':
			b.WriteByte('"')
		case '\u2013', '\u2014', '\u2212':
			b.WriteByte('-')
		case '\u2026':
			b.WriteString("...")
		case '\u2022', '\u25cf', '\u25aa', '\u25e6', '\u00b7', '\u2043':
			b.WriteByte('-')
		default:
			if r < 32 {
				continue
			}
			if r > 255 {
				b.WriteByte('?')
				continue
			}
			b.WriteRune(r)
		}
	}
	return b.String()
}

func pdfSafeProfile(p *models.CVProfile) {
	if p == nil {
		return
	}
	p.FirstName = pdfSafeText(p.FirstName)
	p.LastName = pdfSafeText(p.LastName)
	p.Gender = pdfSafeText(p.Gender)
	p.Nationality = pdfSafeText(p.Nationality)
	p.DateOfBirth = pdfSafeText(p.DateOfBirth)
	p.ProfileText = pdfSafeText(p.ProfileText)
	p.ValueProposition = pdfSafeText(p.ValueProposition)
	for i := range p.ProfessionalSkills {
		p.ProfessionalSkills[i].Skill = pdfSafeText(p.ProfessionalSkills[i].Skill)
		for j := range p.ProfessionalSkills[i].Details {
			p.ProfessionalSkills[i].Details[j] = pdfSafeText(p.ProfessionalSkills[i].Details[j])
		}
	}
	for i := range p.Qualifications {
		p.Qualifications[i] = pdfSafeText(p.Qualifications[i])
	}
	for i := range p.ComputerSkills {
		p.ComputerSkills[i] = pdfSafeText(p.ComputerSkills[i])
	}
	for i := range p.ProfessionalMemberships {
		p.ProfessionalMemberships[i] = pdfSafeText(p.ProfessionalMemberships[i])
	}
	for i := range p.Languages {
		p.Languages[i] = pdfSafeText(p.Languages[i])
	}
	for i := range p.Experience {
		p.Experience[i].Company = pdfSafeText(p.Experience[i].Company)
		p.Experience[i].Position = pdfSafeText(p.Experience[i].Position)
		p.Experience[i].PeriodStart = pdfSafeText(p.Experience[i].PeriodStart)
		p.Experience[i].PeriodEnd = pdfSafeText(p.Experience[i].PeriodEnd)
		for j := range p.Experience[i].ScopeOfWork {
			p.Experience[i].ScopeOfWork[j] = pdfSafeText(p.Experience[i].ScopeOfWork[j])
		}
	}
}

func formatCVDate(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 10 {
		s = s[:10]
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return s
	}
	return t.Format("02 January 2006")
}

func formatCVPeriod(start, end string) string {
	parse := func(s string) string {
		s = strings.TrimSpace(s)
		if s == "" {
			return ""
		}
		if t, err := time.Parse("2006-01", s); err == nil {
			return t.Format("January 2006")
		}
		if len(s) >= 10 {
			s = s[:10]
		}
		if t, err := time.Parse("2006-01-02", s); err == nil {
			return t.Format("January 2006")
		}
		return s
	}
	if start == "" {
		return ""
	}
	endStr := "Present"
	if end != "" {
		endStr = parse(end)
	}
	return parse(start) + " - " + endStr
}
