package detect

import (
	"fmt"

	"github.com/LeeSeokBln/faultbrief/internal/model"
	"github.com/LeeSeokBln/faultbrief/internal/rules"
)

type sigHit struct {
	rule    *rules.Rule
	count   int
	unit    string
	source  string
	samples []string
	first   model.LogRecord
	last    model.LogRecord
}

// SignatureMatcher accumulates rule hits over analysis-window records.
type SignatureMatcher struct {
	rules []rules.Rule
	hits  map[string]*sigHit
	order []string // first-hit order for deterministic output
}

func NewSignatureMatcher(rs []rules.Rule) *SignatureMatcher {
	return &SignatureMatcher{rules: rs, hits: map[string]*sigHit{}}
}

// Feed matches one analysis-window record against every rule.
func (m *SignatureMatcher) Feed(rec model.LogRecord) {
	for i := range m.rules {
		r := &m.rules[i]
		if !r.Matches(rec) {
			continue
		}
		h, ok := m.hits[r.ID]
		if !ok {
			h = &sigHit{rule: r, unit: rec.Unit, source: rec.Source, first: rec, last: rec}
			m.hits[r.ID] = h
			m.order = append(m.order, r.ID)
		}
		h.count++
		if rec.TS.Before(h.first.TS) {
			h.first = rec
		}
		if !rec.TS.Before(h.last.TS) {
			h.last = rec
		}
		if len(h.samples) < 3 {
			h.samples = append(h.samples, rec.Message)
		}
	}
}

// Findings converts accumulated hits into findings.
func (m *SignatureMatcher) Findings() []model.Finding {
	fs := make([]model.Finding, 0, len(m.order))
	for _, id := range m.order {
		h := m.hits[id]
		fs = append(fs, model.Finding{
			Kind:     model.KindSignature,
			RuleID:   h.rule.ID,
			Severity: h.rule.Sev(),
			Title:    h.rule.Title,
			Detail:   fmt.Sprintf("matched %d time(s)", h.count),
			Hint:     h.rule.Hint,
			Count:    h.count,
			Score:    float64(h.count),
			Source:   h.source,
			Unit:     h.unit,
			Samples:  h.samples,
			FirstTS:  h.first.TS,
			LastTS:   h.last.TS,
		})
	}
	return fs
}
