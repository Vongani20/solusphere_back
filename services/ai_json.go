package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"solusphere_backend/internal/ai"
)

// GenerateStructuredJSON asks OpenAI for JSON-only output and parses it into a map.
func GenerateStructuredJSON(ctx context.Context, systemPrompt, userPrompt string, maxTokens int) (map[string]interface{}, error) {
	return GenerateStructuredJSONWithMedia(ctx, systemPrompt, userPrompt, nil, nil, maxTokens, "")
}

// GenerateStructuredJSONWithMedia is GenerateStructuredJSON with optional images/files.
func GenerateStructuredJSONWithMedia(ctx context.Context, systemPrompt, userPrompt string, images []ai.ImageInput, files []ai.FileInput, maxTokens int, model string) (map[string]interface{}, error) {
	if !IsOpenAIInitialized() {
		return nil, fmt.Errorf("OpenAI is not configured")
	}
	if maxTokens <= 0 {
		maxTokens = 2500
	}
	if ctx == nil {
		ctx = context.Background()
	}

	// Respect the caller's deadline; only add a local cap when none exists.
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 90*time.Second)
		defer cancel()
	}

	raw, err := ai.GenerateText(ctx, ai.GenerateTextRequest{
		SystemPrompt:    systemPrompt,
		UserPrompt:      userPrompt,
		Images:          images,
		Files:           files,
		MaxOutputTokens: maxTokens,
		Temperature:     0.1,
		Model:           model,
	})
	if err != nil {
		return nil, err
	}

	return parseJSONObject(raw)
}

func parseJSONObject(raw string) (map[string]interface{}, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)
	if idx := strings.Index(raw, "{"); idx > 0 {
		raw = raw[idx:]
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &result); err == nil {
		return result, nil
	}

	repaired := repairTruncatedJSON(raw)
	if err := json.Unmarshal([]byte(repaired), &result); err != nil {
		preview := raw
		if len(preview) > 240 {
			preview = preview[:240]
		}
		return nil, fmt.Errorf("failed to parse JSON response: %w (%q)", err, preview)
	}
	return result, nil
}

func repairTruncatedJSON(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "{"); i >= 0 {
		s = s[i:]
	}
	inString := false
	escaped := false
	stack := make([]byte, 0, 8)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
				continue
			}
			if c == '"' {
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '{':
			stack = append(stack, '}')
		case '[':
			stack = append(stack, ']')
		case '}', ']':
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
	}
	if inString {
		s += `"`
	}
	s = strings.TrimRight(s, ", \t\n\r")
	for i := len(stack) - 1; i >= 0; i-- {
		s += string(stack[i])
	}
	return s
}

func confidenceFromResult(result map[string]interface{}, fallback float64) float64 {
	if value, ok := result["confidence_score"].(float64); ok && value > 0 && value <= 1 {
		return value
	}
	if extracted, ok := result["extracted_data"].(map[string]interface{}); ok {
		if value, ok := extracted["confidence_score"].(float64); ok && value > 0 && value <= 1 {
			return value
		}
	}
	return fallback
}
