package helpers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io" // Use modern io package
	"net/http"
	"net/url" // For encoding the API key in the URL
	"os"
)

// --- Request and Response Structures for the Real Gemini REST API ---

// Part represents a text prompt
type Part struct {
	Text string `json:"text"`
}

// Content represents the list of parts in the request
type Content struct {
	Parts []Part `json:"parts"`
}

// GeminiRequest models the generateContent request payload
type GeminiRequest struct {
	// The contents field holds the prompt for the model.
	Contents []Content `json:"contents"`
}

// Candidate represents one generated response
type Candidate struct {
	Content Content `json:"content"`
}

// GeminiResponse models the generateContent response payload
type GeminiResponse struct {
	Candidates []Candidate `json:"candidates"`
}

// CallGeminiAI sends a prompt to the real Gemini API and returns the generated result
func CallGeminiAI(prompt string) (string, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("GEMINI_API_KEY not set")
	}

	// 1. Construct the correct request body
	body := GeminiRequest{
		Contents: []Content{
			{
				Parts: []Part{
					{Text: prompt},
				},
			},
		},
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request body: %w", err)
	}

	// 2. Construct the correct URL with API Key as a query parameter
	modelName := "gemini-2.5-flash" // Use a current, fast model
	endpoint := fmt.Sprintf("https://generativelanguage.googleapis.com/v1/models/%s:generateContent", modelName)

	// Add the API key to the URL as a query parameter
	u, _ := url.Parse(endpoint)
	q := u.Query()
	q.Set("key", apiKey)
	u.RawQuery = q.Encode()

	// 3. Create the HTTP request
	req, err := http.NewRequest("POST", u.String(), bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", err
	}

	// No Authorization header needed, API Key is in the URL.
	req.Header.Set("Content-Type", "application/json")

	// 4. Send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// 5. Read and check the response
	respBody, err := io.ReadAll(resp.Body) // Use io.ReadAll
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// Return detailed error for non-200 status codes
		return "", fmt.Errorf("Gemini API error: Status %d, Body: %s", resp.StatusCode, string(respBody))
	}

	// 6. Unmarshal and parse the response
	var result GeminiResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Check if a candidate (response) exists and return the text
	if len(result.Candidates) > 0 && len(result.Candidates[0].Content.Parts) > 0 {
		return result.Candidates[0].Content.Parts[0].Text, nil
	}

	return "AI output unavailable or empty response structure", nil
}
