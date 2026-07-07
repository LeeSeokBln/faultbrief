// Package model defines the normalized log record and finding types shared
// by every stage of the faultbrief pipeline.
package model

import (
	"encoding/json"
	"fmt"
	"time"
)

// Severity is ordered: higher value means more severe.
type Severity int

const (
	SevDebug Severity = iota
	SevInfo
	SevNotice
	SevWarning
	SevError
	SevCritical
)

var sevNames = [...]string{"debug", "info", "notice", "warning", "error", "critical"}

func (s Severity) String() string {
	if s < SevDebug || s > SevCritical {
		return "unknown"
	}
	return sevNames[s]
}

// ParseSeverity converts a lowercase severity name into a Severity.
func ParseSeverity(name string) (Severity, bool) {
	for i, n := range sevNames {
		if n == name {
			return Severity(i), true
		}
	}
	return SevInfo, false
}

func (s Severity) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

func (s *Severity) UnmarshalJSON(b []byte) error {
	var name string
	if err := json.Unmarshal(b, &name); err != nil {
		return err
	}
	sev, ok := ParseSeverity(name)
	if !ok {
		return fmt.Errorf("unknown severity %q", name)
	}
	*s = sev
	return nil
}

// LogRecord is one normalized log entry from any source.
type LogRecord struct {
	TS       time.Time
	Source   string // "journald" | "syslog" | "nginx-access" | "nginx-error"
	Unit     string // systemd unit, syslog tag, or "nginx"
	Severity Severity
	Message  string
	Fields   map[string]string // source-specific extras, e.g. "status" for nginx-access
}

// FindingKind identifies which detector produced a finding.
type FindingKind string

const (
	KindSignature FindingKind = "signature"
	KindSpike     FindingKind = "spike"
	KindNovelty   FindingKind = "novelty"
)

// Finding is one detected incident indicator, ready for reporting.
type Finding struct {
	Kind     FindingKind `json:"kind"`
	RuleID   string      `json:"rule_id"`
	Severity Severity    `json:"severity"`
	Title    string      `json:"title"`
	Detail   string      `json:"detail"`
	Hint     string      `json:"hint,omitempty"`
	Count    int         `json:"count"`
	Score    float64     `json:"score"`
	Source   string      `json:"source"`
	Unit     string      `json:"unit,omitempty"`
	Samples  []string    `json:"samples,omitempty"`
	FirstTS  time.Time   `json:"first_ts"`
	LastTS   time.Time   `json:"last_ts"`
}
