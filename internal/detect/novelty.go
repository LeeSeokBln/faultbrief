package detect

import (
	"fmt"
	"sort"

	"github.com/LeeSeokBln/faultbrief/internal/baseline"
	"github.com/LeeSeokBln/faultbrief/internal/model"
)

// Novelties reports templates present in the analysis window but absent from
// both the baseline window and the optional long-term cache.
func Novelties(acc *baseline.Accumulator, cache *baseline.Cache, p Params) []model.Finding {
	var candidates []*baseline.TemplateStats
	for id, st := range acc.Analysis {
		if st.Count < p.NoveltyMin {
			continue
		}
		if _, inBaseline := acc.BaselineCounts[id]; inBaseline {
			continue
		}
		if cache != nil && cache.Has(id) {
			continue
		}
		candidates = append(candidates, st)
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Severity != candidates[j].Severity {
			return candidates[i].Severity > candidates[j].Severity
		}
		if candidates[i].Count != candidates[j].Count {
			return candidates[i].Count > candidates[j].Count
		}
		return candidates[i].ID < candidates[j].ID // deterministic tie-break
	})
	if len(candidates) > p.MaxNovelty {
		candidates = candidates[:p.MaxNovelty]
	}
	fs := make([]model.Finding, 0, len(candidates))
	for _, st := range candidates {
		fs = append(fs, model.Finding{
			Kind:     model.KindNovelty,
			RuleID:   st.ID,
			Severity: st.Severity,
			Title:    truncate(st.Masked, 100),
			Detail:   fmt.Sprintf("new pattern: %d occurrence(s), not seen in baseline", st.Count),
			Count:    st.Count,
			Score:    float64(st.Count),
			Source:   st.Source,
			Unit:     st.Unit,
			Samples:  st.Samples,
			FirstTS:  st.FirstTS,
			LastTS:   st.LastTS,
		})
	}
	return fs
}
