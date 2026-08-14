package services

import (
	"context"
	"encoding/base64"
	"fmt"
	"path/filepath"
	"strings"

	"solusphere_backend/internal/ai"
	"solusphere_backend/models"
)

const cvImportSystemPrompt = `You are a CV/resume extraction specialist.
Read the FULL document text and return ONLY valid JSON matching this schema:
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
- Preserve profile_text and value_proposition in full from the document — do not shorten or summarize them.
- If a field is missing, use an empty string or empty array.
- Include warnings for ambiguous or missing critical data.
- CRITICAL: Extract EVERY work experience / role in the document, not just the first one.
- For each role, include ALL scope-of-work / responsibility / achievement bullets as separate strings. Do not truncate lists.
- Do not summarize multiple jobs into one entry. Preserve company, position, period, and bullets per role.
- Prefer completeness over brevity for experience, skills, qualifications, and every free-text field.
- Return a single JSON object only. No markdown, no commentary.`

// cvImportFastModel is a vision-capable model that finishes inside CloudFront's
// 60s origin timeout. Production default (gpt-5.4) is too slow for page OCR.
const cvImportFastModel = "gpt-4.1-mini"
const cvImportMaxPages = 4
const cvImportMaxTokens = 12000

// ImportCVProfileFromUpload extracts a CV profile from an uploaded document.
// Text-based files use native extract + JSON mapping. Scanned/vectorized PDFs
// use a single vision call (page images → JSON) to stay within proxy timeouts.
func ImportCVProfileFromUpload(ctx context.Context, filename string, data []byte) (*models.CVProfile, []string, error) {
	if len(data) == 0 {
		return nil, nil, fmt.Errorf("empty document")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	kind := detectCVDocumentKind(filename, data)
	text, err := extractNativeCVText(kind, data)
	if err == nil {
		text = strings.TrimSpace(text)
		if isUsefulCVText(text) {
			return ParseCVFromDocumentText(ctx, text)
		}
	}

	images, files, mediaErr := prepareCVImportMedia(ctx, filename, kind, data)
	if mediaErr != nil {
		return nil, nil, fmt.Errorf("no readable text found in document; %w", mediaErr)
	}
	if len(images) == 0 && len(files) == 0 {
		return nil, nil, fmt.Errorf("no readable text found in document and OCR inputs were unavailable")
	}

	return ParseCVFromDocumentMedia(ctx, images, files)
}

func prepareCVImportMedia(ctx context.Context, filename, kind string, data []byte) ([]ai.ImageInput, []ai.FileInput, error) {
	switch kind {
	case "jpeg", "png", "gif", "webp":
		mime := "image/" + kind
		if kind == "jpeg" {
			mime = "image/jpeg"
		}
		return []ai.ImageInput{{
			ImageURL: fmt.Sprintf("data:%s;base64,%s", mime, base64.StdEncoding.EncodeToString(data)),
			Detail:   "auto",
		}}, nil, nil
	case "pdf":
		pages, err := renderPDFPagesToImages(ctx, data, cvImportMaxPages)
		if err == nil && len(pages) > 0 {
			return pages, nil, nil
		}
		imgs := extractPageLikeImagesFromPDF(data, 4)
		var files []ai.FileInput
		if len(imgs) == 0 && len(data) <= 8<<20 {
			name := filenameBase(filename, "cv.pdf")
			files = []ai.FileInput{{
				Filename: name,
				FileData: "data:application/pdf;base64," + base64.StdEncoding.EncodeToString(data),
			}}
		}
		if len(imgs) == 0 && len(files) == 0 {
			if err != nil {
				return nil, nil, fmt.Errorf("PDF page rendering failed: %w", err)
			}
			return nil, nil, fmt.Errorf("PDF page rendering produced no images")
		}
		return imgs, files, nil
	default:
		name := filenameBase(filename, "document.bin")
		return nil, []ai.FileInput{{
			Filename: name,
			FileData: fmt.Sprintf("data:%s;base64,%s", mimeForCVKind(kind), base64.StdEncoding.EncodeToString(data)),
		}}, nil
	}
}

func filenameBase(filename, fallback string) string {
	name := filepath.Base(strings.TrimSpace(filename))
	if name == "" || name == "." || name == string(filepath.Separator) {
		return fallback
	}
	return name
}

// ParseCVFromDocumentText maps extracted document text into a CV profile draft.
func ParseCVFromDocumentText(ctx context.Context, text string) (*models.CVProfile, []string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, nil, fmt.Errorf("document contains no readable text")
	}

	userPrompt := "Extract COMPLETE CV data from this entire document. Do not skip later experience roles or scope bullets. Return one compact JSON object only — no markdown fences.\n\n" + text
	payload, err := GenerateStructuredJSONWithMedia(ctx, cvImportSystemPrompt, userPrompt, nil, nil, cvImportMaxTokens, cvImportFastModel)
	if err != nil {
		return nil, nil, err
	}
	return profileFromImportPayload(payload, text)
}

// ParseCVFromDocumentMedia maps page images / attached files directly into a CV profile.
func ParseCVFromDocumentMedia(ctx context.Context, images []ai.ImageInput, files []ai.FileInput) (*models.CVProfile, []string, error) {
	userPrompt := "Read every attached CV page/image carefully and extract COMPLETE CV data as one compact JSON object. No markdown fences. Do not skip later experience roles or scope bullets."
	payload, err := GenerateStructuredJSONWithMedia(ctx, cvImportSystemPrompt, userPrompt, images, files, cvImportMaxTokens, cvImportFastModel)
	if err != nil {
		return nil, nil, err
	}
	return profileFromImportPayload(payload, "")
}

func profileFromImportPayload(payload map[string]interface{}, sourceText string) (*models.CVProfile, []string, error) {
	profile := mapToCVProfile(payload)
	models.SanitizeCVProfile(profile)

	warnings := stringListFromAny(payload["warnings"])
	if len(profile.FirstName) == 0 && len(profile.LastName) == 0 {
		warnings = append(warnings, "Name could not be detected — please review personal information.")
	}
	if len(profile.Experience) == 0 {
		warnings = append(warnings, "No experience entries were detected — add roles manually and check the source document.")
	} else if sourceText != "" && len(profile.Experience) == 1 && strings.Count(strings.ToLower(sourceText), "company") >= 2 {
		warnings = append(warnings, "Only one experience entry was extracted. Review the Experience step — later roles may need to be added manually.")
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
