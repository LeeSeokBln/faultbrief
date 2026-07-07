package app

import (
	"bytes"
	"context"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LeeSeokBln/faultbrief/internal/config"
)

var update = flag.Bool("update", false, "rewrite golden files")

func TestMain(m *testing.M) {
	// nginx error timestamps parse in time.Local; pin it for determinism.
	time.Local = time.UTC
	os.Exit(m.Run())
}

func e2eOptions(out, errOut *bytes.Buffer, format, lang string) Options {
	fx := func(n string) []string { return []string{filepath.Join("testdata", "e2e", n)} }
	return Options{
		Now:              time.Date(2026, 7, 7, 10, 0, 0, 0, time.UTC),
		Since:            time.Hour,
		BaselineSpan:     24 * time.Hour,
		Format:           format,
		Lang:             lang,
		MinSeverity:      "info",
		LLM:              config.Default().LLM,
		SyslogPaths:      fx("syslog.log"),
		NginxAccessPaths: fx("access.log"),
		NginxErrorPaths:  fx("error.log"),
		JournaldJSON:     filepath.Join("testdata", "e2e", "journal.ndjson"),
		Stdout:           out,
		Stderr:           errOut,
	}
}

func checkGolden(t *testing.T, got []byte, goldenName string) {
	t.Helper()
	golden := filepath.Join("testdata", "e2e", "golden", goldenName)
	if *update {
		if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(golden, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("missing golden (run: go test ./internal/app -run TestE2E -update): %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("golden mismatch for %s.\n--- got ---\n%s\n--- want ---\n%s", goldenName, got, want)
	}
}

func TestE2EGoldenText(t *testing.T) {
	for _, lang := range []string{"en", "ko"} {
		t.Run(lang, func(t *testing.T) {
			var out, errOut bytes.Buffer
			code := Run(context.Background(), e2eOptions(&out, &errOut, "text", lang))
			if code != ExitFindings {
				t.Fatalf("exit = %d, want 1; stderr: %s", code, errOut.String())
			}
			checkGolden(t, out.Bytes(), "brief."+lang+".txt")
		})
	}
}

func TestE2EGoldenJSON(t *testing.T) {
	var out, errOut bytes.Buffer
	code := Run(context.Background(), e2eOptions(&out, &errOut, "json", "en"))
	if code != ExitFindings {
		t.Fatalf("exit = %d; stderr: %s", code, errOut.String())
	}
	checkGolden(t, out.Bytes(), "brief.json")
}

// The e2e scenario must produce all five expected finding classes.
func TestE2EFindingClasses(t *testing.T) {
	var out, errOut bytes.Buffer
	code := Run(context.Background(), e2eOptions(&out, &errOut, "text", "en"))
	if code != ExitFindings {
		t.Fatalf("exit = %d, want 1; stderr: %s", code, errOut.String())
	}
	s := out.String()
	for _, want := range []string{
		"oom-kill",                  // syslog signature
		"systemd-unit-failed",       // journald signature
		"nginx-upstream-timeout",    // nginx-error signature
		"nginx-5xx-rate",            // nginx metric check
		"pg query timeout",          // spike template (masked title)
		"certificate verify failed", // novelty template
	} {
		if !strings.Contains(s, want) {
			t.Errorf("brief missing %q:\n%s", want, s)
		}
	}
	if strings.Contains(s, "heartbeat") {
		t.Errorf("healthy noise leaked into findings:\n%s", s)
	}
}

func TestE2ECacheSuppressesNoveltyOnSecondRun(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "patterns.json")
	run := func() (int, string) {
		var out, errOut bytes.Buffer
		o := e2eOptions(&out, &errOut, "text", "en")
		o.UseCache = true
		o.CachePath = cachePath
		code := Run(context.Background(), o)
		return code, out.String()
	}
	code1, first := run()
	if code1 != ExitFindings {
		t.Fatalf("first run exit = %d, want 1", code1)
	}
	if !strings.Contains(first, "certificate verify failed") {
		t.Fatalf("first run should report novelty:\n%s", first)
	}
	code2, second := run()
	if code2 != ExitFindings {
		t.Fatalf("second run exit = %d, want 1", code2)
	}
	if strings.Contains(second, "new pattern: 4 occurrence") {
		t.Errorf("second run must suppress cached novelty:\n%s", second)
	}
}
