// Package rules loads signature rules: curated builtins embedded in the
// binary plus optional user-supplied YAML files.
package rules

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/LeeSeokBln/faultbrief/internal/model"
)

//go:embed builtin.yaml
var builtinYAML []byte

// Rule is one signature: a known-bad log pattern with metadata.
type Rule struct {
	ID       string `yaml:"id"`
	Title    string `yaml:"title"`
	Severity string `yaml:"severity"`
	Source   string `yaml:"source"`   // "", "any", or a source name
	Contains string `yaml:"contains"` // exactly one of Contains/Regex
	Regex    string `yaml:"regex"`
	Hint     string `yaml:"hint"`
	Example  string `yaml:"example"` // sample line, verified by tests

	re  *regexp.Regexp
	sev model.Severity
}

// Sev returns the parsed severity.
func (r *Rule) Sev() model.Severity { return r.sev }

// Matches reports whether the record triggers this rule.
func (r *Rule) Matches(rec model.LogRecord) bool {
	if r.Source != "" && r.Source != "any" && r.Source != rec.Source {
		return false
	}
	if r.Contains != "" {
		return strings.Contains(rec.Message, r.Contains)
	}
	return r.re.MatchString(rec.Message)
}

// Load parses and validates rules from YAML.
func Load(r io.Reader) ([]Rule, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	var rs []Rule
	if err := yaml.Unmarshal(data, &rs); err != nil {
		return nil, fmt.Errorf("parse rules: %w", err)
	}
	for i := range rs {
		if err := compile(&rs[i]); err != nil {
			return nil, err
		}
	}
	return rs, nil
}

// Builtin returns the embedded curated rule set.
func Builtin() ([]Rule, error) {
	return Load(bytes.NewReader(builtinYAML))
}

func compile(r *Rule) error {
	if r.ID == "" {
		return fmt.Errorf("rule missing id")
	}
	if r.Title == "" {
		return fmt.Errorf("rule %s: missing title", r.ID)
	}
	sev, ok := model.ParseSeverity(r.Severity)
	if !ok {
		return fmt.Errorf("rule %s: invalid severity %q", r.ID, r.Severity)
	}
	r.sev = sev
	if (r.Contains == "") == (r.Regex == "") {
		return fmt.Errorf("rule %s: exactly one of contains/regex required", r.ID)
	}
	if r.Regex != "" {
		re, err := regexp.Compile(r.Regex)
		if err != nil {
			return fmt.Errorf("rule %s: bad regex: %w", r.ID, err)
		}
		r.re = re
	}
	return nil
}
