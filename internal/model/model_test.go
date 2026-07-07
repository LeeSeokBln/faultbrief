package model

import (
	"encoding/json"
	"testing"
)

func TestSeverityString(t *testing.T) {
	cases := []struct {
		sev  Severity
		want string
	}{
		{SevDebug, "debug"},
		{SevInfo, "info"},
		{SevNotice, "notice"},
		{SevWarning, "warning"},
		{SevError, "error"},
		{SevCritical, "critical"},
		{Severity(99), "unknown"},
	}
	for _, c := range cases {
		if got := c.sev.String(); got != c.want {
			t.Errorf("Severity(%d).String() = %q, want %q", c.sev, got, c.want)
		}
	}
}

func TestParseSeverity(t *testing.T) {
	sev, ok := ParseSeverity("warning")
	if !ok || sev != SevWarning {
		t.Fatalf("ParseSeverity(warning) = %v, %v", sev, ok)
	}
	if _, ok := ParseSeverity("bogus"); ok {
		t.Fatal("ParseSeverity(bogus) should fail")
	}
}

func TestSeverityJSONRoundTrip(t *testing.T) {
	f := Finding{Kind: KindSignature, RuleID: "oom-kill", Severity: SevCritical, Title: "t"}
	b, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got["severity"] != "critical" {
		t.Errorf("severity marshaled as %v, want \"critical\"", got["severity"])
	}
	var back Finding
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back.Severity != SevCritical {
		t.Errorf("round trip severity = %v", back.Severity)
	}
}

func TestSeverityOrdering(t *testing.T) {
	if !(SevCritical > SevError && SevError > SevWarning && SevWarning > SevNotice && SevNotice > SevInfo && SevInfo > SevDebug) {
		t.Fatal("severity ordering broken")
	}
}
