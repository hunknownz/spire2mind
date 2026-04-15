package analyst

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"spire2mind/internal/config"
)

// openAIProvider uses an OpenAI-compatible API (Ollama, OpenAI, etc.)
type openAIProvider struct {
	baseURL string
	model   string
	apiKey  string
	client  *http.Client
}

func NewLLMProvider(cfg config.Config) (LLMProvider, error) {
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
		// No client-level timeout: rely on context cancellation.
		// Individual calls use stream:true to keep the connection alive during generation.
		client: &http.Client{},
	}, nil
}

// Complete sends a prompt to the LLM and returns the full response text.
// It uses Server-Sent Events (stream:true) so the connection stays alive
// during long generation runs instead of waiting for a buffered response.
// For Qwen3 models, thinking is disabled (think:false) to avoid generating
// thousands of internal reasoning tokens that can push generation past 10 min.
func (p *openAIProvider) Complete(ctx context.Context, systemPrompt string, userPrompt string) (string, error) {
	messages := []map[string]string{
		{"role": "system", "content": systemPrompt},
		{"role": "user", "content": userPrompt},
	}

	body := map[string]interface{}{
		"model":    p.model,
		"messages": messages,
		"stream":   true,
		// Disable Qwen3 thinking mode to avoid generating massive CoT blocks.
		// This has no effect on non-Qwen3 models (field is simply ignored).
		"think": false,
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

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("LLM returned %d: %s", resp.StatusCode, string(b))
	}

	// Parse SSE stream: each line is "data: <json>" or "data: [DONE]"
	var sb strings.Builder
	scanner := newSSEScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		if payload == "[DONE]" {
			break
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			continue // skip malformed chunks
		}
		if len(chunk.Choices) > 0 {
			sb.WriteString(chunk.Choices[0].Delta.Content)
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read SSE stream: %w", err)
	}

	return sb.String(), nil
}

// newSSEScanner returns a line scanner over an SSE response body.
func newSSEScanner(r io.Reader) *bufio.Scanner {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 64*1024)
	return scanner
}
