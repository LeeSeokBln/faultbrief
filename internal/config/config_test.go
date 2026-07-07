package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaults(t *testing.T) {
	c := Default()
	if c.Lang != "en" || c.Format != "text" || c.MinSeverity != "info" {
		t.Errorf("defaults = %+v", c)
	}
	if c.BaselineHours != 24 {
		t.Errorf("baseline hours = %d", c.BaselineHours)
	}
	if c.LLM.Provider != "anthropic" || c.LLM.Model == "" || c.LLM.MaxTokens <= 0 {
		t.Errorf("llm defaults = %+v", c.LLM)
	}
}

func TestLoadFileOverridesDefaults(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	os.WriteFile(p, []byte("lang: ko\nllm:\n  provider: openai\n  model: llama3\n"), 0o644)
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if c.Lang != "ko" || c.LLM.Provider != "openai" || c.LLM.Model != "llama3" {
		t.Errorf("loaded = %+v", c)
	}
	// untouched fields keep defaults
	if c.Format != "text" || c.LLM.MaxTokens <= 0 {
		t.Errorf("defaults lost = %+v", c)
	}
}

func TestLoadMissingFileIsDefaults(t *testing.T) {
	c, err := Load(filepath.Join(t.TempDir(), "nope.yaml"))
	if err != nil {
		t.Fatalf("missing config must not error: %v", err)
	}
	if c.Lang != "en" {
		t.Errorf("got %+v", c)
	}
}

func TestEnvOverridesFile(t *testing.T) {
	t.Setenv("FAULTBRIEF_LANG", "ko")
	t.Setenv("FAULTBRIEF_LLM_PROVIDER", "openai")
	t.Setenv("FAULTBRIEF_LLM_MODEL", "qwen3")
	t.Setenv("FAULTBRIEF_LLM_BASE_URL", "http://localhost:11434")
	c, err := Load(filepath.Join(t.TempDir(), "nope.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if c.Lang != "ko" || c.LLM.Provider != "openai" || c.LLM.Model != "qwen3" || c.LLM.BaseURL != "http://localhost:11434" {
		t.Errorf("env not applied: %+v", c)
	}
}

func TestLoadRejectsBadYAML(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	os.WriteFile(p, []byte("lang: [broken"), 0o644)
	if _, err := Load(p); err == nil {
		t.Fatal("bad yaml should error")
	}
}
