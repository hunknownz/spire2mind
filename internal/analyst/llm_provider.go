package analyst

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"spire2mind/internal/config"
)

// openAIProvider uses an OpenAI-compatible API (Ollama, OpenAI, etc.)
type openAIProvider struct {
	baseURL string
	model   string
	apiKey  string
	client  *http.Client
}

func newLLMProvider(cfg config.Config) (LLMProvider, error) {
	baseURL := cfg.APIBaseURL
	if baseURL == "" {
		baseURL = "http://127.0.0.1:11434"
	}
	model := cfg.Model
	if model == "" {
		model = "qwen3.5:35b-a3b-coding-nvfp4"
	}

	return &openAIProvider{
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   model,
		apiKey:  cfg.APIKey,
		client:  &http.Client{Timeout: 5 * time.Minute},
	}, nil
}

func (p *openAIProvider) Complete(ctx context.Context, systemPrompt string, userPrompt string) (string, error) {
	messages := []map[string]string{
		{"role": "system", "content": systemPrompt},
		{"role": "user", "content": userPrompt},
	}

	body := map[string]interface{}{
		"model":    p.model,
		"messages": messages,
		"stream":   false,
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	url := p.baseURL + "/v1/chat/completions"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("LLM request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("LLM returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no choices in LLM response")
	}

	return result.Choices[0].Message.Content, nil
}
