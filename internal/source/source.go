// Package source turns raw logs (journald, syslog files, nginx logs) into
// normalized model.LogRecord streams.
package source

import (
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/LeeSeokBln/faultbrief/internal/model"
)

// Stats summarizes one source's collection pass.
type Stats struct {
	Lines  int // lines seen
	Parsed int // lines parsed into records (regardless of window)
	Failed int // lines that failed to parse
}

// FailRate returns the fraction of lines that failed to parse.
func (s Stats) FailRate() float64 {
	if s.Lines == 0 {
		return 0
	}
	return float64(s.Failed) / float64(s.Lines)
}

// Source streams records with TS in [from, to) to emit.
type Source interface {
	Name() string
	Collect(ctx context.Context, from, to time.Time, emit func(model.LogRecord)) (Stats, error)
}

// fileLines walks plain/.gz files line by line and applies a parser.
type fileLines struct {
	paths []string
}

func (fl fileLines) collect(ctx context.Context, from, to time.Time, emit func(model.LogRecord), parse func(string) (model.LogRecord, error)) (Stats, error) {
	var stats Stats
	for _, p := range fl.paths {
		if err := ctx.Err(); err != nil {
			return stats, err
		}
		f, err := os.Open(p)
		if err != nil {
			return stats, fmt.Errorf("open %s: %w", p, err)
		}

		var r io.Reader = f
		var zr *gzip.Reader
		if strings.HasSuffix(p, ".gz") {
			var err error
			zr, err = gzip.NewReader(f)
			if err != nil {
				f.Close()
				return stats, fmt.Errorf("gzip %s: %w", p, err)
			}
			r = zr
		}
		err = scanLines(r, maxLineBytes, func(b []byte) {
			line := string(b)
			if strings.TrimSpace(line) == "" {
				return
			}
			stats.Lines++
			rec, err := parse(line)
			if err != nil {
				stats.Failed++
				return
			}
			stats.Parsed++
			if !rec.TS.Before(from) && rec.TS.Before(to) {
				emit(rec)
			}
		}, func() {
			stats.Lines++
			stats.Failed++
		})
		if zr != nil {
			zr.Close()
		}
		f.Close()
		if err != nil {
			return stats, fmt.Errorf("read %s: %w", p, err)
		}
	}
	return stats, nil
}

// maxLineBytes bounds a single log line. Longer lines are counted as parse
// failures and skipped — one pathological line must not abort a source.
const maxLineBytes = 4 * 1024 * 1024

// scanLines reads r line by line, calling onLine for each complete line up
// to max bytes and onOversize for lines exceeding it.
func scanLines(r io.Reader, max int, onLine func([]byte), onOversize func()) error {
	br := bufio.NewReaderSize(r, 64*1024)
	buf := make([]byte, 0, 64*1024)
	tooLong := false
	for {
		chunk, isPrefix, err := br.ReadLine()
		if err != nil {
			if err == io.EOF {
				// bufio.ReadLine delivers a final unterminated line with
				// isPrefix=false before EOF, so nothing is pending here
				// unless the stream ended mid-oversize-line.
				if tooLong {
					onOversize()
				} else if len(buf) > 0 {
					onLine(buf)
				}
				return nil
			}
			return err
		}
		if !tooLong {
			if len(buf)+len(chunk) > max {
				tooLong = true
			} else {
				buf = append(buf, chunk...)
			}
		}
		if isPrefix {
			continue
		}
		if tooLong {
			onOversize()
		} else {
			onLine(buf)
		}
		buf = buf[:0]
		tooLong = false
	}
}
