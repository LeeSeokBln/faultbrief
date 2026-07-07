// Package config resolves settings with precedence:
// CLI flags (applied by the caller) > env vars > config file > defaults.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// LLM configures the optional briefing provider.
type LLM struct {
	Provider  string `yaml:"provider"` // "anthropic" | "openai"
	Model     string `yaml:"model"`
	BaseURL   string `yaml:"base_url"`
	MaxTokens int    `yaml:"max_tokens"`
}

// Config is everything tunable outside CLI flags.
type Config struct {
	Lang             string   `yaml:"lang"`
	Format           string   `yaml:"format"`
	MinSeverity      string   `yaml:"min_severity"`
	BaselineHours    int      `yaml:"baseline_hours"`
	UseCache         bool     `yaml:"use_cache"`
	SyslogPaths      []string `yaml:"syslog_paths"`
	NginxAccessPaths []string `yaml:"nginx_access_paths"`
	NginxErrorPaths  []string `yaml:"nginx_error_paths"`
	RulesPaths       []string `yaml:"rules_paths"`
	LLM              LLM      `yaml:"llm"`
}

// Default returns baked-in defaults.
func Default() Config {
	return Config{
		Lang:          "en",
		Format:        "text",
		MinSeverity:   "info",
		BaselineHours: 24,
		LLM: LLM{
			Provider:  "anthropic",
			Model:     "claude-sonnet-5",
			MaxTokens: 1024,
		},
	}
}

// DefaultPath returns ~/.config/faultbrief/config.yaml.
func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "faultbrief", "config.yaml")
}

// DefaultCachePath returns ~/.local/state/faultbrief/patterns.json.
func DefaultCachePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".local", "state", "faultbrief", "patterns.json")
}

// Load merges the config file (if present) and env vars over defaults.
func Load(path string) (Config, error) {
	c := Default()
	if path != "" {
		data, err := os.ReadFile(path)
		switch {
		case err == nil:
			if err := yaml.Unmarshal(data, &c); err != nil {
				return c, fmt.Errorf("parse config %s: %w", path, err)
			}
		case os.IsNotExist(err):
			// fine: defaults
		default:
			return c, fmt.Errorf("read config %s: %w", path, err)
		}
	}
	applyEnv(&c)
	return c, nil
}

func applyEnv(c *Config) {
	set := func(dst *string, key string) {
		if v := os.Getenv(key); v != "" {
			*dst = v
		}
	}
	set(&c.Lang, "FAULTBRIEF_LANG")
	set(&c.Format, "FAULTBRIEF_FORMAT")
	set(&c.MinSeverity, "FAULTBRIEF_MIN_SEVERITY")
	set(&c.LLM.Provider, "FAULTBRIEF_LLM_PROVIDER")
	set(&c.LLM.Model, "FAULTBRIEF_LLM_MODEL")
	set(&c.LLM.BaseURL, "FAULTBRIEF_LLM_BASE_URL")
}
