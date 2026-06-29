package services

import (
	"bytes"
	"fmt"
	"io"
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

func (s *CVPDFService) GeneratePDF(profile *models.CVProfile) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(cvML, cvMT, cvMR)
	pdf.SetAutoPageBreak(false, 0)

	pdf.AddPage()
	s.drawPage1(pdf, profile)

	pdf.AddPage()
	s.drawPage2(pdf, profile)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("PDF generation failed: %w", err)
	}
	return buf.Bytes(), nil
}

func (s *CVPDFService) drawPage1(pdf *fpdf.Fpdf, profile *models.CVProfile) {
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

	pdf.SetXY(cvCol1X, leftY)
	s.drawLeftHeading(pdf, "PROFILE", cvCol1W)
	leftY = pdf.GetY() + 1
	pdf.SetXY(cvCol1X, leftY)
	pdf.SetFont("Helvetica", "", 7.5)
	setCV_Black(pdf)
	profileText := profile.ProfileText
	if profileText == "" {
		profileText = "[Insert profile. Should be no longer than 80 words]."
	}
	pdf.MultiCell(cvCol1W, 4, profileText, "", "L", false)
	leftY = pdf.GetY() + 5

	pdf.SetXY(cvCol1X, leftY)
	s.drawLeftHeading(pdf, "VALUE PROPOSITION", cvCol1W)
	leftY = pdf.GetY() + 1
	pdf.SetXY(cvCol1X, leftY)
	pdf.SetFont("Helvetica", "", 7.5)
	setCV_Black(pdf)
	vp := profile.ValueProposition
	if vp == "" {
		vp = "[Insert value proposition. This is how the user applies their skills in a way that produces any outcome or solves a problem. Should be no longer than 150 words]"
	}
	pdf.MultiCell(cvCol1W, 4, vp, "", "L", false)

	boxTop := contentTopY
	footerY := cvPageH - 10.0
	boxH := footerY - boxTop - 2
	pdf.SetFillColor(cvSideR, cvSideG, cvSideB)
	pdf.Rect(cvCol2X, boxTop, cvCol2W, boxH, "F")

	innerX := cvCol2X + 4
	innerW := cvCol2W - 8
	rightY := boxTop + 4

	rightY = s.drawRightHeading(pdf, "PERSONAL DETAILS", "P", innerX, rightY, innerW)
	rightY = s.drawPersonalDetails(pdf, profile, innerX, rightY, innerW)
	rightY += 3

	rightY = s.drawRightHeading(pdf, "PROFESSIONAL SKILLS", "S", innerX, rightY, innerW)
	rightY = s.drawSkillsList(pdf, profile.ProfessionalSkills, innerX, rightY, innerW)
	rightY += 3

	rightY = s.drawRightHeading(pdf, "QUALIFICATIONS AND TRAINING", "Q", innerX, rightY, innerW)
	rightY = s.drawBulletList(pdf, profile.Qualifications, "[Insert qualification]", innerX, rightY, innerW)
	rightY += 3

	rightY = s.drawRightHeading(pdf, "COMPUTER SKILLS", "C", innerX, rightY, innerW)
	rightY = s.drawBulletList(pdf, profile.ComputerSkills, "[Insert skills]", innerX, rightY, innerW)
	rightY += 3

	rightY = s.drawRightHeading(pdf, "PROFESSIONAL MEMBERSHIP", "M", innerX, rightY, innerW)
	rightY = s.drawBulletList(pdf, profile.ProfessionalMemberships, "[Insert memberships]", innerX, rightY, innerW)
	rightY += 3

	rightY = s.drawRightHeading(pdf, "LANGUAGES", "L", innerX, rightY, innerW)
	s.drawBulletList(pdf, profile.Languages, "[Insert language(s)]", innerX, rightY, innerW)

	_ = rightY
	s.drawFooter(pdf, 1, 2)
}

func (s *CVPDFService) drawPage2(pdf *fpdf.Fpdf, profile *models.CVProfile) {
	logoW, logoH := 52.0, 18.0
	logoX := cvPageW - cvMR - logoW
	s.drawBrandHeader(pdf, logoX, cvMT, logoW, logoH)

	y := cvMT
	pdf.SetXY(cvML, y)
	pdf.SetFont("Helvetica", "B", 11)
	setCV_Teal(pdf)
	pdf.CellFormat(180, 7, "EXPERIENCE", "", 1, "L", false, 0, "")
	y += 9
	setCV_Black(pdf)

	if len(profile.Experience) == 0 {
		pdf.SetXY(cvML, y)
		pdf.SetFont("Helvetica", "", 8)
		setCV_Gray(pdf)
		pdf.CellFormat(180, 5, "[No experience entries added yet]", "", 1, "L", false, 0, "")
		setCV_Black(pdf)
	}

	for i, exp := range profile.Experience {
		if y > 268 {
			break
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

		pdf.SetXY(cvML, y)
		pdf.SetFont("Helvetica", "B", 8.5)
		pdf.CellFormat(28, 5, "Company:", "", 0, "L", false, 0, "")
		pdf.CellFormat(152, 5, company, "", 1, "L", false, 0, "")
		y += 5

		pdf.SetXY(cvML, y)
		pdf.CellFormat(28, 5, "Position:", "", 0, "L", false, 0, "")
		pdf.CellFormat(152, 5, position, "", 1, "L", false, 0, "")
		y += 5

		pdf.SetXY(cvML, y)
		pdf.SetFont("Helvetica", "", 8.5)
		pdf.CellFormat(28, 5, "Period:", "", 0, "L", false, 0, "")
		pdf.CellFormat(152, 5, period, "", 1, "L", false, 0, "")
		y += 5

		scopes := exp.ScopeOfWork
		if len(scopes) == 0 {
			scopes = []string{"[Insert concise points that describe what the user did in the role]"}
		}

		pdf.SetXY(cvML, y)
		pdf.SetFont("Helvetica", "B", 8.5)
		setCV_Teal(pdf)
		pdf.CellFormat(180, 5, "Scope of work:", "", 1, "L", false, 0, "")
		y += 5
		setCV_Black(pdf)
		for _, scope := range scopes {
			if y > 270 {
				break
			}
			pdf.SetXY(cvML+4, y)
			pdf.SetFont("Helvetica", "", 8)
			pdf.MultiCell(176-4, 4.5, "- "+scope, "", "L", false)
			y = pdf.GetY()
		}

		if i < len(profile.Experience)-1 {
			y += 2
			pdf.SetDrawColor(180, 180, 180)
			pdf.SetLineWidth(0.2)
			pdf.Line(cvML, y, cvPageW-cvMR, y)
			y += 4
		} else {
			y += 6
		}
	}

	pdf.SetDrawColor(0, 0, 0)
	pdf.SetLineWidth(0.3)
	pdf.Line(cvML, 285, cvPageW-cvMR, 285)

	s.drawFooter(pdf, 2, 2)
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

	if photoURL != "" {
		imgData, imgType, err := downloadImage(photoURL)
		if err == nil {
			name := fmt.Sprintf("cv-photo-%d", time.Now().UnixNano())
			opts := fpdf.ImageOptions{ImageType: imgType}
			pdf.RegisterImageOptionsReader(name, opts, bytes.NewReader(imgData))
			pdf.ImageOptions(name, x, y, w, h, false, opts, 0, "")
			pdf.SetDrawColor(220, 220, 220)
			pdf.SetLineWidth(0.2)
			pdf.Rect(x, y, w, h, "D")
			return
		}
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
		pdf.CellFormat(labelW, lineH, r.label, "", 0, "L", false, 0, "")
		setCV_Black(pdf)
		pdf.CellFormat(w-labelW, lineH, r.value, "", 1, "L", false, 0, "")
		y += lineH
	}
	return y
}

func (s *CVPDFService) drawSkillsList(pdf *fpdf.Fpdf, skills []models.ProfessionalSkill, x, y, w float64) float64 {
	lineH := 4.0
	if len(skills) == 0 {
		pdf.SetXY(x+2, y)
		pdf.SetFont("Helvetica", "", 7.5)
		setCV_Black(pdf)
		pdf.CellFormat(w-2, lineH, "- [Insert skills]", "", 1, "L", false, 0, "")
		return y + lineH
	}
	for _, skill := range skills {
		label := skill.Skill
		if label == "" {
			label = "[Insert skills]"
		}
		pdf.SetXY(x+2, y)
		pdf.SetFont("Helvetica", "B", 7.5)
		setCV_Black(pdf)
		pdf.CellFormat(w-2, lineH, "- "+label+":", "", 1, "L", false, 0, "")
		y += lineH
		details := skill.Details
		if len(details) == 0 {
			details = []string{"[skill detail point if necessary]"}
		}
		for _, d := range details {
			if strings.TrimSpace(d) == "" {
				continue
			}
			pdf.SetXY(x+7, y)
			pdf.SetFont("Helvetica", "", 7.5)
			pdf.CellFormat(w-7, lineH, "* "+d, "", 1, "L", false, 0, "")
			y += lineH
		}
	}
	return y
}

func (s *CVPDFService) drawBulletList(pdf *fpdf.Fpdf, items []string, placeholder string, x, y, w float64) float64 {
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
		pdf.SetXY(x+2, y)
		pdf.CellFormat(w-2, lineH, "- "+placeholder, "", 1, "L", false, 0, "")
		return y + lineH
	}
	for _, item := range nonEmpty {
		pdf.SetXY(x+2, y)
		pdf.CellFormat(w-2, lineH, "- "+item, "", 1, "L", false, 0, "")
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

func downloadImage(url string) ([]byte, string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("HTTP %d fetching image", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}
	imgType := "JPEG"
	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	urlLower := strings.ToLower(url)
	if strings.Contains(ct, "png") || strings.HasSuffix(urlLower, ".png") {
		imgType = "PNG"
	}
	return data, imgType, nil
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
