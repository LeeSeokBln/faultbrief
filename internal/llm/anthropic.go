package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/LeeSeokBln/faultbrief/internal/config"
)

const anthropicDefaultBase = "https://api.anthropic.com"

type anthropicProvider struct {
	cfg    config.LLM
	key    string
	client *http.Client
}

func (p *anthropicProvider) Name() string { return "anthropic" }

func (p *anthropicProvider) Brief(ctx context.Context, req Request) (string, error) {
	system, user := buildPrompt(req)
	base := p.cfg.BaseURL
	if base == "" {
		base = anthropicDefaultBase
	}
	maxTokens := p.cfg.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 1024
	}
	body, err := json.Marshal(map[string]any{
		"model":      p.cfg.Model,
		"max_tokens": maxTokens,
		"system":     system,
		"messages": []map[string]string{
			{"role": "user", "content": user},
		},
	})
	if err != nil {
		return "", err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.key)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("anthropic request: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("anthropic API %d: %s", resp.StatusCode, truncateBytes(data, 300))
	}
	var out struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return "", fmt.Errorf("anthropic response: %w", err)
	}
	for _, c := range out.Content {
		if c.Type == "text" {
			return c.Text, nil
		}
	}
	return "", fmt.Errorf("anthropic response had no text content")
}

func truncateBytes(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "…"
}
