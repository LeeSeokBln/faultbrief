// Package report renders findings as terminal text, markdown, or JSON.
package report

import (
	"encoding/json"
	"io"
	"time"

	"github.com/LeeSeokBln/faultbrief/internal/model"
)

// SourceStat describes one source's scan result for the report header.
type SourceStat struct {
	Name       string `json:"name"`
	Lines      int    `json:"lines"`
	Parsed     int    `json:"parsed"`
	Failed     int    `json:"failed"`
	Skipped    bool   `json:"skipped,omitempty"`
	SkipReason string `json:"skip_reason,omitempty"`
}

// HighFailure reports whether the parse-failure rate exceeds the spec's 20%
// warning threshold.
func (s SourceStat) HighFailure() bool {
	return s.Lines > 0 && float64(s.Failed)/float64(s.Lines) > 0.2
}

// Report is the complete brief, ready to render in any format.
type Report struct {
	GeneratedAt  time.Time       `json:"generated_at"`
	WindowFrom   time.Time       `json:"window_from"`
	WindowTo     time.Time       `json:"window_to"`
	BaselineFrom time.Time       `json:"baseline_from"`
	BaselineTo   time.Time       `json:"baseline_to"`
	Records      int             `json:"records"`
	Sources      []SourceStat    `json:"sources"`
	Findings     []model.Finding `json:"findings"`
	LLMBrief     string          `json:"llm_brief,omitempty"`

	// Loc controls how text/markdown renderers format times. Not serialized.
	Loc *time.Location `json:"-"`
}

func (r *Report) location() *time.Location {
	if r.Loc != nil {
		return r.Loc
	}
	return time.Local
}

func (r *Report) fmtTime(t time.Time) string {
	return t.In(r.location()).Format("2006-01-02 15:04")
}

func (r *Report) fmtClock(t time.Time) string {
	return t.In(r.location()).Format("15:04:05")
}

// RenderJSON writes the machine-readable report.
func RenderJSON(w io.Writer, r *Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}
