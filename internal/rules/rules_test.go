package rules

import (
	"strings"
	"testing"

	"github.com/LeeSeokBln/faultbrief/internal/model"
)

func TestLoadValidatesRules(t *testing.T) {
	bad := []string{
		`- title: "no id"` + "\n  severity: error\n  contains: x",
		`- id: no-sev` + "\n  title: t\n  contains: x",
		`- id: no-match` + "\n  title: t\n  severity: error",
		`- id: bad-sev` + "\n  title: t\n  severity: silly\n  contains: x",
		`- id: bad-re` + "\n  title: t\n  severity: error\n  regex: \"[\"",
	}
	for _, y := range bad {
		if _, err := Load(strings.NewReader(y)); err == nil {
			t.Errorf("Load should reject:\n%s", y)
		}
	}
}

func TestRuleMatch(t *testing.T) {
	yml := `
- id: oom
  title: OOM
  severity: critical
  contains: "Out of memory"
- id: nginx-only
  title: upstream
  severity: error
  source: nginx-error
  regex: "upstream timed out"
`
	rs, err := Load(strings.NewReader(yml))
	if err != nil {
		t.Fatal(err)
	}
	rec := model.LogRecord{Source: "journald", Message: "Out of memory: Killed process 42 (app)"}
	if !rs[0].Matches(rec) {
		t.Error("contains rule should match")
	}
	up := model.LogRecord{Source: "syslog", Message: "upstream timed out while reading"}
	if rs[1].Matches(up) {
		t.Error("source-scoped rule must not match other sources")
	}
	up.Source = "nginx-error"
	if !rs[1].Matches(up) {
		t.Error("source-scoped rule should match its source")
	}
}

// Every builtin rule must load, and every builtin rule must match its own
// example line — this keeps the curated set honest.
func TestBuiltinRulesMatchExamples(t *testing.T) {
	rs, err := Builtin()
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) < 20 {
		t.Fatalf("expected >= 20 builtin rules, got %d", len(rs))
	}
	seen := map[string]bool{}
	for _, r := range rs {
		if seen[r.ID] {
			t.Errorf("duplicate rule id %s", r.ID)
		}
		seen[r.ID] = true
		if r.Example == "" {
			t.Errorf("rule %s has no example line", r.ID)
			continue
		}
		rec := model.LogRecord{Source: firstSource(r), Message: r.Example}
		if !r.Matches(rec) {
			t.Errorf("rule %s does not match its own example %q", r.ID, r.Example)
		}
	}
}

func firstSource(r Rule) string {
	if r.Source == "" || r.Source == "any" {
		return "syslog"
	}
	return r.Source
}
