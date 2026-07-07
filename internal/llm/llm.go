// Package llm turns a finished rule-engine report into a short incident
// briefing via Anthropic or any OpenAI-compatible endpoint. It is strictly
// optional: callers must degrade gracefully when it errors.
package llm

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/LeeSeokBln/faultbrief/internal/config"
)

// Request carries the rendered report JSON and target language.
type Request struct {
	Lang       string
	ReportJSON []byte
}

// Provider generates a briefing.
type Provider interface {
	Name() string
	Brief(ctx context.Context, req Request) (string, error)
}

// New builds a provider from config. keyLookup abstracts os.Getenv for tests.
func New(cfg config.LLM, keyLookup func(string) string) (Provider, error) {
	if err := validateBaseURL(cfg.BaseURL); err != nil {
		return nil, err
	}
	switch cfg.Provider {
	case "anthropic":
		key := keyLookup("ANTHROPIC_API_KEY")
		if key == "" {
			return nil, fmt.Errorf("ANTHROPIC_API_KEY is not set (required for --llm with provider=anthropic)")
		}
		return &anthropicProvider{cfg: cfg, key: key, client: httpClient()}, nil
	case "openai":
		key := keyLookup("OPENAI_API_KEY")
		if key == "" && (cfg.BaseURL == "" || cfg.BaseURL == openaiDefaultBase) {
			return nil, fmt.Errorf("OPENAI_API_KEY is not set (required for the default OpenAI endpoint)")
		}
		return &openaiProvider{cfg: cfg, key: key, client: httpClient()}, nil
	default:
		return nil, fmt.Errorf("unknown llm provider %q (want anthropic or openai)", cfg.Provider)
	}
}

func httpClient() *http.Client {
	return &http.Client{Timeout: 90 * time.Second}
}

// validateBaseURL restricts custom endpoints to http/https with a host.
// API keys and log excerpts travel to this URL: exotic schemes are always a
// bug, and http is intended for loopback endpoints such as Ollama — use
// https for anything remote.
func validateBaseURL(raw string) error {
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid llm base_url %q: %w", raw, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("llm base_url must use http or https, got %q", u.Scheme)
	}
	if u.Host == "" {
		return fmt.Errorf("llm base_url %q has no host", raw)
	}
	return nil
}

const systemPrompt = `You are an experienced SRE incident analyst. You receive a JSON report produced by a log-analysis rule engine (signature matches, frequency spikes, novel patterns). Write a short incident briefing with exactly these sections:
1. Summary — what appears to be happening, one or two sentences.
2. Likely causes — ranked hypotheses grounded ONLY in the provided findings.
3. Impact — what is likely affected.
4. Next checks — concrete commands or places to look.
Do not invent findings that are not in the report. If findings are sparse, say so.`

func buildPrompt(req Request) (system, user string) {
	langName := "English"
	if req.Lang == "ko" {
		langName = "Korean"
	}
	user = fmt.Sprintf("Report JSON:\n%s\n\nWrite the briefing in %s.", req.ReportJSON, langName)
	return systemPrompt, user
}
