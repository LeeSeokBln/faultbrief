package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LeeSeokBln/faultbrief/internal/config"
)

func TestBuildPromptIncludesLanguageAndFindings(t *testing.T) {
	sys, user := buildPrompt(Request{Lang: "ko", ReportJSON: []byte(`{"findings":[{"rule_id":"oom-kill"}]}`)})
	if !strings.Contains(sys, "SRE") {
		t.Errorf("system prompt = %q", sys)
	}
	if !strings.Contains(user, "oom-kill") || !strings.Contains(user, "Korean") {
		t.Errorf("user prompt missing content: %q", user)
	}
	_, userEn := buildPrompt(Request{Lang: "en", ReportJSON: []byte("{}")})
	if !strings.Contains(userEn, "English") {
		t.Errorf("en prompt = %q", userEn)
	}
}

func TestAnthropicProvider(t *testing.T) {
	var gotPath, gotKey, gotVersion string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotKey = r.Header.Get("x-api-key")
		gotVersion = r.Header.Get("anthropic-version")
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"content":[{"type":"text","text":"brief text"}]}`)
	}))
	defer srv.Close()

	p, err := New(config.LLM{Provider: "anthropic", Model: "claude-sonnet-5", BaseURL: srv.URL, MaxTokens: 512},
		func(k string) string { return map[string]string{"ANTHROPIC_API_KEY": "sk-test"}[k] })
	if err != nil {
		t.Fatal(err)
	}
	out, err := p.Brief(context.Background(), Request{Lang: "en", ReportJSON: []byte("{}")})
	if err != nil {
		t.Fatal(err)
	}
	if out != "brief text" {
		t.Errorf("out = %q", out)
	}
	if gotPath != "/v1/messages" || gotKey != "sk-test" || gotVersion == "" {
		t.Errorf("request meta: path=%q key=%q version=%q", gotPath, gotKey, gotVersion)
	}
	if gotBody["model"] != "claude-sonnet-5" || gotBody["max_tokens"] != float64(512) {
		t.Errorf("body = %v", gotBody)
	}
}

func TestOpenAIProvider(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer ok-key" {
			t.Errorf("auth = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"choices":[{"message":{"content":"openai brief"}}]}`)
	}))
	defer srv.Close()

	p, err := New(config.LLM{Provider: "openai", Model: "gpt-x", BaseURL: srv.URL, MaxTokens: 256},
		func(k string) string { return map[string]string{"OPENAI_API_KEY": "ok-key"}[k] })
	if err != nil {
		t.Fatal(err)
	}
	out, err := p.Brief(context.Background(), Request{Lang: "en", ReportJSON: []byte("{}")})
	if err != nil || out != "openai brief" {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

func TestOpenAIWithoutKeyAllowedForCustomBase(t *testing.T) {
	// Local endpoints (Ollama) need no key — only require a key for the
	// default api.openai.com base.
	_, err := New(config.LLM{Provider: "openai", Model: "llama3", BaseURL: "http://localhost:11434"},
		func(string) string { return "" })
	if err != nil {
		t.Fatalf("keyless custom base should work: %v", err)
	}
}

func TestAnthropicRequiresKey(t *testing.T) {
	_, err := New(config.LLM{Provider: "anthropic", Model: "m"}, func(string) string { return "" })
	if err == nil {
		t.Fatal("anthropic without ANTHROPIC_API_KEY must error")
	}
}

func TestOpenAIDefaultBaseRequiresKey(t *testing.T) {
	_, err := New(config.LLM{Provider: "openai", Model: "m"}, func(string) string { return "" })
	if err == nil {
		t.Fatal("openai default base without key must error")
	}
}

func TestUnknownProvider(t *testing.T) {
	_, err := New(config.LLM{Provider: "bard"}, func(string) string { return "x" })
	if err == nil {
		t.Fatal("unknown provider must error")
	}
}

func TestHTTPErrorSurfaced(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"message":"overloaded"}}`, http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	p, _ := New(config.LLM{Provider: "anthropic", Model: "m", BaseURL: srv.URL},
		func(string) string { return "k" })
	if _, err := p.Brief(context.Background(), Request{Lang: "en", ReportJSON: []byte("{}")}); err == nil {
		t.Fatal("503 must surface as error")
	}
}
