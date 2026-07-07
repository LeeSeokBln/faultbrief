// Package source turns raw logs (journald, syslog files, nginx logs) into
// normalized model.LogRecord streams.
package source

import (
	"context"
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
