package detect

import (
	"sort"

	"github.com/LeeSeokBln/faultbrief/internal/model"
)

// Rank sorts findings in place: severity desc, then score desc, then count
// desc, then rule id for determinism.
func Rank(fs []model.Finding) {
	sort.Slice(fs, func(i, j int) bool {
		if fs[i].Severity != fs[j].Severity {
			return fs[i].Severity > fs[j].Severity
		}
		if fs[i].Score != fs[j].Score {
			return fs[i].Score > fs[j].Score
		}
		if fs[i].Count != fs[j].Count {
			return fs[i].Count > fs[j].Count
		}
		return fs[i].RuleID < fs[j].RuleID
	})
}

// FilterMinSeverity drops findings below min.
func FilterMinSeverity(fs []model.Finding, min model.Severity) []model.Finding {
	out := fs[:0:0]
	for _, f := range fs {
		if f.Severity >= min {
			out = append(out, f)
		}
	}
	return out
}
