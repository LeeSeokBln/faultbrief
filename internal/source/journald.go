package source

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/LeeSeokBln/faultbrief/internal/model"
)

// Runner abstracts process execution so journald parsing is testable on any
// OS (macOS dev boxes have no journalctl).
type Runner interface {
	Run(ctx context.Context, name string, args ...string) (io.ReadCloser, error)
}

// ExecRunner runs real commands.
type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, name string, args ...string) (io.ReadCloser, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &cmdReader{ReadCloser: stdout, cmd: cmd}, nil
}

type cmdReader struct {
	io.ReadCloser
	cmd *exec.Cmd
}

func (c *cmdReader) Close() error {
	c.ReadCloser.Close()
	return c.cmd.Wait()
}

// Journald streams `journalctl -o json` output.
type Journald struct {
	Runner Runner
}

func NewJournald() *Journald { return &Journald{Runner: ExecRunner{}} }

func (j *Journald) Name() string { return "journald" }

const journalTimeLayout = "2006-01-02 15:04:05"

func (j *Journald) Collect(ctx context.Context, from, to time.Time, emit func(model.LogRecord)) (Stats, error) {
	var stats Stats
	args := []string{
		"--since", from.Local().Format(journalTimeLayout),
		"--until", to.Local().Format(journalTimeLayout),
		"-o", "json", "--no-pager", "--quiet",
	}
	out, err := j.Runner.Run(ctx, "journalctl", args...)
	if err != nil {
		return stats, fmt.Errorf("run journalctl: %w", err)
	}
	defer out.Close()
	sc := bufio.NewScanner(out)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		stats.Lines++
		rec, err := parseJournalLine(line)
		if err != nil {
			stats.Failed++
			continue
		}
		stats.Parsed++
		if !rec.TS.Before(from) && rec.TS.Before(to) {
			emit(rec)
		}
	}
	if err := sc.Err(); err != nil {
		return stats, fmt.Errorf("read journalctl: %w", err)
	}
	return stats, nil
}

type journalEntry struct {
	Realtime  string          `json:"__REALTIME_TIMESTAMP"`
	Priority  string          `json:"PRIORITY"`
	Message   json.RawMessage `json:"MESSAGE"`
	Unit      string          `json:"_SYSTEMD_UNIT"`
	SyslogTag string          `json:"SYSLOG_IDENTIFIER"`
}

func parseJournalLine(line []byte) (model.LogRecord, error) {
	var e journalEntry
	if err := json.Unmarshal(line, &e); err != nil {
		return model.LogRecord{}, errUnparsable
	}
	usec, err := strconv.ParseInt(e.Realtime, 10, 64)
	if err != nil {
		return model.LogRecord{}, errUnparsable
	}
	ts := time.UnixMicro(usec).UTC()
	msg, err := decodeJournalMessage(e.Message)
	if err != nil {
		return model.LogRecord{}, errUnparsable
	}
	sev := model.SevInfo
	if e.Priority != "" {
		if p, err := strconv.Atoi(e.Priority); err == nil {
			sev = mapSyslogSeverity(p)
		}
	}
	unit := e.Unit
	if unit == "" {
		unit = e.SyslogTag
	}
	return model.LogRecord{TS: ts, Source: "journald", Unit: unit, Severity: sev, Message: msg}, nil
}

// decodeJournalMessage handles journald's two MESSAGE encodings: a JSON
// string, or an array of bytes for non-UTF8 payloads.
func decodeJournalMessage(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", fmt.Errorf("no message")
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s, nil
	}
	var ints []int
	if err := json.Unmarshal(raw, &ints); err == nil {
		b := make([]byte, len(ints))
		for i, v := range ints {
			b[i] = byte(v)
		}
		return string(b), nil
	}
	return "", fmt.Errorf("undecodable MESSAGE")
}

// JournaldFromFile returns a Journald that reads a saved `journalctl -o json`
// capture instead of executing journalctl. Used by tests and the hidden
// --journald-json flag.
func JournaldFromFile(path string) *Journald {
	return &Journald{Runner: fileRunner{path: path}}
}

type fileRunner struct{ path string }

func (f fileRunner) Run(ctx context.Context, name string, args ...string) (io.ReadCloser, error) {
	return os.Open(f.path)
}
