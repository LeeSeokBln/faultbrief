package report

import (
	"fmt"
	"io"
	"strings"
)

// RenderMarkdown writes the report as a markdown document (for tickets,
// Slack snippets, postmortems).
func RenderMarkdown(w io.Writer, r *Report, lang string) error {
	var b strings.Builder
	b.WriteString("# faultbrief — " + r.fmtTime(r.WindowFrom) + " → " + r.fmtTime(r.WindowTo) + "\n\n")
	b.WriteString(fmt.Sprintf("- %s: %s → %s\n", T(lang, "window"), r.fmtTime(r.WindowFrom), r.fmtTime(r.WindowTo)))
	b.WriteString(fmt.Sprintf("- %s: %d · %s: %d\n", T(lang, "records"), r.Records, T(lang, "findings"), len(r.Findings)))
	for _, s := range r.Sources {
		if s.Skipped {
			b.WriteString(fmt.Sprintf("- %s: %s (%s)\n", s.Name, T(lang, "skipped"), s.SkipReason))
		} else {
			line := fmt.Sprintf("- %s: %d lines, %d failed", s.Name, s.Lines, s.Failed)
			if s.HighFailure() {
				line += " — ⚠ " + T(lang, "high_parse_fail")
			}
			b.WriteString(line + "\n")
		}
	}
	if r.LLMBrief != "" {
		b.WriteString("\n## " + T(lang, "llm_brief") + "\n\n" + strings.TrimSpace(r.LLMBrief) + "\n")
	}
	b.WriteString("\n## " + T(lang, "findings") + "\n\n")
	if len(r.Findings) == 0 {
		b.WriteString(T(lang, "no_findings") + "\n")
	}
	for _, f := range r.Findings {
		b.WriteString(fmt.Sprintf("### **%s** · %s `%s` — %s (×%d)\n\n",
			SevName(lang, int(f.Severity)), T(lang, "kind."+string(f.Kind)), f.RuleID, f.Title, f.Count))
		if f.Detail != "" {
			b.WriteString(f.Detail + "\n\n")
		}
		for _, s := range f.Samples {
			b.WriteString("> `" + truncateLine(s, 200) + "`\n")
		}
		if f.Hint != "" {
			b.WriteString("\n_" + T(lang, "hint") + ": " + f.Hint + "_\n")
		}
		b.WriteString("\n")
	}
	_, err := io.WriteString(w, b.String())
	return err
}
