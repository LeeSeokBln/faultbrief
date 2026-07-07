package report

import (
	"fmt"
	"io"
	"strings"

	"github.com/LeeSeokBln/faultbrief/internal/model"
)

const (
	ansiReset  = "\x1b[0m"
	ansiBold   = "\x1b[1m"
	ansiRed    = "\x1b[31m"
	ansiYellow = "\x1b[33m"
	ansiCyan   = "\x1b[36m"
	ansiDim    = "\x1b[2m"
)

type painter struct{ on bool }

func (p painter) c(code, s string) string {
	if !p.on {
		return s
	}
	return code + s + ansiReset
}

func sevColor(s model.Severity) string {
	switch {
	case s >= model.SevError:
		return ansiRed
	case s == model.SevWarning:
		return ansiYellow
	default:
		return ansiCyan
	}
}

// RenderText writes the human-facing terminal report.
func RenderText(w io.Writer, r *Report, lang string, color bool) error {
	p := painter{on: color}
	var b strings.Builder

	b.WriteString(p.c(ansiBold, T(lang, "title")) + "\n")
	b.WriteString(fmt.Sprintf("%s: %s → %s (%s %s)\n",
		T(lang, "window"), r.fmtTime(r.WindowFrom), r.fmtTime(r.WindowTo),
		T(lang, "baseline"), r.BaselineTo.Sub(r.BaselineFrom)))

	var parts []string
	for _, s := range r.Sources {
		if s.Skipped {
			parts = append(parts, fmt.Sprintf("%s ✗ (%s: %s)", s.Name, T(lang, "skipped"), s.SkipReason))
			continue
		}
		part := fmt.Sprintf("%s ✓ (%d)", s.Name, s.Lines)
		if s.Failed > 0 {
			part += fmt.Sprintf(" [%d %s]", s.Failed, T(lang, "parse_failed"))
		}
		if s.HighFailure() {
			part += " ⚠ " + T(lang, "high_parse_fail")
		}
		parts = append(parts, part)
	}
	b.WriteString(fmt.Sprintf("%s: %s\n", T(lang, "sources"), strings.Join(parts, " · ")))
	b.WriteString(fmt.Sprintf("%s: %d · %s: %d\n", T(lang, "records"), r.Records, T(lang, "findings"), len(r.Findings)))

	if r.LLMBrief != "" {
		b.WriteString("\n" + p.c(ansiBold, "── "+T(lang, "llm_brief")+" ──") + "\n")
		b.WriteString(strings.TrimSpace(r.LLMBrief) + "\n")
	}

	b.WriteString("\n")
	if len(r.Findings) == 0 {
		b.WriteString(T(lang, "no_findings") + "\n")
		_, err := io.WriteString(w, b.String())
		return err
	}

	for _, f := range r.Findings {
		head := fmt.Sprintf("[%s] %s %s — %s (×%d)",
			SevName(lang, int(f.Severity)), T(lang, "kind."+string(f.Kind)), f.RuleID, f.Title, f.Count)
		b.WriteString(p.c(sevColor(f.Severity)+ansiBold, head) + "\n")
		meta := fmt.Sprintf("  %s: %s", T(lang, "first"), r.fmtClock(f.FirstTS))
		meta += fmt.Sprintf(" · %s: %s", T(lang, "last"), r.fmtClock(f.LastTS))
		if f.Unit != "" {
			meta += fmt.Sprintf(" · %s: %s", T(lang, "unit"), f.Unit)
		}
		meta += " · " + f.Source
		b.WriteString(p.c(ansiDim, meta) + "\n")
		if f.Detail != "" {
			b.WriteString("  " + f.Detail + "\n")
		}
		for _, s := range f.Samples {
			b.WriteString(p.c(ansiDim, "  > "+truncateLine(s, 160)) + "\n")
		}
		if f.Hint != "" {
			b.WriteString("  " + T(lang, "hint") + ": " + f.Hint + "\n")
		}
		b.WriteString("\n")
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func truncateLine(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
