// Package app wires sources, detectors, reporting, and the optional LLM
// briefing into the single `faultbrief` run.
package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/LeeSeokBln/faultbrief/internal/baseline"
	"github.com/LeeSeokBln/faultbrief/internal/config"
	"github.com/LeeSeokBln/faultbrief/internal/detect"
	"github.com/LeeSeokBln/faultbrief/internal/llm"
	"github.com/LeeSeokBln/faultbrief/internal/model"
	"github.com/LeeSeokBln/faultbrief/internal/report"
	"github.com/LeeSeokBln/faultbrief/internal/rules"
)

// Exit codes are part of the CLI contract (cron/CI integration).
const (
	ExitHealthy  = 0
	ExitFindings = 1
	ExitError    = 2
)

// Options is the fully-resolved run configuration (flags+env+file merged).
type Options struct {
	Now          time.Time
	Since        time.Duration
	Until        time.Duration
	BaselineSpan time.Duration

	Format      string // text | md | json
	Lang        string // en | ko
	MinSeverity string
	Color       bool

	UseLLM bool
	LLM    config.LLM

	UseCache  bool
	CachePath string

	OnlySources      []string
	SyslogPaths      []string
	NginxAccessPaths []string
	NginxErrorPaths  []string
	JournaldJSON     string // hidden: journalctl -o json capture file
	RulesPaths       []string

	Stdout io.Writer
	Stderr io.Writer

	// LLMFactory overrides provider construction in tests. nil = llm.New.
	LLMFactory func(config.LLM) (llm.Provider, error)
}

// Run executes one brief and returns the process exit code. All user-facing
// errors are written to opts.Stderr.
func Run(ctx context.Context, opts Options) int {
	if err := validate(&opts); err != nil {
		fmt.Fprintln(opts.Stderr, "faultbrief:", err)
		return ExitError
	}
	minSev, _ := model.ParseSeverity(opts.MinSeverity)

	w := baseline.Compute(opts.Now, opts.Since, opts.Until, opts.BaselineSpan)
	acc := baseline.NewAccumulator(w, baseline.DefaultBucket)

	ruleSet, err := loadRules(opts.RulesPaths)
	if err != nil {
		fmt.Fprintln(opts.Stderr, "faultbrief:", err)
		return ExitError
	}
	matcher := detect.NewSignatureMatcher(ruleSet)

	entries := buildSources(opts)
	var stats []report.SourceStat
	records := 0
	active := 0
	for _, e := range entries {
		if e.src == nil {
			stats = append(stats, report.SourceStat{Name: e.name, Skipped: true, SkipReason: e.reason})
			continue
		}
		st, err := e.src.Collect(ctx, w.BaselineFrom, w.AnalysisTo, func(rec model.LogRecord) {
			acc.Add(rec)
			if !rec.TS.Before(w.AnalysisFrom) && rec.TS.Before(w.AnalysisTo) {
				matcher.Feed(rec)
				records++
			}
		})
		if err != nil {
			stats = append(stats, report.SourceStat{Name: e.name, Skipped: true, SkipReason: err.Error()})
			fmt.Fprintf(opts.Stderr, "faultbrief: warning: source %s skipped: %v\n", e.name, err)
			continue
		}
		active++
		stats = append(stats, report.SourceStat{Name: e.name, Lines: st.Lines, Parsed: st.Parsed, Failed: st.Failed})
	}
	if active == 0 {
		fmt.Fprintln(opts.Stderr, "faultbrief: no readable log sources (check paths/permissions; on Linux try sudo or the adm group)")
		return ExitError
	}

	var cache *baseline.Cache
	if opts.UseCache {
		cache, _ = baseline.LoadCache(opts.CachePath)
	}

	findings := matcher.Findings()
	params := detect.DefaultParams()
	findings = append(findings, detect.Spikes(acc, params)...)
	findings = append(findings, detect.Novelties(acc, cache, params)...)
	detect.Rank(findings)
	findings = detect.FilterMinSeverity(findings, minSev)

	if cache != nil {
		for id, st := range acc.Analysis {
			cache.Remember(id, st.Masked, st.LastTS)
		}
		if err := cache.Save(); err != nil {
			fmt.Fprintf(opts.Stderr, "faultbrief: warning: cache not saved: %v\n", err)
		}
	}

	rep := &report.Report{
		GeneratedAt:  opts.Now,
		WindowFrom:   w.AnalysisFrom,
		WindowTo:     w.AnalysisTo,
		BaselineFrom: w.BaselineFrom,
		BaselineTo:   w.BaselineTo,
		Records:      records,
		Sources:      stats,
		Findings:     findings,
		Loc:          opts.Now.Location(),
	}

	if opts.UseLLM {
		brief, err := runLLM(ctx, opts, rep)
		if err != nil {
			fmt.Fprintf(opts.Stderr, "faultbrief: warning: LLM briefing failed (%v); rule-engine report follows\n", err)
		} else {
			rep.LLMBrief = brief
		}
	}

	if err := render(opts, rep); err != nil {
		fmt.Fprintln(opts.Stderr, "faultbrief:", err)
		return ExitError
	}
	if len(findings) > 0 {
		return ExitFindings
	}
	return ExitHealthy
}

func validate(opts *Options) error {
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}
	if opts.Now.IsZero() {
		opts.Now = time.Now()
	}
	if opts.Since <= 0 {
		return fmt.Errorf("--since must be positive")
	}
	if opts.Until < 0 || opts.Until >= opts.Since {
		return fmt.Errorf("--until must be >= 0 and smaller than --since")
	}
	if opts.BaselineSpan <= 0 {
		return fmt.Errorf("--baseline must be positive")
	}
	switch opts.Format {
	case "text", "md", "json":
	default:
		return fmt.Errorf("unknown format %q (want text, md, or json)", opts.Format)
	}
	if _, ok := model.ParseSeverity(opts.MinSeverity); !ok {
		return fmt.Errorf("unknown min severity %q", opts.MinSeverity)
	}
	if opts.Lang != "en" && opts.Lang != "ko" {
		return fmt.Errorf("unsupported lang %q (want en or ko)", opts.Lang)
	}
	return nil
}

func loadRules(paths []string) ([]rules.Rule, error) {
	rs, err := rules.Builtin()
	if err != nil {
		return nil, fmt.Errorf("builtin rules: %w", err)
	}
	for _, p := range paths {
		f, err := os.Open(p)
		if err != nil {
			return nil, fmt.Errorf("rules file: %w", err)
		}
		extra, err := rules.Load(f)
		f.Close()
		if err != nil {
			return nil, fmt.Errorf("rules file %s: %w", p, err)
		}
		rs = append(rs, extra...)
	}
	return rs, nil
}

func render(opts Options, rep *report.Report) error {
	switch opts.Format {
	case "json":
		return report.RenderJSON(opts.Stdout, rep)
	case "md":
		return report.RenderMarkdown(opts.Stdout, rep, opts.Lang)
	default:
		return report.RenderText(opts.Stdout, rep, opts.Lang, opts.Color)
	}
}

// llmFindingsCap keeps the LLM context bounded.
const (
	llmFindingsCap   = 20
	llmSamplesCap    = 2
	llmSampleRuneCap = 300
)

func runLLM(ctx context.Context, opts Options, rep *report.Report) (string, error) {
	factory := opts.LLMFactory
	if factory == nil {
		factory = func(c config.LLM) (llm.Provider, error) { return llm.New(c, os.Getenv) }
	}
	provider, err := factory(opts.LLM)
	if err != nil {
		return "", err
	}
	trimmed := *rep
	if len(trimmed.Findings) > llmFindingsCap {
		trimmed.Findings = trimmed.Findings[:llmFindingsCap]
	}
	fs := make([]model.Finding, len(trimmed.Findings))
	copy(fs, trimmed.Findings)
	for i := range fs {
		samples := fs[i].Samples
		if len(samples) > llmSamplesCap {
			samples = samples[:llmSamplesCap]
		}
		cloned := make([]string, len(samples))
		copy(cloned, samples)
		for j, s := range cloned {
			r := []rune(s)
			if len(r) > llmSampleRuneCap {
				cloned[j] = string(r[:llmSampleRuneCap])
			}
		}
		fs[i].Samples = cloned
	}
	trimmed.Findings = fs
	payload, err := json.Marshal(&trimmed)
	if err != nil {
		return "", err
	}
	llmCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	return provider.Brief(llmCtx, llm.Request{Lang: opts.Lang, ReportJSON: payload})
}
