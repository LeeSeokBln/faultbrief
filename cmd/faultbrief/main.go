package main

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/LeeSeokBln/faultbrief/internal/app"
	"github.com/LeeSeokBln/faultbrief/internal/config"
)

var version = "dev"

func main() {
	os.Exit(run())
}

func run() int {
	var (
		sinceStr, untilStr, baselineStr    string
		format, lang, minSev, configPath   string
		useLLM, useCache, noColor          bool
		onlySources                        []string
		syslogPaths, accessPaths, errPaths []string
		rulesPaths                         []string
		nowStr, journaldJSON               string
	)

	root := &cobra.Command{
		Use:   "faultbrief",
		Short: "Turn Linux logs into an incident brief",
		Long: "faultbrief parses journald/syslog/nginx logs, detects incident signals\n" +
			"with a rule engine (signatures, spikes, novel patterns), and optionally\n" +
			"adds an LLM briefing. Works without any LLM configured.",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(configPath)
			if err != nil {
				return err
			}
			// flags > env/file: only apply file/env values when flag unchanged
			if !cmd.Flags().Changed("format") {
				format = cfg.Format
			}
			if !cmd.Flags().Changed("lang") {
				lang = cfg.Lang
			}
			if !cmd.Flags().Changed("min-severity") {
				minSev = cfg.MinSeverity
			}
			if !cmd.Flags().Changed("use-cache") {
				useCache = cfg.UseCache
			}
			if !cmd.Flags().Changed("baseline") {
				baselineStr = fmt.Sprintf("%dh", cfg.BaselineHours)
			}
			if len(syslogPaths) == 0 {
				syslogPaths = cfg.SyslogPaths
			}
			if len(accessPaths) == 0 {
				accessPaths = cfg.NginxAccessPaths
			}
			if len(errPaths) == 0 {
				errPaths = cfg.NginxErrorPaths
			}
			if len(rulesPaths) == 0 {
				rulesPaths = cfg.RulesPaths
			}

			since, err := parseDur(sinceStr)
			if err != nil {
				return fmt.Errorf("--since: %w", err)
			}
			until, err := parseDur(untilStr)
			if err != nil {
				return fmt.Errorf("--until: %w", err)
			}
			baselineSpan, err := parseDur(baselineStr)
			if err != nil {
				return fmt.Errorf("--baseline: %w", err)
			}

			now := time.Now()
			if nowStr != "" {
				now, err = time.Parse(time.RFC3339, nowStr)
				if err != nil {
					return fmt.Errorf("--now: %w", err)
				}
			}

			color := !noColor && os.Getenv("NO_COLOR") == "" && format == "text" &&
				term.IsTerminal(int(os.Stdout.Fd()))

			opts := app.Options{
				Now: now, Since: since, Until: until, BaselineSpan: baselineSpan,
				Format: format, Lang: lang, MinSeverity: minSev, Color: color,
				UseLLM: useLLM, LLM: cfg.LLM,
				UseCache: useCache, CachePath: config.DefaultCachePath(),
				OnlySources: onlySources,
				SyslogPaths: syslogPaths, NginxAccessPaths: accessPaths, NginxErrorPaths: errPaths,
				JournaldJSON: journaldJSON, RulesPaths: rulesPaths,
				Stdout: os.Stdout, Stderr: os.Stderr,
			}
			code := app.Run(cmd.Context(), opts)
			if code != 0 {
				// Exit codes 1 (findings) and 2 (error) are the contract;
				// cobra returns via error only for usage problems, so exit
				// directly here.
				os.Exit(code)
			}
			return nil
		},
	}

	f := root.Flags()
	f.StringVar(&sinceStr, "since", "1h", "analysis window size (e.g. 30m, 1h, 1d)")
	f.StringVar(&untilStr, "until", "0", "skip the most recent span (e.g. 10m)")
	f.StringVar(&baselineStr, "baseline", "24h", "baseline span before the window")
	f.StringVar(&format, "format", "text", "output format: text, md, json")
	f.StringVar(&lang, "lang", "en", "report language: en, ko")
	f.StringVar(&minSev, "min-severity", "info", "minimum finding severity to report")
	f.BoolVar(&useLLM, "llm", false, "add an LLM incident briefing")
	f.BoolVar(&useCache, "use-cache", false, "use the long-term pattern cache")
	f.BoolVar(&noColor, "no-color", false, "disable colored output")
	f.StringSliceVar(&onlySources, "source", nil, "limit sources: journald, syslog, nginx, nginx-access, nginx-error")
	f.StringSliceVar(&syslogPaths, "syslog-path", nil, "syslog file paths/globs")
	f.StringSliceVar(&accessPaths, "nginx-access-path", nil, "nginx access log paths/globs")
	f.StringSliceVar(&errPaths, "nginx-error-path", nil, "nginx error log paths/globs")
	f.StringSliceVar(&rulesPaths, "rules", nil, "extra signature rule YAML files")
	f.StringVar(&configPath, "config", config.DefaultPath(), "config file path")
	f.StringVar(&nowStr, "now", "", "override current time (RFC3339); for tests")
	f.StringVar(&journaldJSON, "journald-json", "", "read journald from a journalctl -o json capture; for tests")
	f.MarkHidden("now")
	f.MarkHidden("journald-json")

	root.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("faultbrief", version)
		},
	})

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "faultbrief:", err)
		return app.ExitError
	}
	return 0
}
