package detect

import (
	"strings"
	"testing"
	"time"

	"github.com/LeeSeokBln/faultbrief/internal/model"
	"github.com/LeeSeokBln/faultbrief/internal/rules"
	"github.com/LeeSeokBln/faultbrief/internal/template"
)

func loadRules(t *testing.T, yml string) []rules.Rule {
	t.Helper()
	rs, err := rules.Load(strings.NewReader(yml))
	if err != nil {
		t.Fatal(err)
	}
	return rs
}

func TestSignatureMatcherAggregatesHits(t *testing.T) {
	rs := loadRules(t, `
- id: oom
  title: OOM kill
  severity: critical
  contains: "Out of memory"
  hint: "check memory"
`)
	m := NewSignatureMatcher(rs)
	t0 := time.Date(2026, 7, 7, 8, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		m.Feed(model.LogRecord{
			TS: t0.Add(time.Duration(i) * time.Minute), Source: "journald",
			Unit: "kernel", Message: "Out of memory: Killed process 42",
		})
	}
	m.Feed(model.LogRecord{TS: t0, Source: "journald", Message: "healthy"})

	fs := m.Findings()
	if len(fs) != 1 {
		t.Fatalf("findings = %d, want 1", len(fs))
	}
	f := fs[0]
	if f.Kind != model.KindSignature || f.RuleID != "oom" || f.Count != 5 {
		t.Errorf("finding = %+v", f)
	}
	if f.Severity != model.SevCritical || f.Hint != "check memory" {
		t.Errorf("finding meta = %+v", f)
	}
	if len(f.Samples) != 3 {
		t.Errorf("samples = %d, want capped at 3", len(f.Samples))
	}
	if !f.LastTS.Equal(t0.Add(4 * time.Minute)) {
		t.Errorf("last ts = %v", f.LastTS)
	}
}

func TestSignatureMatcherNoHitsNoFindings(t *testing.T) {
	m := NewSignatureMatcher(nil)
	m.Feed(model.LogRecord{Message: "anything"})
	if fs := m.Findings(); len(fs) != 0 {
		t.Fatalf("findings = %d, want 0", len(fs))
	}
}

func TestSignatureMatcherTracksMatchedTemplates(t *testing.T) {
	rs := loadRules(t, `
- id: oom
  title: OOM kill
  severity: critical
  contains: "Out of memory"
`)
	m := NewSignatureMatcher(rs)
	hit := model.LogRecord{Source: "syslog", Message: "Out of memory: Killed process 42"}
	m.Feed(hit)
	m.Feed(model.LogRecord{Source: "syslog", Message: "healthy noise"})

	want := template.Fingerprint("syslog", template.Mask(hit.Message))
	got := m.MatchedTemplates()
	if !got[want] {
		t.Errorf("matched template fingerprint missing: %v", got)
	}
	if len(got) != 1 {
		t.Errorf("expected exactly 1 matched template, got %d", len(got))
	}
}
