package models

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// RFP document statuses.
const (
	RFPStatusProcessing = "processing"
	RFPStatusReady      = "ready"
	RFPStatusReview     = "review"
	RFPStatusFailed     = "failed"
)

type RFPRequirementGroup struct {
	Category string   `json:"category"`
	Items    []string `json:"items"`
}

type RFPCriterion struct {
	Criterion string `json:"criterion"`
	Weight    string `json:"weight"`
}

type RFPMilestone struct {
	Milestone string `json:"milestone"`
	Date      string `json:"date"`
}

type RFPRisk struct {
	Risk     string `json:"risk"`
	Response string `json:"response"`
}

// RFPProposalResponse is one proposal answer to a requirement taken from the tender.
type RFPProposalResponse struct {
	Requirement string `json:"requirement"`
	Response    string `json:"response"`
}

type RFPSolution struct {
	Summary   string                 `json:"summary"`
	Approach  []string               `json:"approach"`
	WhyItFits []string               `json:"whyItFits"`
	Team      []string               `json:"team"`
	Risks     []RFPRisk              `json:"risks"`
	Responses []RFPProposalResponse  `json:"responses"`
}

// RFPTemplate is the structured RFP template built from one or more tender documents.
type RFPTemplate struct {
	Title              string                `json:"title"`
	ReferenceNumber    string                `json:"referenceNumber"`
	Issuer             string                `json:"issuer"`
	IssueDate          string                `json:"issueDate"`
	ClosingDate        string                `json:"closingDate"`
	ContactName        string                `json:"contactName"`
	ContactEmail       string                `json:"contactEmail"`
	Summary            string                `json:"summary"`
	Background         string                `json:"background"`
	Objectives         []string              `json:"objectives"`
	Scope              []string              `json:"scope"`
	Deliverables       []string              `json:"deliverables"`
	Requirements       []RFPRequirementGroup `json:"requirements"`
	Eligibility        []string              `json:"eligibility"`
	EvaluationCriteria []RFPCriterion        `json:"evaluationCriteria"`
	Timeline           []RFPMilestone        `json:"timeline"`
	SubmissionMethod   string                `json:"submissionMethod"`
	SubmissionFormat   string                `json:"submissionFormat"`
	RequiredDocuments  []string              `json:"requiredDocuments"`
	Terms              []string              `json:"terms"`
	Gaps               []string              `json:"gaps"`
	Solution           RFPSolution           `json:"solution"`
}

// RFPSource is one uploaded document and its extracted text.
type RFPSource struct {
	Name string `json:"name"`
	Text string `json:"text,omitempty"`
}

// RFPDocument is a stored RFP template owned by a user.
type RFPDocument struct {
	ID               string      `json:"id"`
	UserID           int         `json:"userId"`
	OriginalFilename string      `json:"originalFilename"`
	Sources          []RFPSource `json:"sources"`
	ExtractedText    string      `json:"-"`
	Source           string      `json:"source"`
	Model            string      `json:"model"`
	Truncated        bool        `json:"truncated"`
	Warning          string      `json:"warning"`
	Status           string      `json:"status"`
	Template         RFPTemplate `json:"template"`
	CreatedAt        time.Time   `json:"createdAt"`
	UpdatedAt        time.Time   `json:"updatedAt"`
}

// RFPDocumentSummary is the list view of an RFP document.
type RFPDocumentSummary struct {
	ID               string    `json:"id"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
	OriginalFilename string    `json:"originalFilename"`
	Source           string    `json:"source"`
	Model            string    `json:"model"`
	Status           string    `json:"status"`
	Warning          string    `json:"warning"`
	Title            string    `json:"title"`
	Issuer           string    `json:"issuer"`
	ReferenceNumber  string    `json:"referenceNumber"`
	ClosingDate      string    `json:"closingDate"`
	GapCount         int       `json:"gapCount"`
	SourceCount      int       `json:"sourceCount"`
}

// Present returns the document as sent to clients: source names only, plus text length.
func (d *RFPDocument) Present() map[string]interface{} {
	names := make([]RFPSource, 0, len(d.Sources))
	for _, s := range d.Sources {
		names = append(names, RFPSource{Name: s.Name})
	}
	if len(names) == 0 && d.OriginalFilename != "" {
		names = append(names, RFPSource{Name: d.OriginalFilename})
	}
	return map[string]interface{}{
		"id":               d.ID,
		"originalFilename": d.OriginalFilename,
		"sources":          names,
		"source":           d.Source,
		"model":            d.Model,
		"truncated":        d.Truncated,
		"warning":          d.Warning,
		"status":           d.Status,
		"template":         d.Template,
		"textLength":       len([]rune(d.ExtractedText)),
		"createdAt":        d.CreatedAt,
		"updatedAt":        d.UpdatedAt,
	}
}

var rfpSpaceRun = regexp.MustCompile(`\s+`)

func rfpString(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(rfpSpaceRun.ReplaceAllString(t, " "))
	case float64, bool, int:
		return strings.TrimSpace(fmt.Sprint(t))
	default:
		return ""
	}
}

func rfpList(v interface{}) []string {
	out := []string{}
	switch t := v.(type) {
	case string:
		if s := rfpString(t); s != "" {
			out = append(out, s)
		}
	case []interface{}:
		for _, item := range t {
			if s := rfpString(item); s != "" {
				out = append(out, s)
			}
		}
	case []string:
		for _, item := range t {
			if s := rfpString(item); s != "" {
				out = append(out, s)
			}
		}
	}
	return out
}

func rfpField(m map[string]interface{}, keys ...string) interface{} {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			if s, isStr := v.(string); isStr && s == "" {
				continue
			}
			return v
		}
	}
	return nil
}

func rfpPairs(v interface{}, left, right []string) [][2]string {
	items, _ := v.([]interface{})
	out := [][2]string{}
	for _, raw := range items {
		var l, r string
		switch t := raw.(type) {
		case map[string]interface{}:
			l = rfpString(rfpField(t, left...))
			r = rfpString(rfpField(t, right...))
		default:
			l = rfpString(t)
		}
		if l != "" {
			out = append(out, [2]string{l, r})
		}
	}
	return out
}

// NormalizeRFPTemplateMap converts loosely-typed JSON (from the model or a client) into a clean template.
func NormalizeRFPTemplateMap(input map[string]interface{}) RFPTemplate {
	if input == nil {
		input = map[string]interface{}{}
	}
	t := RFPTemplate{
		Title:             rfpString(input["title"]),
		ReferenceNumber:   rfpString(input["referenceNumber"]),
		Issuer:            rfpString(input["issuer"]),
		IssueDate:         rfpString(input["issueDate"]),
		ClosingDate:       rfpString(input["closingDate"]),
		ContactName:       rfpString(input["contactName"]),
		ContactEmail:      rfpString(input["contactEmail"]),
		Summary:           rfpString(input["summary"]),
		Background:        rfpString(input["background"]),
		Objectives:        rfpList(input["objectives"]),
		Scope:             rfpList(input["scope"]),
		Deliverables:      rfpList(input["deliverables"]),
		Eligibility:       rfpList(input["eligibility"]),
		SubmissionMethod:  rfpString(input["submissionMethod"]),
		SubmissionFormat:  rfpString(input["submissionFormat"]),
		RequiredDocuments: rfpList(input["requiredDocuments"]),
		Terms:             rfpList(input["terms"]),
		Gaps:              rfpList(input["gaps"]),
	}

	if groups, ok := input["requirements"].([]interface{}); ok {
		for _, raw := range groups {
			g, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			category := rfpString(rfpField(g, "category", "name"))
			if category == "" {
				category = "Requirements"
			}
			t.Requirements = append(t.Requirements, RFPRequirementGroup{
				Category: category,
				Items:    rfpList(rfpField(g, "items", "points")),
			})
		}
	}
	for _, p := range rfpPairs(input["evaluationCriteria"], []string{"criterion", "name"}, []string{"weight", "score"}) {
		t.EvaluationCriteria = append(t.EvaluationCriteria, RFPCriterion{Criterion: p[0], Weight: p[1]})
	}
	for _, p := range rfpPairs(input["timeline"], []string{"milestone", "name"}, []string{"date", "when"}) {
		t.Timeline = append(t.Timeline, RFPMilestone{Milestone: p[0], Date: p[1]})
	}

	sol, _ := input["solution"].(map[string]interface{})
	if sol == nil {
		sol = map[string]interface{}{}
	}
	t.Solution = RFPSolution{
		Summary:   rfpString(sol["summary"]),
		Approach:  rfpList(sol["approach"]),
		WhyItFits: rfpList(rfpField(sol, "whyItFits", "fit")),
		Team:      rfpList(sol["team"]),
	}
	for _, p := range rfpPairs(sol["risks"], []string{"risk"}, []string{"response"}) {
		t.Solution.Risks = append(t.Solution.Risks, RFPRisk{Risk: p[0], Response: p[1]})
	}
	for _, p := range rfpPairs(sol["responses"], []string{"requirement", "item"}, []string{"response", "answer"}) {
		t.Solution.Responses = append(t.Solution.Responses, RFPProposalResponse{Requirement: p[0], Response: p[1]})
	}
	return NormalizeRFPTemplate(t)
}

// NormalizeRFPTemplate trims every field and guarantees non-nil slices so JSON never carries null lists.
func NormalizeRFPTemplate(t RFPTemplate) RFPTemplate {
	clean := func(items []string) []string { return rfpList(toIface(items)) }
	t.Title = rfpString(t.Title)
	t.ReferenceNumber = rfpString(t.ReferenceNumber)
	t.Issuer = rfpString(t.Issuer)
	t.IssueDate = rfpString(t.IssueDate)
	t.ClosingDate = rfpString(t.ClosingDate)
	t.ContactName = rfpString(t.ContactName)
	t.ContactEmail = rfpString(t.ContactEmail)
	t.Summary = rfpString(t.Summary)
	t.Background = rfpString(t.Background)
	t.SubmissionMethod = rfpString(t.SubmissionMethod)
	t.SubmissionFormat = rfpString(t.SubmissionFormat)
	t.Objectives = clean(t.Objectives)
	t.Scope = clean(t.Scope)
	t.Deliverables = clean(t.Deliverables)
	t.Eligibility = clean(t.Eligibility)
	t.RequiredDocuments = clean(t.RequiredDocuments)
	t.Terms = clean(t.Terms)
	t.Gaps = clean(t.Gaps)

	groups := []RFPRequirementGroup{}
	for _, g := range t.Requirements {
		g.Category = rfpString(g.Category)
		g.Items = clean(g.Items)
		if g.Category != "" || len(g.Items) > 0 {
			groups = append(groups, g)
		}
	}
	t.Requirements = groups

	criteria := []RFPCriterion{}
	for _, c := range t.EvaluationCriteria {
		c.Criterion, c.Weight = rfpString(c.Criterion), rfpString(c.Weight)
		if c.Criterion != "" {
			criteria = append(criteria, c)
		}
	}
	t.EvaluationCriteria = criteria

	timeline := []RFPMilestone{}
	for _, m := range t.Timeline {
		m.Milestone, m.Date = rfpString(m.Milestone), rfpString(m.Date)
		if m.Milestone != "" {
			timeline = append(timeline, m)
		}
	}
	t.Timeline = timeline

	t.Solution.Summary = rfpString(t.Solution.Summary)
	t.Solution.Approach = clean(t.Solution.Approach)
	t.Solution.WhyItFits = clean(t.Solution.WhyItFits)
	t.Solution.Team = clean(t.Solution.Team)
	risks := []RFPRisk{}
	for _, r := range t.Solution.Risks {
		r.Risk, r.Response = rfpString(r.Risk), rfpString(r.Response)
		if r.Risk != "" {
			risks = append(risks, r)
		}
	}
	t.Solution.Risks = risks
	responses := []RFPProposalResponse{}
	for _, r := range t.Solution.Responses {
		r.Requirement, r.Response = rfpString(r.Requirement), rfpString(r.Response)
		if r.Requirement != "" || r.Response != "" {
			responses = append(responses, r)
		}
	}
	t.Solution.Responses = responses
	return t
}

func toIface(items []string) []interface{} {
	out := make([]interface{}, len(items))
	for i, s := range items {
		out[i] = s
	}
	return out
}

// RFPDocumentStatus marks thin or gap-heavy templates for review.
func RFPDocumentStatus(t RFPTemplate) string {
	thin := t.Title == "" || (t.Summary == "" && len(t.Scope) == 0 && len(t.Requirements) == 0)
	if thin || len(t.Gaps) > 5 {
		return RFPStatusReview
	}
	return RFPStatusReady
}

const rfpSelectColumns = `id, user_id, original_filename, sources_json, extracted_text, analysis_source,
	model, truncated, warning, status, template_json, created_at, updated_at`

func scanRFPDocument(row interface{ Scan(...interface{}) error }) (*RFPDocument, error) {
	var (
		d                                     RFPDocument
		sourcesJSON, text, model, warning, tj sql.NullString
		truncated                             bool
	)
	if err := row.Scan(&d.ID, &d.UserID, &d.OriginalFilename, &sourcesJSON, &text, &d.Source,
		&model, &truncated, &warning, &d.Status, &tj, &d.CreatedAt, &d.UpdatedAt); err != nil {
		return nil, err
	}
	d.ExtractedText = text.String
	d.Model = model.String
	d.Truncated = truncated
	d.Warning = warning.String
	if sourcesJSON.Valid && sourcesJSON.String != "" {
		_ = json.Unmarshal([]byte(sourcesJSON.String), &d.Sources)
	}
	if tj.Valid && tj.String != "" {
		_ = json.Unmarshal([]byte(tj.String), &d.Template)
	}
	d.Template = NormalizeRFPTemplate(d.Template)
	return &d, nil
}

// SaveRFPDocument inserts or replaces an RFP document.
func SaveRFPDocument(db *sql.DB, d *RFPDocument) error {
	sources, err := json.Marshal(d.Sources)
	if err != nil {
		return err
	}
	tmpl, err := json.Marshal(NormalizeRFPTemplate(d.Template))
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	if d.CreatedAt.IsZero() {
		d.CreatedAt = now
	}
	d.UpdatedAt = now
	_, err = db.Exec(`
		INSERT INTO rfp_documents (id, user_id, original_filename, sources_json, extracted_text, analysis_source,
			model, truncated, warning, status, template_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			original_filename = VALUES(original_filename),
			sources_json = VALUES(sources_json),
			extracted_text = VALUES(extracted_text),
			analysis_source = VALUES(analysis_source),
			model = VALUES(model),
			truncated = VALUES(truncated),
			warning = VALUES(warning),
			status = VALUES(status),
			template_json = VALUES(template_json),
			updated_at = VALUES(updated_at)`,
		d.ID, d.UserID, truncateRunes(d.OriginalFilename, 1000), string(sources), d.ExtractedText, d.Source,
		rfpNullable(d.Model), d.Truncated, d.Warning, d.Status, string(tmpl), d.CreatedAt, d.UpdatedAt)
	return err
}

// GetRFPDocument loads one document owned by userID; returns nil, nil when not found.
func GetRFPDocument(db *sql.DB, id string, userID int) (*RFPDocument, error) {
	row := db.QueryRow(`SELECT `+rfpSelectColumns+` FROM rfp_documents WHERE id = ? AND user_id = ?`, id, userID)
	d, err := scanRFPDocument(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return d, err
}

// ListRFPDocuments returns the user's documents, newest first.
func ListRFPDocuments(db *sql.DB, userID int) ([]RFPDocumentSummary, error) {
	rows, err := db.Query(`SELECT id, original_filename, sources_json, analysis_source, model, warning, status,
		template_json, created_at, updated_at FROM rfp_documents WHERE user_id = ? ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []RFPDocumentSummary{}
	for rows.Next() {
		var (
			s                               RFPDocumentSummary
			sourcesJSON, model, warning, tj sql.NullString
		)
		if err := rows.Scan(&s.ID, &s.OriginalFilename, &sourcesJSON, &s.Source, &model, &warning, &s.Status,
			&tj, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		s.Model, s.Warning = model.String, warning.String
		var sources []RFPSource
		if sourcesJSON.Valid {
			_ = json.Unmarshal([]byte(sourcesJSON.String), &sources)
		}
		s.SourceCount = len(sources)
		if s.SourceCount == 0 {
			s.SourceCount = 1
		}
		var t RFPTemplate
		if tj.Valid {
			_ = json.Unmarshal([]byte(tj.String), &t)
		}
		s.Title = t.Title
		if s.Title == "" {
			s.Title = "Untitled RFP"
		}
		s.Issuer, s.ReferenceNumber, s.ClosingDate, s.GapCount = t.Issuer, t.ReferenceNumber, t.ClosingDate, len(t.Gaps)
		out = append(out, s)
	}
	return out, rows.Err()
}

// DeleteRFPDocument removes a user's document; reports whether a row was deleted.
func DeleteRFPDocument(db *sql.DB, id string, userID int) (bool, error) {
	res, err := db.Exec(`DELETE FROM rfp_documents WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

func rfpNullable(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
