package detect

import (
	"testing"
	"time"

	"github.com/LeeSeokBln/faultbrief/internal/baseline"
	"github.com/LeeSeokBln/faultbrief/internal/model"
	"github.com/LeeSeokBln/faultbrief/internal/template"
)

func noveltyAcc(t *testing.T) (*baseline.Accumulator, baseline.Windows) {
	t.Helper()
	now := time.Date(2026, 7, 7, 10, 0, 0, 0, time.UTC)
	w := baseline.Compute(now, time.Hour, 0, 4*time.Hour)
	return baseline.NewAccumulator(w, 5*time.Minute), w
}

func TestNoveltyDetected(t *testing.T) {
	acc, w := noveltyAcc(t)
	// baseline has an old template
	acc.Add(model.LogRecord{TS: w.BaselineFrom.Add(time.Minute), Source: "syslog", Message: "old friend", Severity: model.SevInfo})
	// analysis has a brand-new error template, 4 times (>= NoveltyMin 3)
	for i := 0; i < 4; i++ {
		acc.Add(model.LogRecord{TS: w.AnalysisFrom.Add(time.Minute), Source: "syslog", Message: "certificate verify failed for backend", Severity: model.SevError})
	}
	fs := Novelties(acc, nil, DefaultParams())
	if len(fs) != 1 {
		t.Fatalf("findings = %d, want 1", len(fs))
	}
	f := fs[0]
	if f.Kind != model.KindNovelty || f.Count != 4 || f.Severity != model.SevError {
		t.Errorf("finding = %+v", f)
	}
}

func TestNoveltyIgnoresKnownTemplates(t *testing.T) {
	acc, w := noveltyAcc(t)
	for i := 0; i < 5; i++ {
		acc.Add(model.LogRecord{TS: w.BaselineFrom.Add(time.Minute), Source: "syslog", Message: "same thing", Severity: model.SevError})
		acc.Add(model.LogRecord{TS: w.AnalysisFrom.Add(time.Minute), Source: "syslog", Message: "same thing", Severity: model.SevError})
	}
	if fs := Novelties(acc, nil, DefaultParams()); len(fs) != 0 {
		t.Fatalf("known template flagged as novel: %+v", fs)
	}
}

func TestNoveltyBelowMinCount(t *testing.T) {
	acc, w := noveltyAcc(t)
	acc.Add(model.LogRecord{TS: w.AnalysisFrom.Add(time.Minute), Source: "syslog", Message: "one-off oddity", Severity: model.SevError})
	if fs := Novelties(acc, nil, DefaultParams()); len(fs) != 0 {
		t.Fatalf("below-min template flagged: %+v", fs)
	}
}

func TestNoveltyRespectsCache(t *testing.T) {
	acc, w := noveltyAcc(t)
	msg := "seen last week though"
	for i := 0; i < 4; i++ {
		acc.Add(model.LogRecord{TS: w.AnalysisFrom.Add(time.Minute), Source: "syslog", Message: msg, Severity: model.SevError})
	}
	cache := baseline.NewCache("")
	id := template.Fingerprint("syslog", template.Mask(msg))
	cache.Remember(id, template.Mask(msg), w.BaselineFrom)
	if fs := Novelties(acc, cache, DefaultParams()); len(fs) != 0 {
		t.Fatalf("cached template flagged as novel: %+v", fs)
	}
}

func TestNoveltyCapAndOrdering(t *testing.T) {
	acc, w := noveltyAcc(t)
	// 12 distinct new templates; the critical one must rank first, cap at 10.
	for i := 0; i < 12; i++ {
		msg := "new pattern variant " + string(rune('A'+i)) + " appeared"
		sev := model.SevWarning
		if i == 7 {
			sev = model.SevCritical
		}
		for j := 0; j < 3+i; j++ {
			acc.Add(model.LogRecord{TS: w.AnalysisFrom.Add(time.Minute), Source: "syslog", Message: msg, Severity: sev})
		}
	}
	fs := Novelties(acc, nil, DefaultParams())
	if len(fs) != 10 {
		t.Fatalf("findings = %d, want capped at 10", len(fs))
	}
	if fs[0].Severity != model.SevCritical {
		t.Errorf("first finding should be the critical one, got %+v", fs[0])
	}
}
