package baseline

import (
	"testing"
	"time"

	"github.com/LeeSeokBln/faultbrief/internal/model"
)

func TestComputeWindows(t *testing.T) {
	now := time.Date(2026, 7, 7, 10, 0, 0, 0, time.UTC)
	w := Compute(now, time.Hour, 0, 24*time.Hour)
	if !w.AnalysisTo.Equal(now) || !w.AnalysisFrom.Equal(now.Add(-time.Hour)) {
		t.Errorf("analysis window wrong: %+v", w)
	}
	if !w.BaselineTo.Equal(w.AnalysisFrom) || !w.BaselineFrom.Equal(w.AnalysisFrom.Add(-24*time.Hour)) {
		t.Errorf("baseline window wrong: %+v", w)
	}
	w2 := Compute(now, time.Hour, 10*time.Minute, 24*time.Hour)
	if !w2.AnalysisTo.Equal(now.Add(-10 * time.Minute)) {
		t.Errorf("until offset ignored: %+v", w2)
	}
}

func mkRec(ts time.Time, source, msg string, sev model.Severity) model.LogRecord {
	return model.LogRecord{TS: ts, Source: source, Message: msg, Severity: sev, Unit: "u"}
}

func TestAccumulatorRoutesWindows(t *testing.T) {
	now := time.Date(2026, 7, 7, 10, 0, 0, 0, time.UTC)
	w := Compute(now, time.Hour, 0, 24*time.Hour)
	acc := NewAccumulator(w, 5*time.Minute)

	inA := mkRec(now.Add(-30*time.Minute), "syslog", "conn to 10.0.0.1 failed", model.SevError)
	inB := mkRec(now.Add(-2*time.Hour), "syslog", "conn to 10.0.0.2 failed", model.SevError)
	out := mkRec(now.Add(-48*time.Hour), "syslog", "conn to 10.0.0.3 failed", model.SevError)
	acc.Add(inA)
	acc.Add(inB)
	acc.Add(out)

	if len(acc.Analysis) != 1 {
		t.Fatalf("analysis templates = %d, want 1", len(acc.Analysis))
	}
	var ts *TemplateStats
	for _, v := range acc.Analysis {
		ts = v
	}
	if ts.Count != 1 || ts.Masked != "conn to <IP> failed" {
		t.Errorf("stats = %+v", ts)
	}
	// Same masked template lands in the baseline map with count 1.
	if acc.BaselineCounts[ts.ID] != 1 {
		t.Errorf("baseline count = %d, want 1", acc.BaselineCounts[ts.ID])
	}
}

func TestAccumulatorBucketsAndAggregates(t *testing.T) {
	now := time.Date(2026, 7, 7, 10, 0, 0, 0, time.UTC)
	w := Compute(now, time.Hour, 0, time.Hour) // 1h baseline for easy math
	acc := NewAccumulator(w, 5*time.Minute)

	// 12 buckets per window. Put 2 errors in analysis bucket 0, 1 in bucket 11.
	acc.Add(mkRec(w.AnalysisFrom.Add(1*time.Minute), "syslog", "boom", model.SevError))
	acc.Add(mkRec(w.AnalysisFrom.Add(2*time.Minute), "syslog", "boom", model.SevError))
	acc.Add(mkRec(w.AnalysisFrom.Add(59*time.Minute), "syslog", "boom", model.SevCritical))
	// Non-error must not count into error aggregate.
	acc.Add(mkRec(w.AnalysisFrom.Add(3*time.Minute), "syslog", "fine", model.SevInfo))

	if got := acc.ErrA[0]; got != 2 {
		t.Errorf("ErrA[0] = %d, want 2", got)
	}
	if got := acc.ErrA[11]; got != 1 {
		t.Errorf("ErrA[11] = %d, want 1", got)
	}
	var boom *TemplateStats
	for _, v := range acc.Analysis {
		if v.Masked == "boom" {
			boom = v
		}
	}
	if boom == nil || boom.Count != 3 || boom.Buckets[0] != 2 || boom.Buckets[11] != 1 {
		t.Fatalf("boom stats = %+v", boom)
	}
	if boom.Severity != model.SevCritical {
		t.Errorf("template severity should be max seen, got %v", boom.Severity)
	}
	if !boom.FirstTS.Equal(w.AnalysisFrom.Add(1 * time.Minute)) {
		t.Errorf("first ts = %v", boom.FirstTS)
	}
}

func TestAccumulatorSamplesCappedAt3(t *testing.T) {
	now := time.Date(2026, 7, 7, 10, 0, 0, 0, time.UTC)
	w := Compute(now, time.Hour, 0, time.Hour)
	acc := NewAccumulator(w, 5*time.Minute)
	for i := 0; i < 10; i++ {
		acc.Add(mkRec(w.AnalysisFrom.Add(time.Minute), "syslog", "same message", model.SevInfo))
	}
	for _, v := range acc.Analysis {
		if len(v.Samples) != 3 {
			t.Errorf("samples = %d, want 3", len(v.Samples))
		}
	}
}

func TestAccumulatorNginxAggregates(t *testing.T) {
	now := time.Date(2026, 7, 7, 10, 0, 0, 0, time.UTC)
	w := Compute(now, time.Hour, 0, time.Hour)
	acc := NewAccumulator(w, 5*time.Minute)
	ok := mkRec(w.AnalysisFrom.Add(time.Minute), "nginx-access", "GET /a [2xx]", model.SevInfo)
	ok.Fields = map[string]string{"status": "200"}
	bad := mkRec(w.AnalysisFrom.Add(time.Minute), "nginx-access", "GET /a [5xx]", model.SevError)
	bad.Fields = map[string]string{"status": "502"}
	acc.Add(ok)
	acc.Add(bad)
	if acc.ReqA[0] != 2 || acc.Err5xxA[0] != 1 {
		t.Errorf("ReqA[0]=%d Err5xxA[0]=%d, want 2,1", acc.ReqA[0], acc.Err5xxA[0])
	}
}
