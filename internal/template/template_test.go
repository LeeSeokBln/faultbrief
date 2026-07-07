package template

import (
	"strings"
	"testing"
)

func TestMask(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"numbers", "retry attempt 3 of 5", "retry attempt <NUM> of <NUM>"},
		{"ipv4 with port", "connect to 10.0.3.7:5432 failed", "connect to <IP> failed"},
		{"ipv6", "peer fe80::1ff:fe23:4567:890a dropped", "peer <IP> dropped"},
		{"time not ipv6", "at 15:04:05 job ran", "at <NUM>:<NUM>:<NUM> job ran"},
		{"uuid", "req 6ba7b810-9dad-11d1-80b4-00c04fd430c8 done", "req <UUID> done"},
		{"hex", "block deadbeefcafe corrupted", "block <HEX> corrupted"},
		{"long digits become hex-class", "id 123456789 assigned", "id <HEX> assigned"},
		{"path", "open /var/log/nginx/error.log failed", "open <PATH> failed"},
		{"quoted", `invalid value "foo bar" rejected`, "invalid value <STR> rejected"},
		{"single quoted", "unit 'my-app.service' entered failed state", "unit <STR> entered failed state"},
		{"status class survives", "GET /api/users/123 [5xx]", "GET <PATH> [5xx]"},
		{"korean text unchanged", "사용자 3명 접속 실패", "사용자 <NUM>명 접속 실패"},
		{"empty", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Mask(c.in); got != c.want {
				t.Errorf("Mask(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestMaskStability(t *testing.T) {
	in := "ERROR: connection to 10.0.3.7:5432 timed out (attempt 3)"
	if Mask(in) != Mask(in) {
		t.Fatal("Mask must be deterministic")
	}
}

func TestFingerprint(t *testing.T) {
	a := Fingerprint("syslog", "connection to <IP> timed out (attempt <NUM>)")
	b := Fingerprint("syslog", "connection to <IP> timed out (attempt <NUM>)")
	c := Fingerprint("nginx-error", "connection to <IP> timed out (attempt <NUM>)")
	if a != b {
		t.Error("same input must produce same fingerprint")
	}
	if a == c {
		t.Error("different sources must produce different fingerprints")
	}
	if len(a) != 16 || strings.ToLower(a) != a {
		t.Errorf("fingerprint should be 16 lowercase hex chars, got %q", a)
	}
}
