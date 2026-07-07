package app

import (
	"os/exec"
	"path/filepath"
	"sort"

	"github.com/LeeSeokBln/faultbrief/internal/source"
)

// sourceEntry pairs a buildable source with its report identity; src == nil
// means "known but skipped" with reason.
type sourceEntry struct {
	name   string
	src    source.Source
	reason string
}

// defaultGlobs are checked when no explicit paths are configured.
var defaultGlobs = map[string][]string{
	"syslog":       {"/var/log/syslog*", "/var/log/messages*"},
	"nginx-access": {"/var/log/nginx/access.log*"},
	"nginx-error":  {"/var/log/nginx/error.log*"},
}

func expand(patterns []string) []string {
	var out []string
	for _, p := range patterns {
		matches, err := filepath.Glob(p)
		if err != nil || len(matches) == 0 {
			// Not a glob hit; keep literal paths so open errors surface
			// as an explicit skip reason instead of silently vanishing.
			if !hasGlobMeta(p) {
				out = append(out, p)
			}
			continue
		}
		out = append(out, matches...)
	}
	sort.Strings(out)
	return out
}

func hasGlobMeta(p string) bool {
	for _, c := range p {
		if c == '*' || c == '?' || c == '[' {
			return true
		}
	}
	return false
}

// buildSources assembles the source list honoring explicit paths, the
// OnlySources filter, and platform autodetection.
func buildSources(opts Options) []sourceEntry {
	want := func(name string) bool {
		if len(opts.OnlySources) == 0 {
			return true
		}
		for _, s := range opts.OnlySources {
			if s == name || (s == "nginx" && (name == "nginx-access" || name == "nginx-error")) {
				return true
			}
		}
		return false
	}

	var entries []sourceEntry

	if want("journald") {
		switch {
		case opts.JournaldJSON != "":
			entries = append(entries, sourceEntry{name: "journald", src: source.JournaldFromFile(opts.JournaldJSON)})
		default:
			if _, err := exec.LookPath("journalctl"); err == nil {
				entries = append(entries, sourceEntry{name: "journald", src: source.NewJournald()})
			} else {
				entries = append(entries, sourceEntry{name: "journald", reason: "journalctl not found"})
			}
		}
	}

	fileSource := func(name string, explicit []string, mk func([]string) source.Source) {
		if !want(name) {
			return
		}
		paths := explicit
		if len(paths) == 0 {
			paths = defaultGlobs[name]
		}
		resolved := expand(paths)
		if len(resolved) == 0 {
			entries = append(entries, sourceEntry{name: name, reason: "no log files found"})
			return
		}
		entries = append(entries, sourceEntry{name: name, src: mk(resolved)})
	}

	fileSource("syslog", opts.SyslogPaths, func(p []string) source.Source { return source.NewSyslogFile(p) })
	fileSource("nginx-access", opts.NginxAccessPaths, func(p []string) source.Source { return source.NewNginxAccess(p) })
	fileSource("nginx-error", opts.NginxErrorPaths, func(p []string) source.Source { return source.NewNginxError(p) })
	return entries
}
