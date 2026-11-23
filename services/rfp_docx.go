package services

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	docx "github.com/fumiama/go-docx"

	"solusphere_backend/models"
)

const (
	rfpGreen = "146B3A"
	rfpInk   = "1B1F1C"
	rfpMuted = "5E675F"
	rfpBrand = "0E8FA8"
)

type rfpDocWriter struct {
	doc *docx.Docx
}

func (w rfpDocWriter) text(value string, bold, italic bool, color, size string) {
	run := w.doc.AddParagraph().AddText(value).Size(size).Color(color)
	if bold {
		run.Bold()
	}
	if italic {
		run.Italic()
	}
}

func (w rfpDocWriter) body(value string)  { w.text(value, false, false, rfpInk, "22") }
func (w rfpDocWriter) hint(value string)  { w.text(value, false, true, rfpMuted, "22") }
func (w rfpDocWriter) label(value string) { w.text(value, true, false, rfpInk, "22") }
func (w rfpDocWriter) heading(value string) {
	w.doc.AddParagraph()
	w.text(value, true, false, rfpGreen, "28")
}
func (w rfpDocWriter) bullets(items []string, empty string) {
	if len(items) == 0 {
		if empty != "" {
			w.hint(empty)
		}
		return
	}
	for _, item := range items {
		w.body("•  " + item)
	}
}

func (w rfpDocWriter) table(rows [][2]string) {
	if len(rows) == 0 {
		return
	}
	t := w.doc.AddTable(len(rows), 2, 9360, nil)
	for i, row := range rows {
		left, right := row[0], row[1]
		if left == "" {
			left = "—"
		}
		if right == "" {
			right = "—"
		}
		t.TableRows[i].TableCells[0].AddParagraph().AddText(left).Bold().Size("20").Color(rfpGreen)
		t.TableRows[i].TableCells[1].AddParagraph().AddText(right).Size("20").Color(rfpInk)
	}
}

// RenderRFPDocx renders an RFP template as a Word document.
func RenderRFPDocx(d *models.RFPDocument) ([]byte, error) {
	t := d.Template
	w := rfpDocWriter{doc: docx.New().WithDefaultTheme().WithA4Page()}

	w.text("SoluGrowth · SoluSphere", true, false, rfpBrand, "18")
	w.text("PROPOSAL", true, false, rfpGreen, "20")
	title := t.Title
	if title == "" {
		title = "Untitled RFP"
	}
	w.text(title, true, false, rfpInk, "40")
	summary := t.Summary
	if summary == "" {
		summary = "Review this template and fill any open items before it is issued."
	}
	w.text(summary, false, false, rfpMuted, "22")

	sourceNames := []string{}
	for _, s := range d.Sources {
		if s.Name != "" {
			sourceNames = append(sourceNames, s.Name)
		}
	}
	sourceLabel := strings.Join(sourceNames, ", ")
	if sourceLabel == "" {
		sourceLabel = d.OriginalFilename
	}
	contact := strings.Join(nonEmpty(t.ContactName, t.ContactEmail), " · ")
	w.table([][2]string{
		{"Reference", t.ReferenceNumber},
		{"Issued by", t.Issuer},
		{"Issue date", t.IssueDate},
		{"Closing", t.ClosingDate},
		{"Contact", contact},
		{"Source files", sourceLabel},
	})

	w.heading("Proposal")
	w.hint("Each answer follows a proposal requirement from the source tender.")
	if len(t.Solution.Responses) == 0 {
		w.hint("Add a response for each proposal requirement.")
	}
	for _, item := range t.Solution.Responses {
		req := item.Requirement
		if req == "" {
			req = "Requirement"
		}
		w.label(req)
		if item.Response != "" {
			w.body(item.Response)
		} else {
			w.hint("Add how the proposal meets this requirement.")
		}
	}

	w.heading("How the proposal is delivered")
	if t.Solution.Summary != "" {
		w.body(t.Solution.Summary)
	} else {
		w.hint("Add the approach a bidder could use to meet this request.")
	}
	if len(t.Solution.Approach) > 0 {
		w.label("How to deliver it")
		w.bullets(t.Solution.Approach, "")
	}
	if len(t.Solution.WhyItFits) > 0 {
		w.label("Why this fits the evaluation")
		w.bullets(t.Solution.WhyItFits, "")
	}
	if len(t.Solution.Team) > 0 {
		w.label("Team to put forward")
		w.bullets(t.Solution.Team, "")
	}
	if len(t.Solution.Risks) > 0 {
		w.label("Risks to handle")
		risks := []string{}
		for _, r := range t.Solution.Risks {
			if r.Response != "" {
				risks = append(risks, r.Risk+" — "+r.Response)
			} else {
				risks = append(risks, r.Risk)
			}
		}
		w.bullets(risks, "")
	}

	w.heading("1. Background")
	if t.Background != "" {
		w.body(t.Background)
	} else {
		w.hint("Add the context a bidder needs before reading the scope.")
	}

	w.heading("2. Objectives")
	w.bullets(t.Objectives, "Add the outcomes this RFP is meant to achieve.")
	w.heading("3. Scope of work")
	w.bullets(t.Scope, "Add the work the supplier is expected to perform.")
	w.heading("4. Deliverables")
	w.bullets(t.Deliverables, "Add the tangible outputs.")

	w.heading("5. Requirements")
	if len(t.Requirements) == 0 {
		w.hint("Add mandatory and technical requirements.")
	}
	for _, g := range t.Requirements {
		cat := g.Category
		if cat == "" {
			cat = "Requirements"
		}
		w.label(cat)
		w.bullets(g.Items, "")
	}

	w.heading("6. Eligibility")
	w.bullets(t.Eligibility, "Add who may bid.")

	w.heading("7. Evaluation")
	if len(t.EvaluationCriteria) == 0 {
		w.hint("Add how proposals will be scored.")
	} else {
		rows := [][2]string{}
		for _, c := range t.EvaluationCriteria {
			rows = append(rows, [2]string{c.Weight, c.Criterion})
		}
		w.table(rows)
	}

	w.heading("8. Timeline")
	if len(t.Timeline) == 0 {
		w.hint("Add milestones and dates.")
	} else {
		rows := [][2]string{}
		for _, m := range t.Timeline {
			date := m.Date
			if date == "" {
				date = "Date"
			}
			rows = append(rows, [2]string{date, m.Milestone})
		}
		w.table(rows)
	}

	w.heading("9. How to submit")
	if t.SubmissionMethod != "" {
		w.body(t.SubmissionMethod)
	} else {
		w.hint("Describe where and how proposals must be sent.")
	}
	if t.SubmissionFormat != "" {
		w.body(t.SubmissionFormat)
	}

	w.heading("10. Documents to include")
	if len(t.RequiredDocuments) == 0 {
		w.hint("None captured from the source. Add the items you need before sending.")
	}
	for _, item := range t.RequiredDocuments {
		w.body("☐  " + item)
	}

	w.heading("11. Terms")
	w.bullets(t.Terms, "Add commercial or legal terms the bidder must accept.")

	w.heading("12. Confirm before issue")
	w.bullets(t.Gaps, "No obvious gaps were flagged. Still read the template against the source document.")

	w.heading("13. Sign-off")
	w.body("Prepared by: ______________________________")
	w.body("Date: ______________________________")
	w.body("Authorised by: ______________________________")

	var buf bytes.Buffer
	if _, err := w.doc.WriteTo(&buf); err != nil {
		return nil, fmt.Errorf("write docx: %w", err)
	}
	return buf.Bytes(), nil
}

func nonEmpty(values ...string) []string {
	out := []string{}
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			out = append(out, v)
		}
	}
	return out
}

var rfpSlugPattern = regexp.MustCompile(`[^a-z0-9]+`)

// RFPSlug turns a title into a safe filename stem.
func RFPSlug(value string) string {
	slug := strings.Trim(rfpSlugPattern.ReplaceAllString(strings.ToLower(value), "-"), "-")
	if len(slug) > 80 {
		slug = strings.Trim(slug[:80], "-")
	}
	if slug == "" {
		return "rfp-template"
	}
	return slug
}
