package source

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/LeeSeokBln/faultbrief/internal/model"
)

type fakeRunner struct {
	output string
	args   []string
	err    error
}

func (f *fakeRunner) Run(ctx context.Context, name string, args ...string) (io.ReadCloser, error) {
	f.args = args
	if f.err != nil {
		return nil, f.err
	}
	return io.NopCloser(strings.NewReader(f.output)), nil
}

const journalFixture = `{"__REALTIME_TIMESTAMP":"1783413123000000","PRIORITY":"3","MESSAGE":"connection refused to backend","_SYSTEMD_UNIT":"myapp.service"}
{"__REALTIME_TIMESTAMP":"1783413124000000","MESSAGE":[104,101,108,108,111],"SYSLOG_IDENTIFIER":"weird"}
{"__REALTIME_TIMESTAMP":"1783413125000000","PRIORITY":"6","MESSAGE":"all good"}
not json at all
`

// 1783413123 sec = 2026-07-07T08:32:03Z (verified: TZ=UTC date -r 1783413123).
func TestJournaldCollect(t *testing.T) {
	fr := &fakeRunner{output: journalFixture}
	j := &Journald{Runner: fr}
	if j.Name() != "journald" {
		t.Errorf("name = %q", j.Name())
	}
	from := time.Date(2026, 7, 7, 8, 0, 0, 0, time.UTC)
	to := time.Date(2026, 7, 7, 9, 0, 0, 0, time.UTC)
	var got []model.LogRecord
	stats, err := j.Collect(context.Background(), from, to, func(r model.LogRecord) { got = append(got, r) })
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d records, want 3", len(got))
	}
	if got[0].Severity != model.SevError || got[0].Unit != "myapp.service" {
		t.Errorf("rec0 = %+v", got[0])
	}
	if got[1].Message != "hello" || got[1].Unit != "weird" || got[1].Severity != model.SevInfo {
		t.Errorf("rec1 = %+v (byte-array MESSAGE must decode, missing PRIORITY -> info)", got[1])
	}
	if stats.Failed != 1 {
		t.Errorf("stats = %+v, want Failed=1", stats)
	}
	// journalctl must be invoked with both windows covered by since/until.
	joined := strings.Join(fr.args, " ")
	for _, want := range []string{"-o json", "--no-pager", "--since", "--until"} {
		if !strings.Contains(joined, want) {
			t.Errorf("journalctl args missing %q: %v", want, fr.args)
		}
	}
}

func TestJournaldRunnerError(t *testing.T) {
	fr := &fakeRunner{err: io.ErrUnexpectedEOF}
	j := &Journald{Runner: fr}
	_, err := j.Collect(context.Background(), time.Now().Add(-time.Hour), time.Now(), func(model.LogRecord) {})
	if err == nil {
		t.Fatal("expected error when journalctl cannot run")
	}
}

func TestJournaldOversizedLineSkipped(t *testing.T) {
	huge := strings.Repeat("y", maxLineBytes+100)
	out := `{"__REALTIME_TIMESTAMP":"1783413123000000","PRIORITY":"3","MESSAGE":"real entry","_SYSTEMD_UNIT":"a.service"}` + "\n" +
		huge + "\n" +
		`{"__REALTIME_TIMESTAMP":"1783413125000000","PRIORITY":"6","MESSAGE":"after huge"}` + "\n"
	j := &Journald{Runner: &fakeRunner{output: out}}
	from := time.Date(2026, 7, 7, 8, 0, 0, 0, time.UTC)
	to := time.Date(2026, 7, 7, 9, 0, 0, 0, time.UTC)
	var got []model.LogRecord
	stats, err := j.Collect(context.Background(), from, to, func(r model.LogRecord) { got = append(got, r) })
	if err != nil {
		t.Fatalf("oversized journal line must not abort the source: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d records, want 2 (entries around the huge line)", len(got))
	}
	if stats.Failed != 1 {
		t.Errorf("stats=%+v, want Failed=1", stats)
	}
}
