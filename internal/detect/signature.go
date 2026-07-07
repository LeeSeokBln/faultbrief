package detect

import (
	"fmt"

	"github.com/LeeSeokBln/faultbrief/internal/model"
	"github.com/LeeSeokBln/faultbrief/internal/rules"
	"github.com/LeeSeokBln/faultbrief/internal/template"
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
	rules      []rules.Rule
	hits       map[string]*sigHit
	order      []string        // first-hit order for deterministic output
	matchedTpl map[string]bool // template fingerprints of matched records
}

func NewSignatureMatcher(rs []rules.Rule) *SignatureMatcher {
	return &SignatureMatcher{rules: rs, hits: map[string]*sigHit{}, matchedTpl: map[string]bool{}}
}

// MatchedTemplates returns the fingerprints of every template that produced
// a signature hit, so the novelty detector can skip already-explained lines.
func (m *SignatureMatcher) MatchedTemplates() map[string]bool { return m.matchedTpl }

// Feed matches one analysis-window record against every rule.
func (m *SignatureMatcher) Feed(rec model.LogRecord) {
	matched := false
	for i := range m.rules {
		r := &m.rules[i]
		if !r.Matches(rec) {
			continue
		}
		matched = true
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
	if matched {
		m.matchedTpl[template.Fingerprint(rec.Source, template.Mask(rec.Message))] = true
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
