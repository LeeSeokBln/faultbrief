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
		defer f.Close()

		var r io.Reader = f
		if strings.HasSuffix(p, ".gz") {
			zr, err := gzip.NewReader(f)
			if err != nil {
				return stats, fmt.Errorf("gzip %s: %w", p, err)
			}
			defer zr.Close()
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
			rec, err := parse(line)
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
			return stats, fmt.Errorf("read %s: %w", p, err)
		}
	}
	return stats, nil
}
