package main

import (
	"testing"
	"time"
)

func TestParseDur(t *testing.T) {
	cases := map[string]time.Duration{
		"1h":    time.Hour,
		"90m":   90 * time.Minute,
		"30s":   30 * time.Second,
		"1d":    24 * time.Hour,
		"2d":    48 * time.Hour,
		"1d12h": 36 * time.Hour,
		"0":     0,
	}
	for in, want := range cases {
		got, err := parseDur(in)
		if err != nil {
			t.Errorf("parseDur(%q) error: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("parseDur(%q) = %v, want %v", in, got, want)
		}
	}
	for _, bad := range []string{"", "abc", "1w", "-1h", "d"} {
		if _, err := parseDur(bad); err == nil {
			t.Errorf("parseDur(%q) should fail", bad)
		}
	}
}
