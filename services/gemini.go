package services

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/go-resty/resty/v2"
)

var (
	apiKey     string
	client     *resty.Client
	once       sync.Once
	clientOnce sync.Once
)

// InitGemini initializes the Gemini service with API key from environment
func InitGemini() {
	once.Do(func() {
		apiKey = os.Getenv("GEMINI_API_KEY")
		if apiKey == "" {
			fmt.Println("WARNING: GEMINI_API_KEY not set")
		}
		initClient()
	})
}

// InitGeminiWithKey initializes the Gemini service with a provided API key
// This should be called after loading secrets from AWS Secrets Manager
func InitGeminiWithKey(key string) {
	once.Do(func() {
		apiKey = key
		// Also set environment variable for backward compatibility
		os.Setenv("GEMINI_API_KEY", key)
		initClient()
	})
}

// RefreshGeminiKey refreshes the Gemini API key (useful when secrets are rotated)
func RefreshGeminiKey(key string) {
	// Reset the once so we can reinitialize
	once = sync.Once{}
	// Reinitialize with new key
	InitGeminiWithKey(key)
	fmt.Println("Gemini API key refreshed")
}

// initClient initializes the HTTP client
func initClient() {
	clientOnce.Do(func() {
		client = resty.New()
		// Set reasonable timeouts
		client.SetTimeout(30) // 30 seconds timeout
		// Enable retries for transient errors
		client.SetRetryCount(3)
		client.SetRetryWaitTime(1)  // Wait 1 second between retries
		client.SetRetryMaxWaitTime(5) // Max 5 seconds total retry time
	})
}

// GetGeminiAPIKey returns the current API key (useful for debugging)
func GetGeminiAPIKey() string {
	return apiKey
}

// IsGeminiInitialized checks if the Gemini service is initialized
func IsGeminiInitialized() bool {
	return apiKey != "" && client != nil
}

// GetBPOResponse sends a message to Gemini and returns the response
func GetBPOResponse(userMessage string) (string, error) {
	// Ensure Gemini is initialized
	if !IsGeminiInitialized() {
		InitGemini() // Try to initialize from environment as fallback
	}

	if apiKey == "" {
		return "", fmt.Errorf("GEMINI_API_KEY not set")
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-pro:generateContent?key=%s", apiKey)

	// Prepare the request payload with correct format
	payload := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]interface{}{
					{
						"text": fmt.Sprintf("You are SIA (Smart Intelligence Assistant), a helpful and professional AI assistant. Respond politely and professionally to the following message:\n\n%s", userMessage),
					},
				},
			},
		},
		"generationConfig": map[string]interface{}{
			"temperature":     0.7,
			"maxOutputTokens": 500,
			"topP":           0.95,
			"topK":            40,
		},
		"safetySettings": []map[string]interface{}{
			{
				"category":  "HARM_CATEGORY_HARASSMENT",
				"threshold": "BLOCK_MEDIUM_AND_ABOVE",
			},
			{
				"category":  "HARM_CATEGORY_HATE_SPEECH",
				"threshold": "BLOCK_MEDIUM_AND_ABOVE",
			},
			{
				"category":  "HARM_CATEGORY_SEXUALLY_EXPLICIT",
				"threshold": "BLOCK_MEDIUM_AND_ABOVE",
			},
			{
				"category":  "HARM_CATEGORY_DANGEROUS_CONTENT",
				"threshold": "BLOCK_MEDIUM_AND_ABOVE",
			},
		},
	}

	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetBody(payload).
		Post(url)

	if err != nil {
		return "", fmt.Errorf("API request failed: %v", err)
	}

	if resp.StatusCode() != 200 {
		// Try to parse error response
		var errorResponse map[string]interface{}
		if jsonErr := json.Unmarshal(resp.Body(), &errorResponse); jsonErr == nil {
			if errorMsg, ok := errorResponse["error"].(map[string]interface{}); ok {
				if message, ok := errorMsg["message"].(string); ok {
					return "", fmt.Errorf("Gemini API error (status %d): %s", resp.StatusCode(), message)
				}
			}
		}
		return "", fmt.Errorf("Gemini API returned status %d: %s", resp.StatusCode(), string(resp.Body()))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %v", err)
	}

	// Extract text from the response with better error handling
	text, err := extractResponseText(result)
	if err != nil {
		return "", err
	}

	return text, nil
}

// extractResponseText extracts the text from Gemini API response
func extractResponseText(result map[string]interface{}) (string, error) {
	// Check if there's an error in the response
	if errorObj, ok := result["error"]; ok {
		if errorMap, ok := errorObj.(map[string]interface{}); ok {
			if message, ok := errorMap["message"].(string); ok {
				return "", fmt.Errorf("Gemini API error: %s", message)
			}
		}
		return "", fmt.Errorf("Gemini API returned an error")
	}

	// Check candidates
	candidates, ok := result["candidates"].([]interface{})
	if !ok || len(candidates) == 0 {
		// Check for blocked content
		if promptFeedback, ok := result["promptFeedback"].(map[string]interface{}); ok {
			if blockReason, ok := promptFeedback["blockReason"].(string); ok {
				return "", fmt.Errorf("content blocked: %s", blockReason)
			}
		}
		return "", fmt.Errorf("no candidates in response")
	}

	candidate, ok := candidates[0].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid candidate format")
	}

	// Check if content was blocked
	if finishReason, ok := candidate["finishReason"].(string); ok {
		if finishReason != "STOP" && finishReason != "MAX_TOKENS" {
			return "", fmt.Errorf("content generation stopped: %s", finishReason)
		}
	}

	content, ok := candidate["content"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid content format")
	}

	parts, ok := content["parts"].([]interface{})
	if !ok || len(parts) == 0 {
		return "", fmt.Errorf("no parts in content")
	}

	part, ok := parts[0].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid part format")
	}

	text, ok := part["text"].(string)
	if !ok {
		return "", fmt.Errorf("no text in response")
	}

	return text, nil
}

// GetBPOResponseWithContext sends a message with conversation history
func GetBPOResponseWithContext(userMessage string, history []map[string]string) (string, error) {
	if !IsGeminiInitialized() {
		InitGemini()
	}

	if apiKey == "" {
		return "", fmt.Errorf("GEMINI_API_KEY not set")
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-pro:generateContent?key=%s", apiKey)

	// Build contents with history
	contents := make([]map[string]interface{}, 0)
	
	// Add system prompt as first message
	contents = append(contents, map[string]interface{}{
		"role": "user",
		"parts": []map[string]interface{}{
			{
				"text": "You are SIA (Smart Intelligence Assistant), a helpful and professional AI assistant. Respond politely and professionally.",
			},
		},
	})

	// Add conversation history
	for _, msg := range history {
		role := msg["role"]
		content := msg["content"]
		contents = append(contents, map[string]interface{}{
			"role": role,
			"parts": []map[string]interface{}{
				{
					"text": content,
				},
			},
		})
	}

	// Add current user message
	contents = append(contents, map[string]interface{}{
		"role": "user",
		"parts": []map[string]interface{}{
			{
				"text": userMessage,
			},
		},
	})

	payload := map[string]interface{}{
		"contents": contents,
		"generationConfig": map[string]interface{}{
			"temperature":     0.7,
			"maxOutputTokens": 500,
			"topP":           0.95,
			"topK":            40,
		},
	}

	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetBody(payload).
		Post(url)

	if err != nil {
		return "", fmt.Errorf("API request failed: %v", err)
	}

	if resp.StatusCode() != 200 {
		return "", fmt.Errorf("API returned status %d: %s", resp.StatusCode(), string(resp.Body()))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %v", err)
	}

	text, err := extractResponseText(result)
	if err != nil {
		return "", err
	}

	return text, nil
}

// TestGeminiConnection tests if the Gemini API is working
func TestGeminiConnection() error {
	if !IsGeminiInitialized() {
		InitGemini()
	}

	if apiKey == "" {
		return fmt.Errorf("GEMINI_API_KEY not set")
	}

	// Simple test message
	_, err := GetBPOResponse("Hello, are you working?")
	return err
}