package source

import (
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LeeSeokBln/faultbrief/internal/model"
)

func TestParseSyslogLineRFC3164(t *testing.T) {
	ref := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	rec, err := parseSyslogLine("Jul  7 08:12:03 web1 kernel: Out of memory: Killed process 1234 (myapp)", ref)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Unit != "kernel" {
		t.Errorf("unit = %q", rec.Unit)
	}
	if rec.Message != "Out of memory: Killed process 1234 (myapp)" {
		t.Errorf("message = %q", rec.Message)
	}
	want := time.Date(2026, 7, 7, 8, 12, 3, 0, time.UTC)
	if !rec.TS.Equal(want) {
		t.Errorf("ts = %v, want %v", rec.TS, want)
	}
	// no <pri>, message contains no error keyword beyond heuristic list:
	// "Out of memory" doesn't contain "error"/"failed"; severity stays info.
	// (The oom signature rule carries the real severity.)
	if rec.Severity != model.SevInfo {
		t.Errorf("severity = %v, want info", rec.Severity)
	}
}

func TestParseSyslogLineWithTagPID(t *testing.T) {
	ref := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	rec, err := parseSyslogLine("Jul  7 09:14:01 web1 CRON[123]: (root) CMD (run-parts /etc/cron.hourly)", ref)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Unit != "CRON" {
		t.Errorf("unit = %q, want CRON", rec.Unit)
	}
}

func TestParseSyslogLineKeywordSeverity(t *testing.T) {
	ref := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	rec, _ := parseSyslogLine("Jul  7 09:00:00 web1 myapp[9]: request failed with timeout", ref)
	if rec.Severity != model.SevError {
		t.Errorf("severity = %v, want error (keyword heuristic)", rec.Severity)
	}
	rec, _ = parseSyslogLine("Jul  7 09:00:00 web1 myapp[9]: warning: disk usage at 81 percent", ref)
	if rec.Severity != model.SevWarning {
		t.Errorf("severity = %v, want warning", rec.Severity)
	}
}

func TestParseSyslogLineWithPri(t *testing.T) {
	ref := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	// pri 11 = facility 1, severity 3 (err)
	rec, err := parseSyslogLine("<11>Jul  7 09:00:00 web1 app: boom", ref)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Severity != model.SevError {
		t.Errorf("severity = %v, want error", rec.Severity)
	}
}

func TestParseSyslogLineRFC5424(t *testing.T) {
	ref := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	rec, err := parseSyslogLine(`<165>1 2026-07-07T08:14:15.003Z web1 evtsys 1234 ID47 - service crashed`, ref)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Unit != "evtsys" || rec.Message != "service crashed" {
		t.Errorf("unit=%q msg=%q", rec.Unit, rec.Message)
	}
	// 165 % 8 = 5 -> notice
	if rec.Severity != model.SevNotice {
		t.Errorf("severity = %v, want notice", rec.Severity)
	}
	want := time.Date(2026, 7, 7, 8, 14, 15, 3_000_000, time.UTC)
	if !rec.TS.Equal(want) {
		t.Errorf("ts = %v, want %v", rec.TS, want)
	}
}

func TestYearRollover(t *testing.T) {
	// ref is Jan 2; a Dec 31 log line must land in the previous year.
	ref := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	rec, err := parseSyslogLine("Dec 31 23:59:00 web1 app: bye", ref)
	if err != nil {
		t.Fatal(err)
	}
	if rec.TS.Year() != 2025 {
		t.Errorf("year = %d, want 2025", rec.TS.Year())
	}
}

func TestSyslogCollectFiltersAndReadsGzip(t *testing.T) {
	dir := t.TempDir()
	plain := filepath.Join(dir, "syslog")
	content := "Jul  7 08:30:00 web1 app: inside window\n" +
		"Jul  7 06:00:00 web1 app: before window\n" +
		"not a syslog line at all\n"
	if err := os.WriteFile(plain, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	gz := filepath.Join(dir, "syslog.2.gz")
	f, err := os.Create(gz)
	if err != nil {
		t.Fatal(err)
	}
	zw := gzip.NewWriter(f)
	zw.Write([]byte("Jul  7 08:45:00 web1 app: gzipped inside window\n"))
	zw.Close()
	f.Close()

	s := NewSyslogFile([]string{plain, gz})
	from := time.Date(2026, 7, 7, 8, 0, 0, 0, time.UTC)
	to := time.Date(2026, 7, 7, 9, 0, 0, 0, time.UTC)
	var got []model.LogRecord
	stats, err := s.Collect(context.Background(), from, to, func(r model.LogRecord) { got = append(got, r) })
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("emitted %d records, want 2 (in-window plain + gz)", len(got))
	}
	if stats.Lines != 4 || stats.Failed != 1 {
		t.Errorf("stats = %+v, want Lines=4 Failed=1", stats)
	}
}

func TestOversizedLineSkippedNotFatal(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "syslog")
	huge := strings.Repeat("x", maxLineBytes+100)
	content := "Jul  7 08:30:00 web1 app: before huge line\n" +
		huge + "\n" +
		"Jul  7 08:45:00 web1 app: after huge line\n"
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	s := NewSyslogFile([]string{p})
	from := time.Date(2026, 7, 7, 8, 0, 0, 0, time.UTC)
	to := time.Date(2026, 7, 7, 9, 0, 0, 0, time.UTC)
	var got []model.LogRecord
	stats, err := s.Collect(context.Background(), from, to, func(r model.LogRecord) { got = append(got, r) })
	if err != nil {
		t.Fatalf("oversized line must not abort the source: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("records after the huge line must still parse; got %d, want 2", len(got))
	}
	if stats.Failed != 1 {
		t.Errorf("oversized line should count as 1 failure, stats=%+v", stats)
	}
}
