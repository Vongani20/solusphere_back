package services

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"solusphere_backend/internal/ai"
	"solusphere_backend/models"
)

const rfpSystemPrompt = `You turn a source tender, bid, or RFP into a clean RFP document template.
Use only facts that appear in the source. Do not invent dates, prices, weights, contacts, or requirements.
Read the whole source, including specifications, returnable schedules, and annexures near the end. Do not stop after the introduction.
If a field is not stated, use an empty string or an empty array and add a short note to gaps.
Write in plain English. Keep each bullet to one sentence.
Return a single JSON object with exactly these keys:
title, referenceNumber, issuer, issueDate, closingDate, contactName, contactEmail, summary, background,
objectives (string[]), scope (string[]), deliverables (string[]),
requirements ({category, items: string[]}[]), eligibility (string[]),
evaluationCriteria ({criterion, weight}[]), timeline ({milestone, date}[]),
submissionMethod, submissionFormat, requiredDocuments (string[]), terms (string[]), gaps (string[]),
solution: { summary, approach (string[]), whyItFits (string[]), team (string[]), risks ({risk, response}[]) }.
The solution is a practical idea a bidder could use. Base it on the scope, deliverables, requirements, and evaluation criteria.
Do not invent a company name, price, past project, or date that the source does not state.`

const (
	rfpSinglePassRunes = 350000
	rfpChunkRunes      = 90000
)

// RFPAnalysis is the result of reading tender text.
type RFPAnalysis struct {
	Template models.RFPTemplate
	Source   string // "ai" or "builtin"
	Model    string
	Warning  string
}

// AnalyzeRFPText builds an RFP template, using OpenAI when configured and the built-in reader otherwise.
func AnalyzeRFPText(ctx context.Context, text, filename string) RFPAnalysis {
	if !IsOpenAIInitialized() {
		return RFPAnalysis{
			Template: withRFPSolution(buildLocalRFPTemplate(text, filename)),
			Source:   "builtin",
			Warning:  "Built with the built-in reader because the AI service is not configured. Ask an admin to set the OpenAI key for a fuller reading.",
		}
	}

	var (
		tmpl models.RFPTemplate
		err  error
	)
	if len([]rune(text)) <= rfpSinglePassRunes {
		tmpl, err = callRFPModel(ctx, filename, text, "")
	} else {
		tmpl, err = analyzeRFPInParts(ctx, filename, text)
	}
	if err != nil {
		return RFPAnalysis{
			Template: withRFPSolution(buildLocalRFPTemplate(text, filename)),
			Source:   "builtin",
			Warning:  fmt.Sprintf("The AI model could not be reached (%s). A template was built with the built-in reader instead.", shortError(err)),
		}
	}
	return RFPAnalysis{Template: withRFPSolution(tmpl), Source: "ai", Model: GetOpenAIModel()}
}

func shortError(err error) string {
	msg := err.Error()
	if len(msg) > 180 {
		msg = msg[:180]
	}
	return msg
}

func withRFPSolution(t models.RFPTemplate) models.RFPTemplate {
	t = models.NormalizeRFPTemplate(t)
	if t.Solution.Summary == "" && len(t.Solution.Approach) == 0 {
		t.Solution = BuildRFPSolutionIdea(t)
	}
	return t
}

func callRFPModel(ctx context.Context, filename, text, partNote string) (models.RFPTemplate, error) {
	user := "Source file: " + filename + "\n"
	if partNote != "" {
		user += partNote + "\n"
	}
	user += "This text is the source to read in full.\nSOURCE:\n" + text

	callCtx, cancel := context.WithTimeout(ctx, 4*time.Minute)
	defer cancel()
	raw, err := ai.GenerateText(callCtx, ai.GenerateTextRequest{
		SystemPrompt:    rfpSystemPrompt,
		UserPrompt:      user,
		MaxOutputTokens: 8000,
		Temperature:     0.1,
	})
	if err != nil {
		return models.RFPTemplate{}, err
	}
	payload, err := parseLooseJSONObject(raw)
	if err != nil {
		return models.RFPTemplate{}, err
	}
	return models.NormalizeRFPTemplateMap(payload), nil
}

var rfpFence = regexp.MustCompile("(?is)```(?:json)?\\s*(.*?)```")

func parseLooseJSONObject(content string) (map[string]interface{}, error) {
	raw := content
	if m := rfpFence.FindStringSubmatch(content); m != nil {
		raw = m[1]
	}
	raw = strings.TrimSpace(raw)
	start, end := strings.Index(raw, "{"), strings.LastIndex(raw, "}")
	if start == -1 || end <= start {
		return nil, fmt.Errorf("the model did not return JSON")
	}
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw[start:end+1]), &out); err != nil {
		return nil, fmt.Errorf("parse model JSON: %w", err)
	}
	return out, nil
}

func analyzeRFPInParts(ctx context.Context, filename, text string) (models.RFPTemplate, error) {
	chunks := splitRFPChunks(text, rfpChunkRunes)
	partials := make([]models.RFPTemplate, 0, len(chunks))
	for i, chunk := range chunks {
		note := fmt.Sprintf("This is part %d of %d of one tender. Extract only the facts in this part.", i+1, len(chunks))
		t, err := callRFPModel(ctx, filename, chunk, note)
		if err != nil {
			return models.RFPTemplate{}, err
		}
		partials = append(partials, t)
	}
	return mergeRFPTemplates(partials), nil
}

func splitRFPChunks(text string, size int) []string {
	runes := []rune(text)
	chunks := []string{}
	start := 0
	for start < len(runes) {
		end := start + size
		if end > len(runes) {
			end = len(runes)
		}
		if end < len(runes) {
			window := string(runes[start:end])
			if idx := strings.LastIndex(window, "\n\n"); idx >= 0 {
				breakAt := start + len([]rune(window[:idx]))
				if breakAt > start+size*6/10 {
					end = breakAt
				}
			}
		}
		chunks = append(chunks, string(runes[start:end]))
		if end >= len(runes) {
			break
		}
		next := end - 500
		if next > start {
			start = next
		} else {
			start = end
		}
	}
	return chunks
}

func longest(values []string) string {
	best := ""
	for _, v := range values {
		if len(v) > len(best) {
			best = v
		}
	}
	return best
}

func uniqueStrings(items []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, item := range items {
		if item == "" {
			continue
		}
		key := strings.ToLower(item)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
	}
	return out
}

func mergeRFPTemplates(parts []models.RFPTemplate) models.RFPTemplate {
	var m models.RFPTemplate
	pick := func(get func(models.RFPTemplate) string) string {
		vals := []string{}
		for _, p := range parts {
			vals = append(vals, get(p))
		}
		return longest(vals)
	}
	flat := func(get func(models.RFPTemplate) []string) []string {
		all := []string{}
		for _, p := range parts {
			all = append(all, get(p)...)
		}
		return uniqueStrings(all)
	}

	m.Title = pick(func(t models.RFPTemplate) string { return t.Title })
	m.ReferenceNumber = pick(func(t models.RFPTemplate) string { return t.ReferenceNumber })
	m.Issuer = pick(func(t models.RFPTemplate) string { return t.Issuer })
	m.IssueDate = pick(func(t models.RFPTemplate) string { return t.IssueDate })
	m.ClosingDate = pick(func(t models.RFPTemplate) string { return t.ClosingDate })
	m.ContactName = pick(func(t models.RFPTemplate) string { return t.ContactName })
	m.ContactEmail = pick(func(t models.RFPTemplate) string { return t.ContactEmail })
	m.Summary = pick(func(t models.RFPTemplate) string { return t.Summary })
	m.Background = pick(func(t models.RFPTemplate) string { return t.Background })
	m.SubmissionMethod = pick(func(t models.RFPTemplate) string { return t.SubmissionMethod })
	m.SubmissionFormat = pick(func(t models.RFPTemplate) string { return t.SubmissionFormat })
	m.Objectives = flat(func(t models.RFPTemplate) []string { return t.Objectives })
	m.Scope = flat(func(t models.RFPTemplate) []string { return t.Scope })
	m.Deliverables = flat(func(t models.RFPTemplate) []string { return t.Deliverables })
	m.Eligibility = flat(func(t models.RFPTemplate) []string { return t.Eligibility })
	m.RequiredDocuments = flat(func(t models.RFPTemplate) []string { return t.RequiredDocuments })
	m.Terms = flat(func(t models.RFPTemplate) []string { return t.Terms })

	truncGap := regexp.MustCompile(`(?i)truncat|not available in the`)
	for _, g := range flat(func(t models.RFPTemplate) []string { return t.Gaps }) {
		if !truncGap.MatchString(g) {
			m.Gaps = append(m.Gaps, g)
		}
	}

	groupIndex := map[string]int{}
	seenCrit, seenTime, seenRisk := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, p := range parts {
		for _, g := range p.Requirements {
			key := strings.ToLower(g.Category)
			if idx, ok := groupIndex[key]; ok {
				m.Requirements[idx].Items = uniqueStrings(append(m.Requirements[idx].Items, g.Items...))
			} else {
				groupIndex[key] = len(m.Requirements)
				m.Requirements = append(m.Requirements, models.RFPRequirementGroup{Category: g.Category, Items: uniqueStrings(g.Items)})
			}
		}
		for _, c := range p.EvaluationCriteria {
			key := strings.ToLower(c.Criterion + "|" + c.Weight)
			if !seenCrit[key] {
				seenCrit[key] = true
				m.EvaluationCriteria = append(m.EvaluationCriteria, c)
			}
		}
		for _, t := range p.Timeline {
			key := strings.ToLower(t.Milestone + "|" + t.Date)
			if !seenTime[key] {
				seenTime[key] = true
				m.Timeline = append(m.Timeline, t)
			}
		}
		for _, r := range p.Solution.Risks {
			key := strings.ToLower(r.Risk + "|" + r.Response)
			if !seenRisk[key] {
				seenRisk[key] = true
				m.Solution.Risks = append(m.Solution.Risks, r)
			}
		}
	}
	m.Solution.Summary = pick(func(t models.RFPTemplate) string { return t.Solution.Summary })
	m.Solution.Approach = flat(func(t models.RFPTemplate) []string { return t.Solution.Approach })
	m.Solution.WhyItFits = flat(func(t models.RFPTemplate) []string { return t.Solution.WhyItFits })
	m.Solution.Team = flat(func(t models.RFPTemplate) []string { return t.Solution.Team })
	return models.NormalizeRFPTemplate(m)
}

// BuildRFPSolutionIdea drafts a bidder approach from the template's own content.
func BuildRFPSolutionIdea(t models.RFPTemplate) models.RFPSolution {
	s := models.RFPSolution{}
	if t.Summary != "" {
		s.Summary = "Meet the request directly: " + t.Summary
	} else {
		s.Summary = "Propose a delivery plan that follows the scope, the mandatory requirements, and the way the bid will be scored."
	}
	for i, item := range t.Scope {
		if i >= 6 {
			break
		}
		s.Approach = append(s.Approach, "Deliver this by "+lowerFirst(item)+".")
	}
	if len(s.Approach) == 0 {
		s.Approach = []string{"Set out a delivery plan that follows the scope in the order the issuer listed it."}
	}
	for _, c := range t.EvaluationCriteria {
		line := "Shape the response so it scores on " + c.Criterion
		if c.Weight != "" {
			line += " (" + c.Weight + ")"
		}
		s.WhyItFits = append(s.WhyItFits, line+".")
	}
	s.Team = []string{"Name a lead, the people who do the work, and a support contact for the contract period."}
	for i, gap := range t.Gaps {
		if i >= 5 {
			break
		}
		s.Risks = append(s.Risks, models.RFPRisk{Risk: gap, Response: "Confirm this point with the issuer before the bid is submitted."})
	}
	return s
}

func lowerFirst(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return v
	}
	r := []rune(v)
	return strings.ToLower(string(r[0])) + string(r[1:])
}

// ---------- built-in reader (no AI) ----------

var (
	reEmail       = regexp.MustCompile(`(?i)[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}`)
	reReference   = regexp.MustCompile(`(?i)\b(?:RFP|RFQ|RFT|ITT|Tender|Bid)\s*(?:No\.?|Number|#)?\s*[:\-]?\s*([A-Z0-9][A-Z0-9\-/]+)`)
	reDate        = regexp.MustCompile(`\b(\d{1,2}\s+[A-Za-z]+\s+\d{4}(?:,\s*\d{1,2}:\d{2})?|\d{4}-\d{2}-\d{2}|\d{1,2}[/\-]\d{1,2}[/\-]\d{2,4})\b`)
	reBulletLead  = regexp.MustCompile(`^[-•*\d).\s]+`)
	reNumHeading  = regexp.MustCompile(`^\d+(?:\.\d+)*[.)\]]\s+\S+`)
	reNumPrefix   = regexp.MustCompile(`^\d+(?:\.\d+)*[.)\]]\s*`)
	reHasUpper    = regexp.MustCompile(`[A-Z]`)
	rePercent     = regexp.MustCompile(`(\d+\s*%)`)
	reTrailPunct  = regexp.MustCompile(`[:\-–\s]+$`)
	reTrailPunct2 = regexp.MustCompile(`[:\-–]+$`)
	reModal       = regexp.MustCompile(`(?i)\b(must|shall|required to|need to)\b`)
	reTimelineKW  = regexp.MustCompile(`(?i)due|clos|award|question|publish|go-live|evaluation`)
)

type rfpSection struct {
	title string
	lines []string
}

func buildLocalRFPTemplate(text, filename string) models.RFPTemplate {
	lines := []string{}
	for _, l := range strings.Split(text, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			lines = append(lines, l)
		}
	}
	sections := splitRFPSections(lines)
	joined := strings.Join(lines, "\n")

	email := reEmail.FindString(joined)
	reference := ""
	if m := reReference.FindStringSubmatch(joined); m != nil {
		reference = m[1]
	}
	issuer := findLabeled(lines, regexp.MustCompile(`(?i)issued by|contracting authority|prepared by|organisation|organization`))
	if issuer == "" {
		issuer = guessIssuer(lines)
	}
	issuer = tidyOrg(issuer)
	contactName := findLabeled(lines, regexp.MustCompile(`(?i)^contact\b|contact person|enquiries`))
	if email != "" {
		contactName = strings.ReplaceAll(contactName, email, "")
	}
	contactName = regexp.MustCompile(`(?i)email.*`).ReplaceAllString(contactName, "")
	contactName = regexp.MustCompile(`[,;].*$`).ReplaceAllString(contactName, "")

	closing := findDateNear(lines, regexp.MustCompile(`(?i)closing|deadline|due date|submission date`))
	if closing == "" {
		closing = findLabeled(lines, regexp.MustCompile(`^closing\b|deadline`))
	}

	summary := firstParagraph(sections, regexp.MustCompile(`(?i)purpose|introduction|summary|overview`))
	if summary == "" {
		summary = strings.Join(firstN(lines, 4), " ")
	}

	t := models.RFPTemplate{
		Title:              guessTitle(lines, filename, issuer),
		ReferenceNumber:    reference,
		Issuer:             issuer,
		IssueDate:          findDateNear(lines, regexp.MustCompile(`(?i)issue date|^issued\b|date of issue`)),
		ClosingDate:        closing,
		ContactName:        strings.TrimSpace(contactName),
		ContactEmail:       email,
		Summary:            summary,
		Background:         firstParagraph(sections, regexp.MustCompile(`(?i)background|context`)),
		Objectives:         bulletsFrom(sections, regexp.MustCompile(`(?i)objective|goal|^aims?$`)),
		Scope:              bulletsFrom(sections, regexp.MustCompile(`(?i)scope|statement of work|services required`)),
		Deliverables:       bulletsFrom(sections, regexp.MustCompile(`(?i)deliverable|outputs?`)),
		Requirements:       requirementGroups(sections, lines),
		Eligibility:        bulletsFrom(sections, regexp.MustCompile(`(?i)eligib|qualification`)),
		EvaluationCriteria: criteriaFrom(sections, lines),
		Timeline:           timelineFrom(sections, lines),
		SubmissionMethod:   firstParagraph(sections, regexp.MustCompile(`(?i)submission|how to submit|lodgement|lodgment`)),
		SubmissionFormat:   findIn(sections, regexp.MustCompile(`(?i)submission`), regexp.MustCompile(`(?i)\bformat\b|structure`)),
		RequiredDocuments:  bulletsFrom(sections, regexp.MustCompile(`(?i)required document|documents to include|checklist|annex`)),
		Terms:              bulletsFrom(sections, regexp.MustCompile(`(?i)^terms|conditions`)),
	}
	t = models.NormalizeRFPTemplate(t)
	t.Gaps = collectRFPGaps(t)
	t.Solution = BuildRFPSolutionIdea(t)
	return models.NormalizeRFPTemplate(t)
}

func firstN(items []string, n int) []string {
	if len(items) < n {
		return items
	}
	return items[:n]
}

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}

func guessTitle(lines []string, filename, issuer string) string {
	rfpLabel := regexp.MustCompile(`(?i)request for proposal|invitation to tender|request for quotation`)
	for _, l := range lines {
		if rfpLabel.MatchString(l) && len(l) < 80 {
			if issuer != "" && len(l) < 40 {
				return issuer + " — " + l
			}
			return reTrailPunct2.ReplaceAllString(l, "")
		}
	}
	startsDigit := regexp.MustCompile(`^\d`)
	for _, l := range lines {
		if len(l) > 18 && len(l) < 140 && !strings.Contains(l, "@") && !startsDigit.MatchString(l) {
			return reTrailPunct2.ReplaceAllString(l, "")
		}
	}
	name := filename
	if i := strings.LastIndex(name, "."); i > 0 {
		name = name[:i]
	}
	return name
}

func tidyOrg(v string) string {
	if v == "" {
		return ""
	}
	letters := regexp.MustCompile(`[^A-Za-z]`).ReplaceAllString(v, "")
	if letters == "" || letters != strings.ToUpper(letters) {
		return v
	}
	small := map[string]bool{"of": true, "the": true, "and": true, "for": true, "a": true, "an": true}
	words := strings.Fields(strings.ToLower(v))
	for i, w := range words {
		if i > 0 && small[w] {
			continue
		}
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}

func guessIssuer(lines []string) string {
	org := regexp.MustCompile(`(?i)city|department|ministry|authority|office|company|ltd|inc|municipality`)
	for _, l := range firstN(lines, 6) {
		if org.MatchString(l) {
			return l
		}
	}
	return ""
}

func findLabeled(lines []string, pattern *regexp.Regexp) string {
	for _, l := range lines {
		if !pattern.MatchString(l) || len(l) > 220 || !strings.Contains(l, ":") {
			continue
		}
		parts := strings.SplitN(l, ":", 2)
		if len(parts[0]) > 48 {
			continue
		}
		return strings.TrimSpace(parts[1])
	}
	return ""
}

func findIn(sections []rfpSection, sectionPattern, linePattern *regexp.Regexp) string {
	for _, s := range matchingSections(sections, sectionPattern) {
		for _, l := range s.lines {
			if linePattern.MatchString(l) {
				return strings.TrimSpace(reBulletLead.ReplaceAllString(l, ""))
			}
		}
	}
	return ""
}

func findDateNear(lines []string, pattern *regexp.Regexp) string {
	for i, l := range lines {
		if !pattern.MatchString(l) {
			continue
		}
		if m := reDate.FindStringSubmatch(l); m != nil {
			return m[1]
		}
		if i+1 < len(lines) {
			if m := reDate.FindStringSubmatch(lines[i+1]); m != nil {
				return m[1]
			}
		}
	}
	return ""
}

func isRFPHeading(l string) bool {
	if len(l) > 90 {
		return false
	}
	if reNumHeading.MatchString(l) && len(l) < 80 {
		return true
	}
	if strings.HasSuffix(l, ":") && len(l) < 60 {
		return true
	}
	return l == strings.ToUpper(l) && reHasUpper.MatchString(l) && len(l) > 4
}

func splitRFPSections(lines []string) []rfpSection {
	sections := []rfpSection{}
	cur := rfpSection{title: "Intro"}
	for _, l := range lines {
		if isRFPHeading(l) {
			if len(cur.lines) > 0 || cur.title != "Intro" {
				sections = append(sections, cur)
			}
			title := strings.TrimSuffix(reNumPrefix.ReplaceAllString(l, ""), ":")
			cur = rfpSection{title: title}
		} else {
			cur.lines = append(cur.lines, l)
		}
	}
	return append(sections, cur)
}

func matchingSections(sections []rfpSection, pattern *regexp.Regexp) []rfpSection {
	out := []rfpSection{}
	for _, s := range sections {
		if pattern.MatchString(s.title) {
			out = append(out, s)
		}
	}
	return out
}

func firstParagraph(sections []rfpSection, pattern *regexp.Regexp) string {
	ms := matchingSections(sections, pattern)
	if len(ms) == 0 {
		return ""
	}
	bulletStart := regexp.MustCompile(`^[-•*]`)
	parts := []string{}
	for _, l := range ms[0].lines {
		if !bulletStart.MatchString(l) {
			parts = append(parts, l)
		}
	}
	return clip(strings.Join(parts, " "), 700)
}

func bulletsFrom(sections []rfpSection, pattern *regexp.Regexp) []string {
	items := []string{}
	for _, s := range matchingSections(sections, pattern) {
		for _, l := range s.lines {
			clean := strings.TrimSpace(reBulletLead.ReplaceAllString(l, ""))
			if len(clean) > 8 {
				items = append(items, clean)
			}
		}
	}
	return firstN(items, 12)
}

func requirementGroups(sections []rfpSection, lines []string) []models.RFPRequirementGroup {
	const category = "Requirements stated in the source"
	if items := bulletsFrom(sections, regexp.MustCompile(`(?i)requirement|mandatory|specification|technical`)); len(items) > 0 {
		return []models.RFPRequirementGroup{{Category: category, Items: items}}
	}
	modal := []string{}
	for _, l := range lines {
		if reModal.MatchString(l) {
			modal = append(modal, reBulletLead.ReplaceAllString(l, ""))
		}
	}
	modal = uniqueStrings(firstN(modal, 12))
	if len(modal) == 0 {
		return nil
	}
	return []models.RFPRequirementGroup{{Category: category, Items: modal}}
}

func uniqueExact(items []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, i := range items {
		if !seen[i] {
			seen[i] = true
			out = append(out, i)
		}
	}
	return out
}

func criteriaFrom(sections []rfpSection, lines []string) []models.RFPCriterion {
	source := []string{}
	for _, s := range matchingSections(sections, regexp.MustCompile(`(?i)evaluat|criteria|scoring|weight`)) {
		source = append(source, s.lines...)
	}
	for _, l := range lines {
		if rePercent.MatchString(l) {
			source = append(source, l)
		}
	}
	out := []models.RFPCriterion{}
	for _, l := range uniqueExact(source) {
		weight := ""
		if m := rePercent.FindStringSubmatch(l); m != nil {
			weight = m[1]
		}
		c := l
		if weight != "" {
			c = strings.Replace(c, weight, "", 1)
		}
		c = strings.TrimSpace(reTrailPunct.ReplaceAllString(reBulletLead.ReplaceAllString(c, ""), ""))
		if c != "" {
			out = append(out, models.RFPCriterion{Criterion: c, Weight: weight})
		}
		if len(out) >= 8 {
			break
		}
	}
	return out
}

func timelineFrom(sections []rfpSection, lines []string) []models.RFPMilestone {
	source := []string{}
	for _, s := range matchingSections(sections, regexp.MustCompile(`(?i)timeline|schedule|milestone|key date`)) {
		source = append(source, s.lines...)
	}
	for _, l := range lines {
		if reDate.MatchString(l) && reTimelineKW.MatchString(l) {
			source = append(source, l)
		}
	}
	out := []models.RFPMilestone{}
	for _, l := range uniqueExact(source) {
		date := ""
		if m := reDate.FindStringSubmatch(l); m != nil {
			date = m[1]
		}
		ms := l
		if date != "" {
			ms = strings.Replace(ms, date, "", 1)
		}
		ms = strings.TrimSpace(reTrailPunct2.ReplaceAllString(reBulletLead.ReplaceAllString(ms, ""), ""))
		if ms == "" {
			ms = l
		}
		out = append(out, models.RFPMilestone{Milestone: ms, Date: date})
		if len(out) >= 10 {
			break
		}
	}
	return out
}

func collectRFPGaps(t models.RFPTemplate) []string {
	gaps := []string{}
	if t.Issuer == "" {
		gaps = append(gaps, "Issuing organisation was not found.")
	}
	if t.ClosingDate == "" {
		gaps = append(gaps, "Closing date was not found.")
	}
	if t.ContactEmail == "" {
		gaps = append(gaps, "Submission contact email was not found.")
	}
	if len(t.Scope) == 0 {
		gaps = append(gaps, "Scope of work needs a manual check.")
	}
	if len(t.Requirements) == 0 {
		gaps = append(gaps, "Mandatory requirements need a manual check.")
	}
	if len(t.EvaluationCriteria) == 0 {
		gaps = append(gaps, "Evaluation criteria were not found.")
	}
	if len(t.RequiredDocuments) == 0 {
		gaps = append(gaps, "Required submission documents were not found.")
	}
	return gaps
}
