package models

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/go-resty/resty/v2"
)

var geminiClient *resty.Client
var geminiAPIKey string

// InitGemini initializes the Gemini client and API key from environment variables.
func InitGemini() {
	geminiAPIKey = os.Getenv("GEMINI_API_KEY")
	geminiClient = resty.New()
}

// GetBPOResponse sends a message to the Gemini API with a BPO agent persona
// and returns the model's response.
func GetBPOResponse(userMessage string) (string, error) {
	if geminiClient == nil {
		return "", fmt.Errorf("Gemini client is not initialized. Call InitGemini() first")
	}
	if geminiAPIKey == "" {
		return "", fmt.Errorf("GEMINI_API_KEY is empty")
	}

	// Use the v1beta generateContent endpoint
	// The API Key is passed as a query parameter, not a Bearer token
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash-preview-09-2025:generateContent?key=%s", geminiAPIKey)

	// System instruction defines the model's persona
	systemInstruction := map[string]interface{}{
		"parts": []map[string]interface{}{
			{
				"text": "You are an experienced BPO agent. Respond politely and professionally.",
			},
		},
	}

	// User's message
	userContent := map[string]interface{}{
		"role": "user",
		"parts": []map[string]interface{}{
			{
				"text": userMessage,
			},
		},
	}

	// Updated payload structure for v1beta
	payload := map[string]interface{}{
		"contents":          []map[string]interface{}{userContent},
		"systemInstruction": systemInstruction,
		"generationConfig": map[string]interface{}{
			"temperature":     0.7,
			"maxOutputTokens": 500,
		},
	}

	resp, err := geminiClient.R().
		SetHeader("Content-Type", "application/json").
		SetBody(payload).
		Post(url)

	if err != nil {
		return "", fmt.Errorf("error making request to Gemini: %w", err)
	}

	if resp.IsError() {
		return "", fmt.Errorf("gemini API returned an error: %s", resp.String())
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return "", fmt.Errorf("error parsing Gemini response: %w", err)
	}

	// Parse the new response structure
	// candidates[0].content.parts[0].text
	if candidates, ok := result["candidates"].([]interface{}); ok && len(candidates) > 0 {
		firstCandidate := candidates[0].(map[string]interface{})
		if content, ok := firstCandidate["content"].(map[string]interface{}); ok {
			if parts, ok := content["parts"].([]interface{}); ok && len(parts) > 0 {
				firstPart := parts[0].(map[string]interface{})
				if text, ok := firstPart["text"].(string); ok {
					return text, nil
				}
			}
		}
	}

	// Provide a more detailed error if parsing fails
	return "", fmt.Errorf("no valid 'text' content found in Gemini response. Response body: %s", resp.String())
}
