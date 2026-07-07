package source

import (
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/LeeSeokBln/faultbrief/internal/model"
)

// SyslogFile reads classic syslog files (RFC3164 and RFC5424 lines),
// including rotated .1 and .gz files.
type SyslogFile struct {
	Paths []string
}

func NewSyslogFile(paths []string) *SyslogFile { return &SyslogFile{Paths: paths} }

func (s *SyslogFile) Name() string { return "syslog" }

func (s *SyslogFile) Collect(ctx context.Context, from, to time.Time, emit func(model.LogRecord)) (Stats, error) {
	var stats Stats
	ref := to
	for _, p := range s.Paths {
		if err := ctx.Err(); err != nil {
			return stats, err
		}
		f, err := os.Open(p)
		if err != nil {
			return stats, fmt.Errorf("open %s: %w", p, err)
		}
		var r io.Reader = f
		if strings.HasSuffix(p, ".gz") {
			zr, err := gzip.NewReader(f)
			if err != nil {
				f.Close()
				return stats, fmt.Errorf("gzip %s: %w", p, err)
			}
			r = zr
		}
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			line := sc.Text()
			if strings.TrimSpace(line) == "" {
				continue
			}
			stats.Lines++
			rec, err := parseSyslogLine(line, ref)
			if err != nil {
				stats.Failed++
				continue
			}
			stats.Parsed++
			if !rec.TS.Before(from) && rec.TS.Before(to) {
				emit(rec)
			}
		}
		f.Close()
		if err := sc.Err(); err != nil {
			return stats, fmt.Errorf("read %s: %w", p, err)
		}
	}
	return stats, nil
}

var (
	// "Jul  7 08:12:03 host tag[pid]: message" / "... tag: message"
	re3164 = regexp.MustCompile(`^([A-Z][a-z]{2} [ \d]\d \d{2}:\d{2}:\d{2}) (\S+) ([^\s:\[]+)(?:\[(\d+)\])?: ?(.*)$`)
	rePri  = regexp.MustCompile(`^<(\d{1,3})>`)
)

var errUnparsable = fmt.Errorf("unparsable syslog line")

// parseSyslogLine parses one RFC3164 or RFC5424 line. ref supplies the year
// (RFC3164 has none) and the location for wall-clock timestamps.
func parseSyslogLine(line string, ref time.Time) (model.LogRecord, error) {
	sev := model.Severity(-1) // unknown; resolve later
	rest := line
	if m := rePri.FindStringSubmatch(line); m != nil {
		pri, _ := strconv.Atoi(m[1])
		sev = mapSyslogSeverity(pri % 8)
		rest = line[len(m[0]):]
	}
	if strings.HasPrefix(rest, "1 ") {
		return parse5424(rest, sev)
	}
	m := re3164.FindStringSubmatch(rest)
	if m == nil {
		return model.LogRecord{}, errUnparsable
	}
	ts, err := time.ParseInLocation("Jan _2 15:04:05", m[1], ref.Location())
	if err != nil {
		return model.LogRecord{}, errUnparsable
	}
	ts = ts.AddDate(ref.Year(), 0, 0)
	// RFC3164 has no year: if the stamped time lands notably after ref,
	// it belongs to the previous year (Dec logs read in Jan).
	if ts.After(ref.Add(48 * time.Hour)) {
		ts = ts.AddDate(-1, 0, 0)
	}
	msg := m[5]
	if sev == -1 {
		sev = keywordSeverity(msg)
	}
	return model.LogRecord{
		TS: ts, Source: "syslog", Unit: m[3], Severity: sev, Message: msg,
	}, nil
}

func parse5424(rest string, sev model.Severity) (model.LogRecord, error) {
	// VERSION SP TIMESTAMP SP HOSTNAME SP APP-NAME SP PROCID SP MSGID SP SD [SP MSG]
	parts := strings.SplitN(rest, " ", 8)
	if len(parts) < 7 {
		return model.LogRecord{}, errUnparsable
	}
	ts, err := time.Parse(time.RFC3339Nano, parts[1])
	if err != nil {
		return model.LogRecord{}, errUnparsable
	}
	msg := ""
	if len(parts) == 8 {
		msg = parts[7]
	}
	// Skip structured data if present (naive: "[...]" prefix).
	if strings.HasPrefix(msg, "[") {
		if end := strings.Index(msg, "] "); end >= 0 {
			msg = msg[end+2:]
		}
	}
	if sev == -1 {
		sev = keywordSeverity(msg)
	}
	return model.LogRecord{
		TS: ts, Source: "syslog", Unit: parts[3], Severity: sev, Message: msg,
	}, nil
}

func mapSyslogSeverity(s int) model.Severity {
	switch s {
	case 0, 1, 2:
		return model.SevCritical
	case 3:
		return model.SevError
	case 4:
		return model.SevWarning
	case 5:
		return model.SevNotice
	case 6:
		return model.SevInfo
	default:
		return model.SevDebug
	}
}

// keywordSeverity is a heuristic for lines without a syslog priority.
func keywordSeverity(msg string) model.Severity {
	l := strings.ToLower(msg)
	for _, kw := range []string{"error", "failed", "failure", "fatal", "panic", "crit"} {
		if strings.Contains(l, kw) {
			return model.SevError
		}
	}
	if strings.Contains(l, "warn") {
		return model.SevWarning
	}
	return model.SevInfo
}
