package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Provider is the common interface all AI providers implement.
type Provider interface {
	Name() string
	Generate(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}

// ----- Shared OpenAI-compatible types -----
// Used by OpenAI, Gemini (via OpenAI-compatible endpoint), Groq, and OpenRouter.

type openAIReq struct {
	Model    string          `json:"model"`
	Messages []openAIMessage `json:"messages"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIResp struct {
	Choices []struct {
		Message openAIMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// postOpenAICompat sends a chat-completion request to any OpenAI-compatible
// endpoint (OpenAI, Gemini shim, Groq, OpenRouter) with the given Bearer key
// and optional extra headers.
func postOpenAICompat(
	ctx context.Context,
	client *http.Client,
	endpoint, apiKey, providerName string,
	req openAIReq,
	extraHeaders map[string]string,
) (string, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	for k, v := range extraHeaders {
		httpReq.Header.Set(k, v)
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var parsed openAIResp
	if err := json.Unmarshal(data, &parsed); err != nil {
		return "", fmt.Errorf("%s: invalid JSON (status %d): %s", providerName, resp.StatusCode, truncate(string(data), 500))
	}
	if parsed.Error != nil {
		return "", fmt.Errorf("%s: %s - %s", providerName, parsed.Error.Type, parsed.Error.Message)
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("%s: HTTP %d: %s", providerName, resp.StatusCode, truncate(string(data), 500))
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("%s: no choices returned", providerName)
	}
	return parsed.Choices[0].Message.Content, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// ----- Anthropic (native API, different format) -----

type AnthropicProvider struct {
	apiKey string
	model  string
	client *http.Client
}

func NewAnthropicProvider(apiKey, model string) *AnthropicProvider {
	if model == "" {
		model = "claude-opus-4-7"
	}
	return &AnthropicProvider{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{Timeout: 120 * time.Second},
	}
}

func (p *AnthropicProvider) Name() string { return "anthropic" }

type anthropicReq struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResp struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

func (p *AnthropicProvider) Generate(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	reqBody := anthropicReq{
		Model:     p.model,
		MaxTokens: 4096,
		System:    systemPrompt,
		Messages:  []anthropicMessage{{Role: "user", Content: userPrompt}},
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var parsed anthropicResp
	if err := json.Unmarshal(data, &parsed); err != nil {
		return "", fmt.Errorf("anthropic: invalid JSON (status %d): %s", resp.StatusCode, truncate(string(data), 500))
	}
	if parsed.Error != nil {
		return "", fmt.Errorf("anthropic: %s - %s", parsed.Error.Type, parsed.Error.Message)
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("anthropic: HTTP %d: %s", resp.StatusCode, truncate(string(data), 500))
	}

	var sb strings.Builder
	for _, c := range parsed.Content {
		if c.Type == "text" {
			sb.WriteString(c.Text)
			sb.WriteString("\n")
		}
	}
	return sb.String(), nil
}

// ----- OpenAI -----

type OpenAIProvider struct {
	apiKey string
	model  string
	client *http.Client
}

func NewOpenAIProvider(apiKey, model string) *OpenAIProvider {
	if model == "" {
		model = "gpt-4o"
	}
	return &OpenAIProvider{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{Timeout: 120 * time.Second},
	}
}

func (p *OpenAIProvider) Name() string { return "openai" }

func (p *OpenAIProvider) Generate(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return postOpenAICompat(
		ctx,
		p.client,
		"https://api.openai.com/v1/chat/completions",
		p.apiKey,
		"openai",
		openAIReq{
			Model: p.model,
			Messages: []openAIMessage{
				{Role: "system", Content: systemPrompt},
				{Role: "user", Content: userPrompt},
			},
		},
		nil,
	)
}

// ----- Gemini (Google AI Studio) -----

type GeminiProvider struct {
	apiKey string
	model  string
	client *http.Client
}

func NewGeminiProvider(apiKey, model string) *GeminiProvider {
	if model == "" {
		model = "gemini-3-flash-preview"
	}
	return &GeminiProvider{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{Timeout: 120 * time.Second},
	}
}

func (p *GeminiProvider) Name() string { return "gemini" }

type geminiGenerateReq struct {
	SystemInstruction *geminiContent `json:"systemInstruction,omitempty"`
	Contents          []geminiContent `json:"contents"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiGenerateResp struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}

func (p *GeminiProvider) Generate(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent",
		p.model,
	)

	reqBody := geminiGenerateReq{
		SystemInstruction: &geminiContent{
			Parts: []geminiPart{
				{Text: systemPrompt},
			},
		},
		Contents: []geminiContent{
			{
				Parts: []geminiPart{
					{Text: userPrompt},
				},
			},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var parsed geminiGenerateResp
	if err := json.Unmarshal(data, &parsed); err != nil {
		return "", fmt.Errorf("gemini: invalid JSON (status %d): %s", resp.StatusCode, string(data))
	}

	if parsed.Error != nil {
		return "", fmt.Errorf("gemini: %s - %s", parsed.Error.Status, parsed.Error.Message)
	}

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("gemini: HTTP %d: %s", resp.StatusCode, string(data))
	}

	if len(parsed.Candidates) == 0 ||
		len(parsed.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("gemini: no candidates returned")
	}

	return parsed.Candidates[0].Content.Parts[0].Text, nil
}

// ----- Groq -----

type GroqProvider struct {
	apiKey string
	model  string
	client *http.Client
}

func NewGroqProvider(apiKey, model string) *GroqProvider {
	if model == "" {
		model = "llama-3.3-70b-versatile"
	}
	return &GroqProvider{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{Timeout: 120 * time.Second},
	}
}

func (p *GroqProvider) Name() string { return "groq" }

func (p *GroqProvider) Generate(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return postOpenAICompat(
		ctx,
		p.client,
		"https://api.groq.com/openai/v1/chat/completions",
		p.apiKey,
		"groq",
		openAIReq{
			Model: p.model,
			Messages: []openAIMessage{
				{Role: "system", Content: systemPrompt},
				{Role: "user", Content: userPrompt},
			},
		},
		nil,
	)
}

// ----- OpenRouter -----

type OpenRouterProvider struct {
	apiKey string
	model  string
	client *http.Client
}

func NewOpenRouterProvider(apiKey, model string) *OpenRouterProvider {
	if model == "" {
		model = "meta-llama/llama-3.3-70b-instruct:free"
	}
	return &OpenRouterProvider{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{Timeout: 120 * time.Second},
	}
}

func (p *OpenRouterProvider) Name() string { return "openrouter" }

func (p *OpenRouterProvider) Generate(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return postOpenAICompat(
		ctx,
		p.client,
		"https://openrouter.ai/api/v1/chat/completions",
		p.apiKey,
		"openrouter",
		openAIReq{
			Model: p.model,
			Messages: []openAIMessage{
				{Role: "system", Content: systemPrompt},
				{Role: "user", Content: userPrompt},
			},
		},
		map[string]string{
			"HTTP-Referer": "https://github.com/AnggaTechI/domgrab",
			"X-Title":      "domgrab",
		},
	)
}