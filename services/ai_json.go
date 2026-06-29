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
	if !IsOpenAIInitialized() {
		return nil, fmt.Errorf("OpenAI is not configured")
	}
	if maxTokens <= 0 {
		maxTokens = 2500
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	raw, err := ai.GenerateText(ctx, ai.GenerateTextRequest{
		SystemPrompt:    systemPrompt,
		UserPrompt:      userPrompt,
		MaxOutputTokens: maxTokens,
		Temperature:     0.1,
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

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}
	return result, nil
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
