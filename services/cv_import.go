package services

import (
	"context"
	"fmt"
	"strings"

	"solusphere_backend/models"
)

const cvImportSystemPrompt = `You are a CV/resume extraction specialist.
Read the document text and return ONLY valid JSON matching this schema:
{
  "first_name": "string",
  "last_name": "string",
  "profile_text": "string",
  "value_proposition": "string",
  "gender": "string",
  "nationality": "string",
  "date_of_birth": "YYYY-MM-DD or empty",
  "professional_skills": [{"skill": "string", "details": ["string"]}],
  "qualifications": ["string"],
  "computer_skills": ["string"],
  "professional_memberships": ["string"],
  "languages": ["string"],
  "experience": [{
    "company": "string",
    "position": "string",
    "period_start": "string",
    "period_end": "string",
    "scope_of_work": ["string"]
  }],
  "warnings": ["note any missing or uncertain fields"]
}
Rules:
- Extract only facts present in the document. Do not invent employers, degrees, or dates.
- Normalize date_of_birth to YYYY-MM-DD when possible.
- Keep profile_text concise (2-4 sentences).
- If a field is missing, use an empty string or empty array.
- Include warnings for ambiguous or missing critical data.`

// ParseCVFromDocumentText maps extracted document text into a CV profile draft.
func ParseCVFromDocumentText(ctx context.Context, text string) (*models.CVProfile, []string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, nil, fmt.Errorf("document contains no readable text")
	}

	payload, err := GenerateStructuredJSON(ctx, cvImportSystemPrompt, "Extract CV data from this document:\n\n"+text, 3500)
	if err != nil {
		return nil, nil, err
	}

	profile := mapToCVProfile(payload)
	models.SanitizeCVProfile(profile)

	warnings := stringListFromAny(payload["warnings"])
	if len(profile.FirstName) == 0 && len(profile.LastName) == 0 {
		warnings = append(warnings, "Name could not be detected — please review personal information.")
	}

	return profile, warnings, nil
}

func mapToCVProfile(payload map[string]interface{}) *models.CVProfile {
	profile := &models.CVProfile{
		FirstName:               stringFromAny(payload["first_name"]),
		LastName:                stringFromAny(payload["last_name"]),
		ProfileText:             stringFromAny(payload["profile_text"]),
		ValueProposition:        stringFromAny(payload["value_proposition"]),
		Gender:                  stringFromAny(payload["gender"]),
		Nationality:             stringFromAny(payload["nationality"]),
		DateOfBirth:             models.NormalizeDateOfBirth(stringFromAny(payload["date_of_birth"])),
		Qualifications:          stringListFromAny(payload["qualifications"]),
		ComputerSkills:          stringListFromAny(payload["computer_skills"]),
		ProfessionalMemberships: stringListFromAny(payload["professional_memberships"]),
		Languages:               stringListFromAny(payload["languages"]),
		ProfessionalSkills:      parseProfessionalSkills(payload["professional_skills"]),
		Experience:              parseExperience(payload["experience"]),
	}
	return profile
}

func stringFromAny(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return ""
	}
}

func stringListFromAny(value interface{}) []string {
	items, ok := value.([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if text := stringFromAny(item); text != "" {
			out = append(out, text)
		}
	}
	return out
}

func parseProfessionalSkills(value interface{}) []models.ProfessionalSkill {
	items, ok := value.([]interface{})
	if !ok {
		return nil
	}
	out := make([]models.ProfessionalSkill, 0, len(items))
	for _, item := range items {
		obj, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		out = append(out, models.ProfessionalSkill{
			Skill:   stringFromAny(obj["skill"]),
			Details: stringListFromAny(obj["details"]),
		})
	}
	return out
}

func parseExperience(value interface{}) []models.CVExperience {
	items, ok := value.([]interface{})
	if !ok {
		return nil
	}
	out := make([]models.CVExperience, 0, len(items))
	for _, item := range items {
		obj, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		out = append(out, models.CVExperience{
			Company:     stringFromAny(obj["company"]),
			Position:    stringFromAny(obj["position"]),
			PeriodStart: stringFromAny(obj["period_start"]),
			PeriodEnd:   stringFromAny(obj["period_end"]),
			ScopeOfWork: stringListFromAny(obj["scope_of_work"]),
		})
	}
	return out
}
