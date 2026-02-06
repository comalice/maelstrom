package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
)

type LLMConfig struct {
	Provider   string
	Endpoint   string
	Model      string
	APIKey     string
	Temp       float64
	MaxTokens  int
}

type Caller interface {
	Call(context.Context, LLMConfig, string) (string, error)
}

type HTTPClient struct{}

func (h *HTTPClient) Call(ctx context.Context, cfg LLMConfig, prompt string) (string, error) {
	var url string
	var headers map[string]string
	var payload map[string]any

	switch cfg.Provider {
	case "anthropic":
		url = cfg.Endpoint + "/v1/messages"
		headers = map[string]string{
			"Content-Type":      "application/json",
			"x-api-key":         cfg.APIKey,
			"anthropic-version": "2023-06-01",
		}
		payload = map[string]any{
			"model":       cfg.Model,
			"max_tokens":  cfg.MaxTokens,
			"temperature": cfg.Temp,
			"messages": []map[string]string{{
				"role": "user",
				"content": prompt,
			}},
		}
	case "openai":
		url = cfg.Endpoint + "/v1/chat/completions"
		headers = map[string]string{
			"Content-Type":   "application/json",
			"Authorization":  "Bearer " + cfg.APIKey,
		}
		payload = map[string]any{
			"model":       cfg.Model,
			"max_tokens":  cfg.MaxTokens,
			"temperature": cfg.Temp,
			"messages": []map[string]string{{
				"role": "user",
				"content": prompt,
			}},
		}
	case "openrouter":
		url = cfg.Endpoint + "/api/v1/chat/completions"
		headers = map[string]string{
			"Content-Type":   "application/json",
			"Authorization":  "Bearer " + cfg.APIKey,
			"HTTP-Referer":   "https://maelstrom-stillpoint.com",
			"X-Title":        "Maelstrom CLI Demo",
		}
		payload = map[string]any{
			"model":       cfg.Model,
			"max_tokens":  cfg.MaxTokens,
			"temperature": cfg.Temp,
			"messages": []map[string]string{{
				"role": "user",
				"content": prompt,
			}},
		}
	default:
		return "", fmt.Errorf("unsupported LLM provider: %s", cfg.Provider)
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payloadBytes))
	if err != nil {
		return "", fmt.Errorf("new request: %w", err)
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("http do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("http %d: %s", resp.StatusCode, string(body))
	}

	if cfg.Provider == "anthropic" {
		var ar struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil {
			return "", fmt.Errorf("decode anthropic resp: %w", err)
		}
		if len(ar.Content) > 0 {
			return ar.Content[0].Text, nil
		}
	} else {
		var ar struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil {
			return "", fmt.Errorf("decode openai resp: %w", err)
		}
		if len(ar.Choices) > 0 {
			return ar.Choices[0].Message.Content, nil
		}
	}

	return "", fmt.Errorf("no content in response")
}

var DefaultCaller Caller = &HTTPClient{}

func Call(ctx context.Context, cfg LLMConfig, prompt string) (string, error) {
	return DefaultCaller.Call(ctx, cfg, prompt)
}

type CallRec struct {
	Config LLMConfig
	Prompt string
}

type MockCaller struct {
	mu    sync.Mutex
	Calls []CallRec
}

func (m *MockCaller) Call(ctx context.Context, cfg LLMConfig, prompt string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Calls = append(m.Calls, CallRec{Config: cfg, Prompt: prompt})
	return "{}", nil
}

func (m *MockCaller) ResetCalls() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Calls = nil
}