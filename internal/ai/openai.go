package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
)

const (
	DefaultOpenAIModel = "gpt-4.1-mini"
	responsesEndpoint  = "https://api.openai.com/v1/responses"
)

type Message struct {
	Role    string
	Content string
}

type GenerateTextRequest struct {
	SystemPrompt    string
	UserPrompt      string
	Messages        []Message
	MaxOutputTokens int
	Temperature     float64
	WebSearch       bool
}

type OpenAIClient struct {
	apiKey string
	model  string
	client *resty.Client
}

type Citation struct {
	Title string `json:"title,omitempty"`
	URL   string `json:"url"`
}

type GenerateTextResult struct {
	Text      string     `json:"text"`
	Citations []Citation `json:"citations,omitempty"`
	Model     string     `json:"model"`
}

var (
	defaultMu     sync.RWMutex
	defaultClient *OpenAIClient
)

func NewOpenAIClient(apiKey, model string) (*OpenAIClient, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, errors.New("OPENAI_API_KEY is required")
	}

	model = NormalizeOpenAIModel(model)

	client := resty.New().
		SetTimeout(60 * time.Second).
		SetRetryCount(2).
		SetRetryWaitTime(500 * time.Millisecond).
		SetRetryMaxWaitTime(3 * time.Second)

	return &OpenAIClient{
		apiKey: apiKey,
		model:  model,
		client: client,
	}, nil
}

func InitOpenAI(apiKey, model string) error {
	client, err := NewOpenAIClient(apiKey, model)
	if err != nil {
		return err
	}

	defaultMu.Lock()
	defaultClient = client
	defaultMu.Unlock()
	return nil
}

func InitOpenAIFromEnv() error {
	return InitOpenAI(os.Getenv("OPENAI_API_KEY"), os.Getenv("OPENAI_MODEL"))
}

func IsOpenAIConfigured() bool {
	defaultMu.RLock()
	defer defaultMu.RUnlock()
	return defaultClient != nil
}

func GetOpenAIModel() string {
	defaultMu.RLock()
	defer defaultMu.RUnlock()
	if defaultClient == nil {
		return defaultModel()
	}
	return defaultClient.model
}

func GenerateText(ctx context.Context, req GenerateTextRequest) (string, error) {
	defaultMu.RLock()
	client := defaultClient
	defaultMu.RUnlock()

	if client == nil {
		if err := InitOpenAIFromEnv(); err != nil {
			return "", err
		}
		defaultMu.RLock()
		client = defaultClient
		defaultMu.RUnlock()
	}

	result, err := client.GenerateTextResult(ctx, req)
	if err != nil {
		return "", err
	}
	return result.Text, nil
}

func (c *OpenAIClient) GenerateText(ctx context.Context, req GenerateTextRequest) (string, error) {
	result, err := c.GenerateTextResult(ctx, req)
	if err != nil {
		return "", err
	}
	return result.Text, nil
}

func GenerateTextResultWithDefault(ctx context.Context, req GenerateTextRequest) (GenerateTextResult, error) {
	defaultMu.RLock()
	client := defaultClient
	defaultMu.RUnlock()

	if client == nil {
		if err := InitOpenAIFromEnv(); err != nil {
			return GenerateTextResult{}, err
		}
		defaultMu.RLock()
		client = defaultClient
		defaultMu.RUnlock()
	}

	return client.GenerateTextResult(ctx, req)
}

func (c *OpenAIClient) GenerateTextResult(ctx context.Context, req GenerateTextRequest) (GenerateTextResult, error) {
	if c == nil {
		return GenerateTextResult{}, errors.New("OpenAI client is not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	maxTokens := req.MaxOutputTokens
	if maxTokens <= 0 {
		maxTokens = 500
	}
	if maxTokens < 16 {
		maxTokens = 16
	}

	payload := map[string]interface{}{
		"model":             c.model,
		"input":             buildInput(req),
		"max_output_tokens": maxTokens,
	}
	if req.SystemPrompt != "" {
		payload["instructions"] = req.SystemPrompt
	}
	if req.Temperature > 0 {
		payload["temperature"] = req.Temperature
	}
	if req.WebSearch {
		payload["tools"] = []map[string]interface{}{
			{
				"type":                "web_search",
				"search_context_size": "high",
			},
		}
		payload["tool_choice"] = "auto"
		payload["include"] = []string{"web_search_call.action.sources"}
	}

	resp, err := c.client.R().
		SetContext(ctx).
		SetAuthToken(c.apiKey).
		SetHeader("Content-Type", "application/json").
		SetBody(payload).
		Post(responsesEndpoint)
	if err != nil {
		return GenerateTextResult{}, fmt.Errorf("OpenAI API request failed: %w", err)
	}
	if resp.IsError() {
		return GenerateTextResult{}, fmt.Errorf("OpenAI API error (status %d): %s", resp.StatusCode(), extractAPIError(resp.Body()))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return GenerateTextResult{}, fmt.Errorf("failed to parse OpenAI response: %w", err)
	}

	text := extractOutputText(result)
	if text == "" {
		return GenerateTextResult{}, fmt.Errorf("OpenAI response did not contain text output")
	}

	return GenerateTextResult{
		Text:      text,
		Citations: extractCitations(result),
		Model:     c.model,
	}, nil
}

func buildInput(req GenerateTextRequest) []map[string]string {
	messages := req.Messages
	if len(messages) == 0 {
		messages = []Message{{Role: "user", Content: req.UserPrompt}}
	}

	input := make([]map[string]string, 0, len(messages))
	for _, msg := range messages {
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}

		role := normalizeRole(msg.Role)
		input = append(input, map[string]string{
			"role":    role,
			"content": content,
		})
	}

	if len(input) == 0 {
		input = append(input, map[string]string{
			"role":    "user",
			"content": req.UserPrompt,
		})
	}

	return input
}

func normalizeRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "assistant", "system", "developer":
		return strings.ToLower(strings.TrimSpace(role))
	case "model":
		return "assistant"
	default:
		return "user"
	}
}

func extractAPIError(body []byte) string {
	var parsed map[string]interface{}
	if err := json.Unmarshal(body, &parsed); err == nil {
		if errObj, ok := parsed["error"].(map[string]interface{}); ok {
			if message, ok := errObj["message"].(string); ok && message != "" {
				return message
			}
		}
	}

	return string(body)
}

func extractOutputText(result map[string]interface{}) string {
	if text, ok := result["output_text"].(string); ok && strings.TrimSpace(text) != "" {
		return strings.TrimSpace(text)
	}

	output, ok := result["output"].([]interface{})
	if !ok {
		return ""
	}

	var builder strings.Builder
	for _, outputItem := range output {
		item, ok := outputItem.(map[string]interface{})
		if !ok {
			continue
		}

		contentItems, ok := item["content"].([]interface{})
		if !ok {
			continue
		}

		for _, contentItem := range contentItems {
			content, ok := contentItem.(map[string]interface{})
			if !ok {
				continue
			}

			if contentType, _ := content["type"].(string); contentType == "refusal" {
				if refusal, _ := content["refusal"].(string); refusal != "" {
					builder.WriteString(refusal)
				}
				continue
			}

			if text, _ := content["text"].(string); text != "" {
				builder.WriteString(text)
			}
		}
	}

	return strings.TrimSpace(builder.String())
}

func extractCitations(result map[string]interface{}) []Citation {
	seen := make(map[string]struct{})
	citations := make([]Citation, 0)

	var walk func(value interface{})
	walk = func(value interface{}) {
		switch typed := value.(type) {
		case map[string]interface{}:
			if citation, ok := citationFromMap(typed); ok {
				if _, exists := seen[citation.URL]; !exists {
					seen[citation.URL] = struct{}{}
					citations = append(citations, citation)
				}
			}
			for _, child := range typed {
				walk(child)
			}
		case []interface{}:
			for _, child := range typed {
				walk(child)
			}
		}
	}

	walk(result)
	return citations
}

func citationFromMap(value map[string]interface{}) (Citation, bool) {
	citationType, _ := value["type"].(string)
	if citationType != "url_citation" && citationType != "citation" {
		return Citation{}, false
	}

	url, _ := value["url"].(string)
	url = strings.TrimSpace(url)
	if url == "" {
		return Citation{}, false
	}

	title, _ := value["title"].(string)
	return Citation{
		Title: strings.TrimSpace(title),
		URL:   url,
	}, true
}

func defaultModel() string {
	return NormalizeOpenAIModel(os.Getenv("OPENAI_MODEL"))
}

// NormalizeOpenAIModel keeps accidental secret values out of model fields.
func NormalizeOpenAIModel(model string) string {
	model = strings.TrimSpace(model)
	if model == "" || looksLikeOpenAIAPIKey(model) {
		return DefaultOpenAIModel
	}
	return model
}

func looksLikeOpenAIAPIKey(value string) bool {
	return strings.HasPrefix(strings.TrimSpace(value), "sk-")
}
