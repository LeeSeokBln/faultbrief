package app

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LeeSeokBln/faultbrief/internal/config"
)

func fixedNow() time.Time {
	return time.Date(2026, 7, 7, 10, 0, 0, 0, time.UTC)
}

func writeFixture(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func baseOptions(t *testing.T, dir string) Options {
	var out, errOut bytes.Buffer
	_ = errOut
	return Options{
		Now:          fixedNow(),
		Since:        time.Hour,
		BaselineSpan: 24 * time.Hour,
		Format:       "text",
		Lang:         "en",
		MinSeverity:  "info",
		LLM:          config.Default().LLM,
		Stdout:       &out,
		Stderr:       &bytes.Buffer{},
	}
}

func TestRunFindingsExitCode(t *testing.T) {
	dir := t.TempDir()
	syslog := writeFixture(t, dir, "syslog",
		"Jul  7 09:12:03 web1 kernel: Out of memory: Killed process 1234 (myapp)\n")
	opts := baseOptions(t, dir)
	var out bytes.Buffer
	opts.Stdout = &out
	opts.SyslogPaths = []string{syslog}
	code := Run(context.Background(), opts)
	if code != ExitFindings {
		t.Fatalf("exit = %d, want %d (findings present)", code, ExitFindings)
	}
	if !strings.Contains(out.String(), "oom-kill") {
		t.Errorf("output missing finding:\n%s", out.String())
	}
}

func TestRunHealthyExitCode(t *testing.T) {
	dir := t.TempDir()
	syslog := writeFixture(t, dir, "syslog",
		"Jul  7 09:12:03 web1 app: routine heartbeat ok\n"+
			"Jul  6 12:00:00 web1 app: routine heartbeat ok\n")
	opts := baseOptions(t, dir)
	opts.SyslogPaths = []string{syslog}
	code := Run(context.Background(), opts)
	if code != ExitHealthy {
		t.Fatalf("exit = %d, want %d", code, ExitHealthy)
	}
}

func TestRunNoSourcesIsError(t *testing.T) {
	dir := t.TempDir()
	opts := baseOptions(t, dir)
	// Explicit nonexistent paths: deterministic regardless of whether the
	// machine happens to have /var/log/nginx.
	opts.OnlySources = []string{"nginx"}
	opts.NginxAccessPaths = []string{filepath.Join(dir, "nope-access.log")}
	opts.NginxErrorPaths = []string{filepath.Join(dir, "nope-error.log")}
	var errOut bytes.Buffer
	opts.Stderr = &errOut
	code := Run(context.Background(), opts)
	if code != ExitError {
		t.Fatalf("exit = %d, want %d", code, ExitError)
	}
	if errOut.Len() == 0 {
		t.Error("expected an error message on stderr")
	}
}

func TestRunSourceFailureIsSkippedNotFatal(t *testing.T) {
	dir := t.TempDir()
	good := writeFixture(t, dir, "syslog", "Jul  7 09:12:03 web1 app: fine\n")
	opts := baseOptions(t, dir)
	opts.SyslogPaths = []string{good}
	opts.NginxAccessPaths = []string{filepath.Join(dir, "does-not-exist.log")}
	var out bytes.Buffer
	opts.Stdout = &out
	code := Run(context.Background(), opts)
	if code == ExitError {
		t.Fatalf("one broken source must not be fatal, got exit %d, stderr shown to user", code)
	}
	if !strings.Contains(out.String(), "skipped") {
		t.Errorf("report should note the skipped source:\n%s", out.String())
	}
}

func TestRunJSONFormat(t *testing.T) {
	dir := t.TempDir()
	syslog := writeFixture(t, dir, "syslog",
		"Jul  7 09:12:03 web1 kernel: Out of memory: Killed process 1234 (x)\n")
	opts := baseOptions(t, dir)
	opts.SyslogPaths = []string{syslog}
	opts.Format = "json"
	var out bytes.Buffer
	opts.Stdout = &out
	Run(context.Background(), opts)
	if !strings.HasPrefix(strings.TrimSpace(out.String()), "{") {
		t.Errorf("json output expected:\n%s", out.String())
	}
}

func TestRunMinSeverityFilters(t *testing.T) {
	dir := t.TempDir()
	// ssh-auth-fail is severity=warning
	syslog := writeFixture(t, dir, "syslog",
		"Jul  7 09:12:03 web1 sshd[1]: Failed password for root from 203.0.113.9 port 1 ssh2\n")
	opts := baseOptions(t, dir)
	opts.SyslogPaths = []string{syslog}
	opts.MinSeverity = "error"
	var out bytes.Buffer
	opts.Stdout = &out
	code := Run(context.Background(), opts)
	if code != ExitHealthy {
		t.Fatalf("warning finding must be filtered by min-severity=error, exit=%d", code)
	}
}

func TestRunInvalidOptions(t *testing.T) {
	opts := baseOptions(t, t.TempDir())
	opts.Format = "pdf"
	if code := Run(context.Background(), opts); code != ExitError {
		t.Errorf("bad format: exit = %d, want 2", code)
	}
	opts = baseOptions(t, t.TempDir())
	opts.Since = 0
	if code := Run(context.Background(), opts); code != ExitError {
		t.Errorf("zero since: exit = %d, want 2", code)
	}
}

func TestRunWithJournaldFixture(t *testing.T) {
	dir := t.TempDir()
	// 1783414800 sec = 2026-07-07T09:00:00Z (verified with date -r), which is
	// the inclusive start of the 09:00-10:00 analysis window.
	nd := writeFixture(t, dir, "journal.ndjson",
		`{"__REALTIME_TIMESTAMP":"1783414800000000","PRIORITY":"3","MESSAGE":"myapp.service: Failed with result 'exit-code'.","_SYSTEMD_UNIT":"myapp.service"}`+"\n")
	opts := baseOptions(t, dir)
	opts.JournaldJSON = nd
	opts.OnlySources = []string{"journald"}
	var out bytes.Buffer
	opts.Stdout = &out
	code := Run(context.Background(), opts)
	if code != ExitFindings {
		t.Fatalf("exit = %d, want findings; out:\n%s", code, out.String())
	}
	if !strings.Contains(out.String(), "systemd-unit-failed") {
		t.Errorf("journald signature missing:\n%s", out.String())
	}
}
