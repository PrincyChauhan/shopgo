package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var ErrNotConfigured = errors.New("AI is not configured")

// Client calls either Google Gemini's native generateContent API (supports AQ.
// and AIza keys via x-goog-api-key) or any OpenAI-compatible /chat/completions
// endpoint (Groq, OpenRouter, or local Ollama).
type Client struct {
	BaseURL string
	APIKey  string
	Model   string
	HTTP    *http.Client
}

func NewClient(baseURL, apiKey, model string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		APIKey:  strings.TrimSpace(apiKey),
		Model:   strings.TrimSpace(model),
		HTTP:    &http.Client{Timeout: 20 * time.Second},
	}
}

func (c *Client) Enabled() bool {
	if c == nil || c.BaseURL == "" || c.Model == "" {
		return false
	}
	if c.APIKey != "" {
		return true
	}
	// Local servers like Ollama do not require an API key.
	return strings.Contains(c.BaseURL, "localhost") || strings.Contains(c.BaseURL, "127.0.0.1")
}

func (c *Client) isGemini() bool {
	return strings.Contains(c.BaseURL, "generativelanguage.googleapis.com") ||
		strings.Contains(c.BaseURL, "aiplatform.googleapis.com")
}

// Chat sends a system + user prompt and returns the trimmed assistant reply.
func (c *Client) Chat(ctx context.Context, systemPrompt, userPrompt string, maxTokens int) (string, error) {
	if !c.Enabled() {
		return "", ErrNotConfigured
	}
	if c.isGemini() {
		return c.chatGemini(ctx, systemPrompt, userPrompt, maxTokens)
	}
	return c.chatOpenAI(ctx, systemPrompt, userPrompt, maxTokens)
}

// ---- Native Google Gemini (generativelanguage.googleapis.com) ----

type geminiPart struct {
	Text string `json:"text"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiGenConfig struct {
	Temperature     float64 `json:"temperature"`
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
}

type geminiRequest struct {
	SystemInstruction *geminiContent  `json:"system_instruction,omitempty"`
	Contents          []geminiContent `json:"contents"`
	GenerationConfig  geminiGenConfig `json:"generationConfig"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []geminiPart `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (c *Client) chatGemini(ctx context.Context, systemPrompt, userPrompt string, maxTokens int) (string, error) {
	base := strings.TrimSuffix(c.BaseURL, "/openai")
	endpoint := fmt.Sprintf("%s/models/%s:generateContent", base, c.Model)

	// Gemini 2.5 Flash can use reasoning tokens inside maxOutputTokens, so keep a safe floor.
	outLimit := maxTokens * 4
	if outLimit < 1024 {
		outLimit = 1024
	}

	reqBody := geminiRequest{
		Contents: []geminiContent{
			{Role: "user", Parts: []geminiPart{{Text: userPrompt}}},
		},
		GenerationConfig: geminiGenConfig{
			Temperature:     0.3,
			MaxOutputTokens: outLimit,
		},
	}
	if strings.TrimSpace(systemPrompt) != "" {
		reqBody.SystemInstruction = &geminiContent{
			Parts: []geminiPart{{Text: systemPrompt}},
		}
	}

	headers := map[string]string{"x-goog-api-key": c.APIKey}
	resp, raw, err := c.postJSON(ctx, endpoint, headers, reqBody)
	if err != nil {
		return "", err
	}

	var parsed geminiResponse
	_ = json.Unmarshal(raw, &parsed)

	if resp.StatusCode >= 400 {
		if parsed.Error != nil && parsed.Error.Message != "" {
			return "", fmt.Errorf("AI provider error (%d): %s", resp.StatusCode, parsed.Error.Message)
		}
		return "", fmt.Errorf("AI provider returned HTTP %d", resp.StatusCode)
	}
	if len(parsed.Candidates) == 0 || len(parsed.Candidates[0].Content.Parts) == 0 {
		return "", errors.New("AI provider returned no candidates")
	}

	var sb strings.Builder
	for _, p := range parsed.Candidates[0].Content.Parts {
		sb.WriteString(p.Text)
	}
	text := strings.TrimSpace(sb.String())
	if text == "" {
		return "", errors.New("AI provider returned empty response")
	}
	return text, nil
}

// ---- OpenAI-compatible (/chat/completions) for Groq, OpenRouter, Ollama ----

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (c *Client) chatOpenAI(ctx context.Context, systemPrompt, userPrompt string, maxTokens int) (string, error) {
	payload := chatRequest{
		Model: c.Model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: 0.3,
		MaxTokens:   maxTokens,
	}

	headers := map[string]string{}
	if c.APIKey != "" {
		headers["Authorization"] = "Bearer " + c.APIKey
	}

	resp, raw, err := c.postJSON(ctx, c.BaseURL+"/chat/completions", headers, payload)
	if err != nil {
		return "", err
	}

	var parsed chatResponse
	_ = json.Unmarshal(raw, &parsed)

	if resp.StatusCode >= 400 {
		if parsed.Error != nil && parsed.Error.Message != "" {
			return "", fmt.Errorf("AI provider error (%d): %s", resp.StatusCode, parsed.Error.Message)
		}
		return "", fmt.Errorf("AI provider returned HTTP %d", resp.StatusCode)
	}
	if len(parsed.Choices) == 0 {
		return "", errors.New("AI provider returned no choices")
	}

	text := strings.TrimSpace(parsed.Choices[0].Message.Content)
	if text == "" {
		return "", errors.New("AI provider returned empty response")
	}
	return text, nil
}

func (c *Client) postJSON(ctx context.Context, endpoint string, headers map[string]string, payload any) (*http.Response, []byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, nil, err
	}
	return resp, raw, nil
}
