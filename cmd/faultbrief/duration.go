package main

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
)

var reDays = regexp.MustCompile(`^(\d+)d(.*)$`)

// parseDur parses Go durations plus a "d" (day) prefix unit: "1d", "1d12h".
func parseDur(s string) (time.Duration, error) {
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}
	if s == "0" {
		return 0, nil
	}
	var days time.Duration
	if m := reDays.FindStringSubmatch(s); m != nil {
		n, err := strconv.Atoi(m[1])
		if err != nil {
			return 0, fmt.Errorf("bad duration %q", s)
		}
		days = time.Duration(n) * 24 * time.Hour
		s = m[2]
		if s == "" {
			return days, nil
		}
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("bad duration %q", s)
	}
	if d < 0 {
		return 0, fmt.Errorf("negative duration %q", s)
	}
	return days + d, nil
}
