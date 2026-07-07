package detect

import (
	"testing"
	"time"

	"github.com/LeeSeokBln/faultbrief/internal/baseline"
	"github.com/LeeSeokBln/faultbrief/internal/model"
)

// buildAcc fills an accumulator: baseline gets `basePerBucket` occurrences of
// msg per bucket; analysis gets `analysisTotal` spread over the window.
func buildAcc(t *testing.T, basePerBucket, analysisTotal int, msg string, sev model.Severity) *baseline.Accumulator {
	t.Helper()
	now := time.Date(2026, 7, 7, 10, 0, 0, 0, time.UTC)
	w := baseline.Compute(now, time.Hour, 0, 4*time.Hour) // 12 analysis, 48 baseline buckets
	acc := baseline.NewAccumulator(w, 5*time.Minute)
	for b := 0; b < acc.NumBaselineBuckets(); b++ {
		ts := w.BaselineFrom.Add(time.Duration(b)*5*time.Minute + time.Second)
		for i := 0; i < basePerBucket; i++ {
			acc.Add(model.LogRecord{TS: ts, Source: "syslog", Message: msg, Severity: sev})
		}
	}
	for i := 0; i < analysisTotal; i++ {
		ts := w.AnalysisFrom.Add(time.Duration(i%12)*5*time.Minute + time.Second)
		acc.Add(model.LogRecord{TS: ts, Source: "syslog", Message: msg, Severity: sev})
	}
	return acc
}

func TestSpikeDetected(t *testing.T) {
	// baseline: 1/bucket steady; analysis: 120 total = 10/bucket -> huge z.
	acc := buildAcc(t, 1, 120, "conn to 10.0.0.1 refused", model.SevError)
	fs := Spikes(acc, DefaultParams())
	if len(fs) == 0 {
		t.Fatal("expected a spike finding")
	}
	f := fs[0]
	if f.Kind != model.KindSpike {
		t.Errorf("kind = %v", f.Kind)
	}
	if f.Severity < model.SevWarning {
		t.Errorf("spike severity should be at least warning, got %v", f.Severity)
	}
	if f.Count != 120 {
		t.Errorf("count = %d", f.Count)
	}
	if f.Score <= 3.0 {
		t.Errorf("score (z) = %f, want > threshold", f.Score)
	}
}

func TestNoSpikeOnSteadyRate(t *testing.T) {
	// baseline 10/bucket, analysis 120 total = 10/bucket -> no spike.
	acc := buildAcc(t, 10, 120, "steady noise", model.SevError)
	if fs := Spikes(acc, DefaultParams()); len(fs) != 0 {
		t.Fatalf("steady rate should not spike, got %+v", fs)
	}
}

func TestNoSpikeBelowMinCount(t *testing.T) {
	// Big relative jump but only 6 occurrences (< MinCount 10).
	acc := buildAcc(t, 0, 6, "rare thing", model.SevError)
	// absent from baseline -> novelty's turf, not spike's
	if fs := Spikes(acc, DefaultParams()); len(fs) != 0 {
		t.Fatalf("low-count template must not spike, got %+v", fs)
	}
}

func TestTemplateAbsentFromBaselineIsNotSpike(t *testing.T) {
	acc := buildAcc(t, 0, 120, "brand new flood", model.SevError)
	for _, f := range Spikes(acc, DefaultParams()) {
		if f.RuleID != "error-rate" { // aggregate may fire; template must not
			t.Fatalf("template spike on baseline-absent template: %+v", f)
		}
	}
}

func TestAggregateErrorRateSpike(t *testing.T) {
	acc := buildAcc(t, 1, 120, "conn to 10.0.0.1 refused", model.SevError)
	fs := Spikes(acc, DefaultParams())
	var agg *model.Finding
	for i := range fs {
		if fs[i].RuleID == "error-rate" {
			agg = &fs[i]
		}
	}
	if agg == nil {
		t.Fatal("expected aggregate error-rate spike finding")
	}
}

func TestNginx5xxRate(t *testing.T) {
	now := time.Date(2026, 7, 7, 10, 0, 0, 0, time.UTC)
	w := baseline.Compute(now, time.Hour, 0, 4*time.Hour)
	acc := baseline.NewAccumulator(w, 5*time.Minute)
	// analysis: 100 requests, 20 5xx -> 20% error rate
	for i := 0; i < 100; i++ {
		status := "200"
		sev := model.SevInfo
		if i < 20 {
			status, sev = "502", model.SevError
		}
		acc.Add(model.LogRecord{
			TS: w.AnalysisFrom.Add(time.Duration(i%12) * 5 * time.Minute), Source: "nginx-access",
			Message: "GET /x [5xx]", Severity: sev, Fields: map[string]string{"status": status},
		})
	}
	fs := Spikes(acc, DefaultParams())
	var hit *model.Finding
	for i := range fs {
		if fs[i].RuleID == "nginx-5xx-rate" {
			hit = &fs[i]
		}
	}
	if hit == nil {
		t.Fatal("expected nginx-5xx-rate finding")
	}
	if hit.Severity != model.SevError {
		t.Errorf("20%% 5xx should be error severity, got %v", hit.Severity)
	}
}

func TestNginx5xxRateBelowMinRequests(t *testing.T) {
	now := time.Date(2026, 7, 7, 10, 0, 0, 0, time.UTC)
	w := baseline.Compute(now, time.Hour, 0, 4*time.Hour)
	acc := baseline.NewAccumulator(w, 5*time.Minute)
	for i := 0; i < 10; i++ { // only 10 requests, all 5xx — too few to judge
		acc.Add(model.LogRecord{
			TS: w.AnalysisFrom, Source: "nginx-access", Message: "GET /x [5xx]",
			Severity: model.SevError, Fields: map[string]string{"status": "500"},
		})
	}
	for _, f := range Spikes(acc, DefaultParams()) {
		if f.RuleID == "nginx-5xx-rate" {
			t.Fatal("must not judge 5xx rate under Nginx5xxMinReq")
		}
	}
}
