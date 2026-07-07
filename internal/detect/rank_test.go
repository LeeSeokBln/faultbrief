package detect

import (
	"testing"

	"github.com/LeeSeokBln/faultbrief/internal/model"
)

func TestRankOrdersBySeverityThenScore(t *testing.T) {
	fs := []model.Finding{
		{RuleID: "a", Severity: model.SevWarning, Score: 99},
		{RuleID: "b", Severity: model.SevCritical, Score: 1},
		{RuleID: "c", Severity: model.SevError, Score: 5},
		{RuleID: "d", Severity: model.SevError, Score: 50},
	}
	Rank(fs)
	got := []string{fs[0].RuleID, fs[1].RuleID, fs[2].RuleID, fs[3].RuleID}
	want := []string{"b", "d", "c", "a"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}

func TestFilterMinSeverity(t *testing.T) {
	fs := []model.Finding{
		{RuleID: "info", Severity: model.SevInfo},
		{RuleID: "warn", Severity: model.SevWarning},
		{RuleID: "crit", Severity: model.SevCritical},
	}
	out := FilterMinSeverity(fs, model.SevWarning)
	if len(out) != 2 {
		t.Fatalf("filtered = %d, want 2", len(out))
	}
	for _, f := range out {
		if f.Severity < model.SevWarning {
			t.Errorf("leaked %+v", f)
		}
	}
}
