package services

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

type GeminiService struct {
	client *genai.Client
	model  *genai.GenerativeModel
}

func NewGeminiService(apiKey string) (*GeminiService, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY is required")
	}

	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %v", err)
	}

	model := client.GenerativeModel("gemini-pro")

	// Configure generation parameters
	model.SetTemperature(0.1)
	model.SetTopK(40)
	model.SetTopP(0.8)
	model.SetMaxOutputTokens(2048)

	return &GeminiService{
		client: client,
		model:  model,
	}, nil
}

func (g *GeminiService) AnalyzeContent(ctx context.Context, prompt string) (string, error) {
	resp, err := g.model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", fmt.Errorf("failed to generate content: %v", err)
	}

	if resp == nil || len(resp.Candidates) == 0 {
		return "", fmt.Errorf("no content generated from Gemini")
	}

	// Extract text from response
	var result strings.Builder
	for _, part := range resp.Candidates[0].Content.Parts {
		if text, ok := part.(genai.Text); ok {
			result.WriteString(string(text))
		}
	}

	if result.Len() == 0 {
		return "", fmt.Errorf("empty response from Gemini")
	}

	log.Printf("Gemini analysis completed, response length: %d", result.Len())
	return result.String(), nil
}

func (g *GeminiService) Close() {
	if g.client != nil {
		g.client.Close()
	}
}
