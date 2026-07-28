package services

import (
	"strings"
	"unicode/utf8"

	"solusphere_backend/models"
)

// Template soft limits for PDF/Word layout fitting only (not used on import).
const (
	cvMaxProfileWords       = 500
	cvMaxValuePropWords     = 500
	cvMaxNameRunes          = 80
	cvMaxLineRunes          = 240
	cvMaxSkillDetails       = 20
	cvMaxProfessionalSkills = 40
	cvMaxQualifications     = 40
	cvMaxComputerSkills     = 40
	cvMaxMemberships        = 40
	cvMaxLanguages          = 40
	cvMaxExperience         = 40
	cvMaxScopeBullets       = 40
)

// FitCVProfileForTemplate returns a copy trimmed to the designated template slots
// so PDF/Word exports stay clean and do not overflow the branded layout.
func FitCVProfileForTemplate(src *models.CVProfile) *models.CVProfile {
	if src == nil {
		return nil
	}
	out := *src
	out.FirstName = truncateRunes(strings.TrimSpace(src.FirstName), cvMaxNameRunes/2)
	out.LastName = truncateRunes(strings.TrimSpace(src.LastName), cvMaxNameRunes/2)
	out.ProfileText = limitWords(src.ProfileText, cvMaxProfileWords)
	out.ValueProposition = limitWords(src.ValueProposition, cvMaxValuePropWords)
	out.Gender = truncateRunes(strings.TrimSpace(src.Gender), 24)
	out.Nationality = truncateRunes(strings.TrimSpace(src.Nationality), 40)
	out.DateOfBirth = strings.TrimSpace(src.DateOfBirth)

	out.ProfessionalSkills = fitSkills(src.ProfessionalSkills, cvMaxProfessionalSkills, cvMaxSkillDetails)
	out.Qualifications = fitStringList(src.Qualifications, cvMaxQualifications, cvMaxLineRunes)
	out.ComputerSkills = fitStringList(src.ComputerSkills, cvMaxComputerSkills, cvMaxLineRunes)
	out.ProfessionalMemberships = fitStringList(src.ProfessionalMemberships, cvMaxMemberships, cvMaxLineRunes)
	out.Languages = fitStringList(src.Languages, cvMaxLanguages, 40)
	out.Experience = fitExperience(src.Experience, cvMaxExperience, cvMaxScopeBullets)
	return &out
}

func fitSkills(skills []models.ProfessionalSkill, maxSkills, maxDetails int) []models.ProfessionalSkill {
	if len(skills) > maxSkills {
		skills = skills[:maxSkills]
	}
	out := make([]models.ProfessionalSkill, 0, len(skills))
	for _, skill := range skills {
		s := models.ProfessionalSkill{
			Skill:   truncateRunes(strings.TrimSpace(skill.Skill), cvMaxLineRunes),
			Details: fitStringList(skill.Details, maxDetails, cvMaxLineRunes),
		}
		if s.Skill == "" && len(s.Details) == 0 {
			continue
		}
		out = append(out, s)
	}
	return out
}

func fitExperience(entries []models.CVExperience, maxEntries, maxScope int) []models.CVExperience {
	if len(entries) > maxEntries {
		entries = entries[:maxEntries]
	}
	out := make([]models.CVExperience, 0, len(entries))
	for _, exp := range entries {
		out = append(out, models.CVExperience{
			Company:     truncateRunes(strings.TrimSpace(exp.Company), cvMaxLineRunes),
			Position:    truncateRunes(strings.TrimSpace(exp.Position), cvMaxLineRunes),
			PeriodStart: strings.TrimSpace(exp.PeriodStart),
			PeriodEnd:   strings.TrimSpace(exp.PeriodEnd),
			ScopeOfWork: fitStringList(exp.ScopeOfWork, maxScope, 140),
		})
	}
	return out
}

func fitStringList(items []string, maxItems, maxRunes int) []string {
	out := make([]string, 0, min(len(items), maxItems))
	for _, item := range items {
		item = truncateRunes(strings.TrimSpace(item), maxRunes)
		if item == "" {
			continue
		}
		out = append(out, item)
		if len(out) >= maxItems {
			break
		}
	}
	return out
}

func limitWords(text string, maxWords int) string {
	text = strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	if text == "" || maxWords <= 0 {
		return text
	}
	words := strings.Fields(text)
	if len(words) <= maxWords {
		return text
	}
	return strings.Join(words[:maxWords], " ")
}

func truncateRunes(s string, max int) string {
	if max <= 0 || s == "" {
		return s
	}
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	if max <= 1 {
		return string(runes[:max])
	}
	return string(runes[:max-1]) + "…"
}
