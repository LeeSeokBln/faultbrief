//go:build ignore

// Generates the e2e fixtures. Run from repo root:
//
//	go run internal/app/testdata/e2e/gen.go
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func main() {
	dir := "internal/app/testdata/e2e"
	now := time.Date(2026, 7, 7, 10, 0, 0, 0, time.UTC)
	aFrom := now.Add(-time.Hour)
	bFrom := aFrom.Add(-24 * time.Hour)

	var syslog []string
	sys := func(ts time.Time, tag, msg string) {
		syslog = append(syslog, fmt.Sprintf("%s web1 %s: %s", ts.Format("Jan _2 15:04:05"), tag, msg))
	}
	// healthy noise both windows
	for i := 0; i < 24; i++ {
		sys(bFrom.Add(time.Duration(i)*time.Hour), "app", "heartbeat ok")
	}
	for i := 0; i < 4; i++ {
		sys(aFrom.Add(time.Duration(i)*15*time.Minute), "app", "heartbeat ok")
	}
	// spike template: 5 in baseline spread, 30 in analysis
	for i := 0; i < 5; i++ {
		sys(bFrom.Add(time.Duration(i*5)*time.Hour), "myapp[77]", "error: pg query timeout after 5000 ms")
	}
	for i := 0; i < 30; i++ {
		sys(aFrom.Add(time.Duration(i*2)*time.Minute), "myapp[77]", "error: pg query timeout after 5000 ms")
	}
	// signature: OOM ×2 in analysis
	sys(aFrom.Add(12*time.Minute), "kernel", "Out of memory: Killed process 1234 (myapp) total-vm:204800kB")
	sys(aFrom.Add(14*time.Minute), "kernel", "Out of memory: Killed process 1301 (myapp) total-vm:204800kB")
	// novelty: ×4 analysis only
	for i := 0; i < 4; i++ {
		sys(aFrom.Add(time.Duration(30+i)*time.Minute), "myapp[77]", "certificate verify failed for backend api.internal")
	}
	writeLines(filepath.Join(dir, "syslog.log"), syslog)

	var access []string
	acc := func(ts time.Time, status int, path string) {
		access = append(access, fmt.Sprintf(`10.0.0.5 - - [%s] "GET %s HTTP/1.1" %d 123 "-" "curl/8"`,
			ts.Format("02/Jan/2006:15:04:05 -0700"), path, status))
	}
	for i := 0; i < 200; i++ { // baseline traffic, all healthy
		acc(bFrom.Add(time.Duration(i*7)*time.Minute), 200, "/api/list")
	}
	for i := 0; i < 60; i++ { // analysis: 48 ok, 12 5xx
		status := 200
		if i%5 == 0 {
			status = 502
		}
		acc(aFrom.Add(time.Duration(i)*time.Minute), status, "/api/list")
	}
	writeLines(filepath.Join(dir, "access.log"), access)

	var errlog []string
	for i := 0; i < 3; i++ {
		ts := aFrom.Add(time.Duration(20+i) * time.Minute)
		errlog = append(errlog, fmt.Sprintf("%s [error] 88#0: *%d upstream timed out (110: Connection timed out) while reading response header from upstream",
			ts.Format("2006/01/02 15:04:05"), 100+i))
	}
	writeLines(filepath.Join(dir, "error.log"), errlog)

	var journal []string
	jts := aFrom.Add(40 * time.Minute)
	journal = append(journal, fmt.Sprintf(
		`{"__REALTIME_TIMESTAMP":"%d","PRIORITY":"3","MESSAGE":"myapp.service: Failed with result 'exit-code'.","_SYSTEMD_UNIT":"myapp.service"}`,
		jts.UnixMicro()))
	writeLines(filepath.Join(dir, "journal.ndjson"), journal)
}

func writeLines(path string, lines []string) {
	out := ""
	for _, l := range lines {
		out += l + "\n"
	}
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		panic(err)
	}
}
