package services

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"solusphere_backend/internal/ai"
)

var openAIOnce sync.Once

type OpenAIService struct {
	client *ai.OpenAIClient
}

type AgentResponse struct {
	Reply            string        `json:"reply"`
	Sources          []ai.Citation `json:"sources,omitempty"`
	SourceCount      int           `json:"source_count"`
	Model            string        `json:"model"`
	WebSearchEnabled bool          `json:"web_search_enabled"`
}

func InitOpenAI() {
	openAIOnce.Do(func() {
		if err := ai.InitOpenAIFromEnv(); err != nil {
			fmt.Printf("WARNING: %v\n", err)
		}
	})
}

func InitOpenAIWithKey(key, model string) {
	openAIOnce.Do(func() {
		model = ai.NormalizeOpenAIModel(model)
		if err := ai.InitOpenAI(key, model); err != nil {
			fmt.Printf("WARNING: %v\n", err)
			return
		}
		os.Setenv("OPENAI_API_KEY", key)
		os.Setenv("OPENAI_MODEL", model)
	})
}

func RefreshOpenAIKey(key, model string) {
	openAIOnce = sync.Once{}
	InitOpenAIWithKey(key, model)
	fmt.Println("OpenAI API key refreshed")
}

func IsOpenAIInitialized() bool {
	return ai.IsOpenAIConfigured()
}

func GetOpenAIModel() string {
	return ai.GetOpenAIModel()
}

func GetBPOResponse(userMessage string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return ai.GenerateText(ctx, ai.GenerateTextRequest{
		SystemPrompt:    "You are SIA (Smart Intelligence Assistant), a helpful and professional AI assistant. Respond politely, professionally, and concisely.",
		UserPrompt:      userMessage,
		MaxOutputTokens: 500,
		Temperature:     0.7,
	})
}

func GetAgentResponse(userMessage string, webSearch bool) (*AgentResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Second)
	defer cancel()

	result, err := ai.GenerateTextResultWithDefault(ctx, ai.GenerateTextRequest{
		SystemPrompt: `You are SIA, Solusphere's AI operations and analytics agent.
Answer with accuracy, practical reasoning, and clear uncertainty.
When web search is available and the question may depend on current public information, search the web before answering.
Research must be based on multiple independent sources whenever available.
For research, website analysis, market analysis, competitor analysis, news, product, legal, financial, technical, or current-fact questions, use and cite at least three different credible sources when available.
Prefer source diversity: official/company sources for primary facts, reputable news or industry sources for context, documentation or standards for technical facts, and independent analysis when relevant.
Do not rely on a single source for a research answer unless only one credible source is available; if fewer than three sources are available, say that clearly.
Compare sources when they disagree and explain which source is most authoritative for each claim.
For website analysis, inspect and summarize the public facts you can verify, then give actionable recommendations.
For analytics questions, structure the answer with findings, likely drivers, risks, and next actions.
Do not invent facts, metrics, prices, policies, or source claims. Cite web sources when you use them.`,
		UserPrompt:      userMessage,
		MaxOutputTokens: 1200,
		Temperature:     0.3,
		WebSearch:       webSearch,
	})
	if err != nil {
		return nil, err
	}

	return &AgentResponse{
		Reply:            result.Text,
		Sources:          result.Citations,
		SourceCount:      len(result.Citations),
		Model:            result.Model,
		WebSearchEnabled: webSearch,
	}, nil
}

func GetBPOResponseWithContext(userMessage string, history []map[string]string) (string, error) {
	messages := make([]ai.Message, 0, len(history)+1)
	for _, msg := range history {
		messages = append(messages, ai.Message{
			Role:    msg["role"],
			Content: msg["content"],
		})
	}
	messages = append(messages, ai.Message{Role: "user", Content: userMessage})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return ai.GenerateText(ctx, ai.GenerateTextRequest{
		SystemPrompt:    "You are SIA (Smart Intelligence Assistant), a helpful and professional AI assistant. Respond politely and professionally.",
		Messages:        messages,
		MaxOutputTokens: 500,
		Temperature:     0.7,
	})
}

func TestOpenAIConnection() error {
	_, err := GetBPOResponse("Hello, are you working?")
	return err
}

func NewOpenAIService(apiKey, model string) (*OpenAIService, error) {
	client, err := ai.NewOpenAIClient(apiKey, model)
	if err != nil {
		return nil, err
	}
	return &OpenAIService{client: client}, nil
}

func (o *OpenAIService) AnalyzeContent(ctx context.Context, prompt string) (string, error) {
	if o == nil || o.client == nil {
		return "", fmt.Errorf("OpenAI service is not initialized")
	}

	result, err := o.client.GenerateText(ctx, ai.GenerateTextRequest{
		SystemPrompt:    "You are a BPO document analysis assistant. Extract concise, accurate business information from documents.",
		UserPrompt:      prompt,
		MaxOutputTokens: 2048,
		Temperature:     0.1,
	})
	if err != nil {
		return "", err
	}

	log.Printf("OpenAI analysis completed, response length: %d", len(result))
	return result, nil
}

func (o *OpenAIService) Close() {}
