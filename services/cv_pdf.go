package services

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
	"solusphere_backend/models"
)

// Page layout constants (mm, A4 = 210 x 297)
const (
	cvPageW = 210.0
	cvPageH = 297.0
	cvML    = 12.0 // left margin
	cvMR    = 12.0 // right margin
	cvMT    = 10.0 // top margin
	cvCol1X = cvML
	cvCol1W = 63.0
	cvGap   = 5.0
	cvCol2X = cvCol1X + cvCol1W + cvGap // 80
	cvCol2W = cvPageW - cvMR - cvCol2X  // 118
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

// ---------- Page 1 ----------

func (s *CVPDFService) drawPage1(pdf *fpdf.Fpdf, profile *models.CVProfile) {
	// Logo / brand top-right
	logoW, logoH := 40.0, 15.0
	logoX := cvPageW - cvMR - logoW
	s.drawBrandHeader(pdf, logoX, cvMT, logoW, logoH)

	// Name block (left-aligned, beside logo)
	pdf.SetXY(cvML, cvMT)
	pdf.SetFont("Helvetica", "B", 15)
	setCV_Blue(pdf)
	name := strings.TrimSpace(profile.FirstName + " " + profile.LastName)
	if name == "" {
		name = "[First Name] [Surname]"
	}
	availW := logoX - cvML - 2
	pdf.CellFormat(availW, 7, name, "", 1, "L", false, 0, "")
	pdf.SetX(cvML)
	pdf.SetFont("Helvetica", "", 8)
	setCV_Gray(pdf)
	pdf.CellFormat(availW, 5, "CURRICULUM VITAE", "", 1, "L", false, 0, "")
	setCV_Black(pdf)

	contentTopY := cvMT + 14

	// ---- Left column ----
	leftY := contentTopY

	// Profile photo placeholder / actual photo
	photoH := 52.0
	s.drawPhoto(pdf, cvCol1X, leftY, cvCol1W, photoH, profile.ProfilePhotoURL)
	leftY += photoH + 4

	// PROFILE section
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

	// VALUE PROPOSITION section
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

	// ---- Right column (bordered box) ----
	boxTop := contentTopY
	footerY := cvPageH - 10.0
	boxH := footerY - boxTop - 2

	pdf.SetDrawColor(180, 180, 180)
	pdf.SetLineWidth(0.3)
	pdf.Rect(cvCol2X, boxTop, cvCol2W, boxH, "D")

	innerX := cvCol2X + 3
	innerW := cvCol2W - 6
	rightY := boxTop + 3

	// Personal Details
	rightY = s.drawRightHeading(pdf, "PERSONAL DETAILS", innerX, rightY, innerW)
	rightY = s.drawPersonalDetails(pdf, profile, innerX, rightY, innerW)
	rightY += 3

	// Professional Skills
	rightY = s.drawRightHeading(pdf, "PROFESSIONAL SKILLS", innerX, rightY, innerW)
	rightY = s.drawSkillsList(pdf, profile.ProfessionalSkills, innerX, rightY, innerW)
	rightY += 3

	// Qualifications
	rightY = s.drawRightHeading(pdf, "QUALIFICATIONS AND TRAINING", innerX, rightY, innerW)
	rightY = s.drawBulletList(pdf, profile.Qualifications, "[Insert qualification]", innerX, rightY, innerW)
	rightY += 3

	// Computer Skills
	rightY = s.drawRightHeading(pdf, "COMPUTER SKILLS", innerX, rightY, innerW)
	rightY = s.drawBulletList(pdf, profile.ComputerSkills, "[Insert skills]", innerX, rightY, innerW)
	rightY += 3

	// Professional Membership
	rightY = s.drawRightHeading(pdf, "PROFESSIONAL MEMBERSHIP", innerX, rightY, innerW)
	mem := strings.Join(profile.ProfessionalMemberships, ", ")
	if mem == "" {
		mem = "[Insert memberships]"
	}
	pdf.SetXY(innerX, rightY)
	pdf.SetFont("Helvetica", "", 7.5)
	setCV_Black(pdf)
	pdf.MultiCell(innerW, 4, mem, "", "L", false)
	rightY = pdf.GetY() + 3

	// Languages
	rightY = s.drawRightHeading(pdf, "LANGUAGES", innerX, rightY, innerW)
	langs := strings.Join(profile.Languages, ", ")
	if langs == "" {
		langs = "[Insert language(s)]"
	}
	pdf.SetXY(innerX, rightY)
	pdf.SetFont("Helvetica", "", 7.5)
	setCV_Black(pdf)
	pdf.MultiCell(innerW, 4, langs, "", "L", false)

	_ = rightY
	s.drawFooter(pdf, 1, 2)
}

// ---------- Page 2 ----------

func (s *CVPDFService) drawPage2(pdf *fpdf.Fpdf, profile *models.CVProfile) {
	// Brand header top-right
	logoW, logoH := 40.0, 15.0
	logoX := cvPageW - cvMR - logoW
	s.drawBrandHeader(pdf, logoX, cvMT, logoW, logoH)

	// EXPERIENCE heading
	y := cvMT
	pdf.SetXY(cvML, y)
	pdf.SetFont("Helvetica", "B", 11)
	setCV_Blue(pdf)
	pdf.CellFormat(180, 7, "EXPERIENCE", "", 1, "L", false, 0, "")
	y += 9
	setCV_Black(pdf)

	if len(profile.Experience) == 0 {
		pdf.SetXY(cvML, y)
		pdf.SetFont("Helvetica", "", 8)
		setCV_Gray(pdf)
		pdf.CellFormat(180, 5, "[No experience entries added yet]", "", 1, "L", false, 0, "")
	}

	for _, exp := range profile.Experience {
		if y > 268 {
			break
		}

		// Company row
		pdf.SetXY(cvML, y)
		pdf.SetFont("Helvetica", "B", 8.5)
		setCV_Black(pdf)
		pdf.CellFormat(28, 5, "Company:", "", 0, "L", false, 0, "")
		pdf.SetFont("Helvetica", "B", 8.5)
		pdf.CellFormat(152, 5, exp.Company, "", 1, "L", false, 0, "")
		y += 5

		// Position row
		pdf.SetXY(cvML, y)
		pdf.SetFont("Helvetica", "B", 8.5)
		pdf.CellFormat(28, 5, "Position:", "", 0, "L", false, 0, "")
		pdf.CellFormat(152, 5, exp.Position, "", 1, "L", false, 0, "")
		y += 5

		// Period row
		pdf.SetXY(cvML, y)
		pdf.SetFont("Helvetica", "", 8.5)
		pdf.CellFormat(28, 5, "Period:", "", 0, "L", false, 0, "")
		pdf.CellFormat(152, 5, formatCVPeriod(exp.PeriodStart, exp.PeriodEnd), "", 1, "L", false, 0, "")
		y += 5

		// Scope of work
		if len(exp.ScopeOfWork) > 0 {
			pdf.SetXY(cvML, y)
			pdf.SetFont("Helvetica", "B", 8.5)
			setCV_Blue(pdf)
			pdf.CellFormat(180, 5, "Scope of work:", "", 1, "L", false, 0, "")
			y += 5
			setCV_Black(pdf)
			for _, scope := range exp.ScopeOfWork {
				if y > 270 {
					break
				}
				pdf.SetXY(cvML+4, y)
				pdf.SetFont("Helvetica", "", 8)
				pdf.MultiCell(176-4, 4.5, "- "+scope, "", "L", false)
				y = pdf.GetY()
			}
		}
		y += 6
	}

	// Horizontal rule
	pdf.SetDrawColor(0, 0, 0)
	pdf.SetLineWidth(0.3)
	pdf.Line(cvML, 285, cvPageW-cvMR, 285)

	s.drawFooter(pdf, 2, 2)
}

// ---------- Drawing helpers ----------

func (s *CVPDFService) drawBrandHeader(pdf *fpdf.Fpdf, x, y, w, h float64) {
	logoPath := "assets/solugrowth_logo.png"
	if _, err := os.Stat(logoPath); err == nil {
		opts := fpdf.ImageOptions{ImageType: "PNG", ReadDpi: false}
		pdf.ImageOptions(logoPath, x, y, w, h, false, opts, 0, "")
		return
	}
	// Text fallback
	pdf.SetFont("Helvetica", "B", 9)
	setCV_Blue(pdf)
	pdf.SetXY(x, y+2)
	pdf.CellFormat(w, 5, "SoluGrowth", "", 1, "R", false, 0, "")
	pdf.SetX(x)
	pdf.SetFont("Helvetica", "", 6.5)
	setCV_Gray(pdf)
	pdf.CellFormat(w, 4, "BPO - ITO - KPO", "", 0, "R", false, 0, "")
	setCV_Black(pdf)
}

func (s *CVPDFService) drawPhoto(pdf *fpdf.Fpdf, x, y, w, h float64, photoURL string) {
	if photoURL != "" {
		imgData, imgType, err := downloadImage(photoURL)
		if err == nil {
			name := fmt.Sprintf("cv-photo-%d", time.Now().UnixNano())
			opts := fpdf.ImageOptions{ImageType: imgType}
			pdf.RegisterImageOptionsReader(name, opts, bytes.NewReader(imgData))
			pdf.ImageOptions(name, x, y, w, h, false, opts, 0, "")
			return
		}
	}
	// Grey placeholder
	pdf.SetDrawColor(200, 200, 200)
	pdf.SetFillColor(235, 235, 235)
	pdf.Rect(x, y, w, h, "FD")
	pdf.SetFont("Helvetica", "", 7)
	setCV_Gray(pdf)
	pdf.SetXY(x, y+h/2-2)
	pdf.CellFormat(w, 4, "Insert photo", "", 0, "C", false, 0, "")
	setCV_Black(pdf)
	pdf.SetFillColor(255, 255, 255)
}

func (s *CVPDFService) drawLeftHeading(pdf *fpdf.Fpdf, title string, w float64) {
	pdf.SetFont("Helvetica", "B", 9)
	setCV_Blue(pdf)
	pdf.CellFormat(w, 5, title, "", 1, "L", false, 0, "")
	setCV_Black(pdf)
}

func (s *CVPDFService) drawRightHeading(pdf *fpdf.Fpdf, title string, x, y, w float64) float64 {
	pdf.SetXY(x, y)
	pdf.SetFont("Helvetica", "B", 8)
	setCV_Blue(pdf)
	pdf.CellFormat(w, 5, title, "", 1, "L", false, 0, "")
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
		pdf.SetXY(x+2, y)
		pdf.SetFont("Helvetica", "B", 7.5)
		setCV_Black(pdf)
		pdf.CellFormat(w-2, lineH, "- "+skill.Skill+":", "", 1, "L", false, 0, "")
		y += lineH
		for _, d := range skill.Details {
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
	if len(items) == 0 {
		pdf.SetXY(x+2, y)
		pdf.CellFormat(w-2, lineH, "- "+placeholder, "", 1, "L", false, 0, "")
		return y + lineH
	}
	for _, item := range items {
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

// ---------- Color helpers ----------

func setCV_Blue(pdf *fpdf.Fpdf)  { pdf.SetTextColor(51, 102, 204) }
func setCV_Gray(pdf *fpdf.Fpdf)  { pdf.SetTextColor(100, 100, 100) }
func setCV_Black(pdf *fpdf.Fpdf) { pdf.SetTextColor(0, 0, 0) }

// ---------- Utility ----------

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
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return s
	}
	return t.Format("02 January 2006")
}

func formatCVPeriod(start, end string) string {
	parse := func(s string) string {
		if t, err := time.Parse("2006-01", s); err == nil {
			return t.Format("January 2006")
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
