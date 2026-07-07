package source

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/LeeSeokBln/faultbrief/internal/model"
)

func TestParseErrorLine(t *testing.T) {
	line := `2026/07/07 08:31:02 [error] 1234#0: *567 upstream timed out (110: Connection timed out) while reading response header from upstream, client: 1.2.3.4, server: example.com`
	rec, err := parseErrorLine(line)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Severity != model.SevError {
		t.Errorf("severity = %v", rec.Severity)
	}
	if rec.Unit != "nginx" || rec.Source != "nginx-error" {
		t.Errorf("unit=%q source=%q", rec.Unit, rec.Source)
	}
	wantMsg := "upstream timed out (110: Connection timed out) while reading response header from upstream, client: 1.2.3.4, server: example.com"
	if rec.Message != wantMsg {
		t.Errorf("message = %q", rec.Message)
	}
	want := time.Date(2026, 7, 7, 8, 31, 2, 0, time.Local)
	if !rec.TS.Equal(want) {
		t.Errorf("ts = %v, want %v", rec.TS, want)
	}
}

func TestParseErrorLineLevels(t *testing.T) {
	cases := map[string]model.Severity{
		"emerg": model.SevCritical, "alert": model.SevCritical, "crit": model.SevCritical,
		"error": model.SevError, "warn": model.SevWarning, "notice": model.SevNotice, "info": model.SevInfo,
		"debug": model.SevDebug,
	}
	for lvl, want := range cases {
		line := "2026/07/07 08:00:00 [" + lvl + "] 1#0: something happened"
		rec, err := parseErrorLine(line)
		if err != nil {
			t.Fatalf("%s: %v", lvl, err)
		}
		if rec.Severity != want {
			t.Errorf("[%s] -> %v, want %v", lvl, rec.Severity, want)
		}
	}
}

func TestParseErrorLineWithoutConnID(t *testing.T) {
	line := `2026/07/07 08:00:01 [notice] 1#0: signal process started`
	rec, err := parseErrorLine(line)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Message != "signal process started" {
		t.Errorf("message = %q", rec.Message)
	}
}

func TestNginxErrorCollect(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "error.log")
	content := `2026/07/07 08:31:02 [error] 1234#0: *567 upstream timed out` + "\n" +
		`2026/07/07 07:00:00 [notice] 1#0: old message` + "\n" +
		"garbage line\n"
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	s := NewNginxError([]string{p})
	if s.Name() != "nginx-error" {
		t.Errorf("name = %q", s.Name())
	}
	from := time.Date(2026, 7, 7, 8, 0, 0, 0, time.Local)
	to := time.Date(2026, 7, 7, 9, 0, 0, 0, time.Local)
	var got []model.LogRecord
	stats, err := s.Collect(context.Background(), from, to, func(r model.LogRecord) { got = append(got, r) })
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || stats.Failed != 1 || stats.Lines != 3 {
		t.Fatalf("got %d recs, stats %+v", len(got), stats)
	}
}
