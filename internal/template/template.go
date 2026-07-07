// Package template reduces raw log messages to stable fingerprints by masking
// variable parts (numbers, IPs, UUIDs, hex ids, paths, quoted strings).
package template

import (
	"fmt"
	"hash/fnv"
	"regexp"
	"strings"
)

var (
	reQuoted      = regexp.MustCompile(`"[^"]*"|'[^']*'`)
	reUUID        = regexp.MustCompile(`(?i)[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
	reIPv6        = regexp.MustCompile(`[0-9a-fA-F:]*(?:[0-9a-fA-F]{1,4}:){2,7}[0-9a-fA-F]{1,4}`)
	reIPv4        = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}(?::\d+)?\b`)
	reHex         = regexp.MustCompile(`\b(?:0x)?[0-9a-fA-F]{8,}\b`)
	rePath        = regexp.MustCompile(`/[\w.\-]+(?:/[\w.\-]+)+`)
	reNum         = regexp.MustCompile(`\d+(?:\.\d+)?`)
	reStatusClass = regexp.MustCompile(`\[[1-5]xx\]`)
)

const statusClassGuard = "\x00SC\x00"

// Mask replaces variable tokens with placeholders. The order of passes is
// fixed and is part of the fingerprint contract; changing it invalidates
// caches and golden files.
func Mask(msg string) string {
	// Preserve nginx status-class tokens like [5xx] emitted by the access source.
	guards := reStatusClass.FindAllString(msg, -1)
	m := reStatusClass.ReplaceAllString(msg, statusClassGuard)
	m = reQuoted.ReplaceAllString(m, "<STR>")
	m = reUUID.ReplaceAllString(m, "<UUID>")
	m = reIPv6.ReplaceAllStringFunc(m, func(s string) string {
		// Require hex letters or "::" so clock values like 15:04:05 pass through.
		if strings.ContainsAny(s, "abcdefABCDEF") || strings.Contains(s, "::") {
			return "<IP>"
		}
		return s
	})
	m = reIPv4.ReplaceAllString(m, "<IP>")
	m = reHex.ReplaceAllString(m, "<HEX>")
	m = rePath.ReplaceAllString(m, "<PATH>")
	m = reNum.ReplaceAllString(m, "<NUM>")
	for _, g := range guards {
		m = strings.Replace(m, statusClassGuard, g, 1)
	}
	return m
}

// Fingerprint returns a stable 16-char hex id for a masked message within a
// source namespace.
func Fingerprint(source, masked string) string {
	h := fnv.New64a()
	h.Write([]byte(source))
	h.Write([]byte{0})
	h.Write([]byte(masked))
	return fmt.Sprintf("%016x", h.Sum64())
}
