package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/LeeSeokBln/faultbrief/internal/model"
)

func sampleReport() *Report {
	loc := time.UTC
	return &Report{
		GeneratedAt:  time.Date(2026, 7, 7, 10, 0, 0, 0, loc),
		WindowFrom:   time.Date(2026, 7, 7, 9, 0, 0, 0, loc),
		WindowTo:     time.Date(2026, 7, 7, 10, 0, 0, 0, loc),
		BaselineFrom: time.Date(2026, 7, 6, 9, 0, 0, 0, loc),
		BaselineTo:   time.Date(2026, 7, 7, 9, 0, 0, 0, loc),
		Loc:          loc,
		Records:      1234,
		Sources: []SourceStat{
			{Name: "syslog", Lines: 1000, Parsed: 990, Failed: 10},
			{Name: "journald", Skipped: true, SkipReason: "journalctl not found"},
		},
		Findings: []model.Finding{{
			Kind: model.KindSignature, RuleID: "oom-kill", Severity: model.SevCritical,
			Title: "OOM killer terminated a process", Detail: "matched 2 time(s)",
			Hint: "free -h", Count: 2, Source: "syslog", Unit: "kernel",
			Samples: []string{"Out of memory: Killed process 1234 (myapp)"},
			FirstTS: time.Date(2026, 7, 7, 9, 12, 3, 0, loc),
			LastTS:  time.Date(2026, 7, 7, 9, 14, 11, 0, loc),
		}},
	}
}

func TestRenderTextEnglish(t *testing.T) {
	var buf bytes.Buffer
	if err := RenderText(&buf, sampleReport(), "en", false); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"FAULTBRIEF", "2026-07-07 09:00", "CRITICAL", "OOM killer terminated a process",
		"oom-kill", "free -h", "journalctl not found", "1234",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("text output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "\x1b[") {
		t.Error("color=false must not emit ANSI codes")
	}
}

func TestRenderTextKorean(t *testing.T) {
	var buf bytes.Buffer
	if err := RenderText(&buf, sampleReport(), "ko", false); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"장애 브리핑", "심각", "발견"} {
		if !strings.Contains(out, want) {
			t.Errorf("korean output missing %q:\n%s", want, out)
		}
	}
}

func TestRenderTextNoFindings(t *testing.T) {
	r := sampleReport()
	r.Findings = nil
	var buf bytes.Buffer
	if err := RenderText(&buf, r, "en", false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "No findings") {
		t.Errorf("missing healthy message:\n%s", buf.String())
	}
}

func TestRenderTextColor(t *testing.T) {
	var buf bytes.Buffer
	if err := RenderText(&buf, sampleReport(), "en", true); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "\x1b[") {
		t.Error("color=true should emit ANSI codes")
	}
}

func TestRenderJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := RenderJSON(&buf, sampleReport()); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if got["records"] != float64(1234) {
		t.Errorf("records = %v", got["records"])
	}
	fs, ok := got["findings"].([]any)
	if !ok || len(fs) == 0 {
		t.Fatal("findings is not an array or is empty")
	}
	f0, ok := fs[0].(map[string]any)
	if !ok {
		t.Fatalf("first finding is not a map, got: %T", fs[0])
	}
	if f0["severity"] != "critical" || f0["rule_id"] != "oom-kill" {
		t.Errorf("finding json = %v", f0)
	}
}

func TestRenderMarkdown(t *testing.T) {
	var buf bytes.Buffer
	if err := RenderMarkdown(&buf, sampleReport(), "en"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "# faultbrief") || !strings.Contains(out, "## Findings") {
		t.Errorf("markdown structure missing:\n%s", out)
	}
	if !strings.Contains(out, "**CRITICAL**") {
		t.Errorf("severity emphasis missing:\n%s", out)
	}
}

func TestParseFailureWarning(t *testing.T) {
	r := sampleReport()
	// 30% failure rate must trigger the spec-mandated >20% warning.
	r.Sources = []SourceStat{{Name: "syslog", Lines: 100, Parsed: 70, Failed: 30}}
	var buf bytes.Buffer
	if err := RenderText(&buf, r, "en", false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "high parse-failure rate") {
		t.Errorf("missing >20%% parse-failure warning:\n%s", buf.String())
	}
	// 1% failure rate must not warn.
	r.Sources = []SourceStat{{Name: "syslog", Lines: 100, Parsed: 99, Failed: 1}}
	buf.Reset()
	if err := RenderText(&buf, r, "en", false); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "high parse-failure rate") {
		t.Errorf("warning fired below threshold:\n%s", buf.String())
	}
}

func TestLLMBriefSectionRendered(t *testing.T) {
	r := sampleReport()
	r.LLMBrief = "Everything is on fire because of X."
	var buf bytes.Buffer
	if err := RenderText(&buf, r, "en", false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "on fire because of X") {
		t.Error("LLM brief not rendered")
	}
}
