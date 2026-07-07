package source

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/LeeSeokBln/faultbrief/internal/model"
)

func TestParseAccessLine(t *testing.T) {
	line := `203.0.113.9 - - [07/Jul/2026:08:30:15 +0000] "GET /api/users/123 HTTP/1.1" 500 1234 "-" "curl/8.0"`
	rec, err := parseAccessLine(line)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Severity != model.SevError {
		t.Errorf("500 should map to error, got %v", rec.Severity)
	}
	if rec.Message != "GET /api/users/123 [5xx]" {
		t.Errorf("message = %q", rec.Message)
	}
	if rec.Fields["status"] != "500" || rec.Fields["path"] != "/api/users/123" {
		t.Errorf("fields = %v", rec.Fields)
	}
	want := time.Date(2026, 7, 7, 8, 30, 15, 0, time.UTC)
	if !rec.TS.Equal(want) {
		t.Errorf("ts = %v, want %v", rec.TS, want)
	}
}

func TestParseAccessLineSeverities(t *testing.T) {
	mk := func(status string) string {
		return `1.2.3.4 - - [07/Jul/2026:08:30:15 +0000] "GET / HTTP/1.1" ` + status + ` 0 "-" "-"`
	}
	cases := map[string]model.Severity{"200": model.SevInfo, "301": model.SevInfo, "404": model.SevNotice, "503": model.SevError}
	for status, want := range cases {
		rec, err := parseAccessLine(mk(status))
		if err != nil {
			t.Fatalf("%s: %v", status, err)
		}
		if rec.Severity != want {
			t.Errorf("status %s -> %v, want %v", status, rec.Severity, want)
		}
	}
}

func TestParseAccessLineMalformedRequest(t *testing.T) {
	// Broken clients send garbage request lines; status class still parses.
	line := `1.2.3.4 - - [07/Jul/2026:08:30:15 +0000] "\x16\x03\x01" 400 0 "-" "-"`
	rec, err := parseAccessLine(line)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Fields["status"] != "400" {
		t.Errorf("fields = %v", rec.Fields)
	}
}

func TestNginxAccessCollect(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "access.log")
	content := `1.2.3.4 - - [07/Jul/2026:08:30:15 +0000] "GET /ok HTTP/1.1" 200 5 "-" "-"` + "\n" +
		`1.2.3.4 - - [07/Jul/2026:07:00:00 +0000] "GET /old HTTP/1.1" 200 5 "-" "-"` + "\n" +
		"garbage line\n"
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	s := NewNginxAccess([]string{p})
	if s.Name() != "nginx-access" {
		t.Errorf("name = %q", s.Name())
	}
	from := time.Date(2026, 7, 7, 8, 0, 0, 0, time.UTC)
	to := time.Date(2026, 7, 7, 9, 0, 0, 0, time.UTC)
	var got []model.LogRecord
	stats, err := s.Collect(context.Background(), from, to, func(r model.LogRecord) { got = append(got, r) })
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || stats.Failed != 1 || stats.Lines != 3 {
		t.Fatalf("got %d recs, stats %+v", len(got), stats)
	}
}
