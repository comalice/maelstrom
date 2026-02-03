package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type LLMConfig struct {
	Provider   string
	Endpoint   string
	Model      string
	APIKey     string
	Temp       float64
	MaxTokens  int
}

// Global config not used - per-spec LLMConfig

func Call(ctx context.Context, cfg LLMConfig, prompt string) (string, error) {
	var url string
	var headers map[string]string
	var payload map[string]interface{}

	switch cfg.Provider {
	case "anthropic":
		url = cfg.Endpoint + "/v1/messages"
		headers = map[string]string{
			"Content-Type":      "application/json",
			"x-api-key":         cfg.APIKey,
			"anthropic-version": "2023-06-01",
		}
		payload = map[string]interface{}{
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
			"Content-Type":  "application/json",
			"Authorization": "Bearer " + cfg.APIKey,
		}
		payload = map[string]interface{}{
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