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

const openaiDefaultBase = "https://api.openai.com"

type openaiProvider struct {
	cfg    config.LLM
	key    string
	client *http.Client
}

func (p *openaiProvider) Name() string { return "openai" }

func (p *openaiProvider) Brief(ctx context.Context, req Request) (string, error) {
	system, user := buildPrompt(req)
	base := p.cfg.BaseURL
	if base == "" {
		base = openaiDefaultBase
	}
	maxTokens := p.cfg.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 1024
	}
	body, err := json.Marshal(map[string]any{
		"model":      p.cfg.Model,
		"max_tokens": maxTokens,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
	})
	if err != nil {
		return "", err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if p.key != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.key)
	}
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("openai request: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("openai API %d: %s", resp.StatusCode, truncateBytes(data, 300))
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return "", fmt.Errorf("openai response: %w", err)
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("openai response had no choices")
	}
	return out.Choices[0].Message.Content, nil
}
