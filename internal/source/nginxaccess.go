package source

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/LeeSeokBln/faultbrief/internal/model"
)

// NginxAccess parses nginx access logs in the default "combined" format.
type NginxAccess struct {
	files fileLines
}

func NewNginxAccess(paths []string) *NginxAccess {
	return &NginxAccess{files: fileLines{paths: paths}}
}

func (n *NginxAccess) Name() string { return "nginx-access" }

// combined: $remote_addr - $remote_user [$time_local] "$request" $status $bytes "$referer" "$ua"
var reAccess = regexp.MustCompile(`^(\S+) \S+ (\S+) \[([^\]]+)\] "([^"]*)" (\d{3}) (\d+|-)`)

const accessTimeLayout = "02/Jan/2006:15:04:05 -0700"

func parseAccessLine(line string) (model.LogRecord, error) {
	m := reAccess.FindStringSubmatch(line)
	if m == nil {
		return model.LogRecord{}, errUnparsable
	}
	ts, err := time.Parse(accessTimeLayout, m[3])
	if err != nil {
		return model.LogRecord{}, errUnparsable
	}
	status, _ := strconv.Atoi(m[5])
	method, path := splitRequest(m[4])
	sev := model.SevInfo
	switch {
	case status >= 500:
		sev = model.SevError
	case status >= 400:
		sev = model.SevNotice
	}
	msg := fmt.Sprintf("%s %s [%dxx]", method, path, status/100)
	return model.LogRecord{
		TS: ts, Source: "nginx-access", Unit: "nginx", Severity: sev, Message: msg,
		Fields: map[string]string{
			"status": m[5],
			"method": method,
			"path":   path,
			"remote": m[1],
		},
	}, nil
}

// splitRequest splits `GET /path HTTP/1.1`; garbage request lines become
// method "-" with the raw text as path so they still fingerprint together.
func splitRequest(req string) (method, path string) {
	parts := strings.SplitN(req, " ", 3)
	if len(parts) >= 2 && !strings.ContainsAny(parts[0], `\/`) {
		return parts[0], parts[1]
	}
	return "-", "(malformed)"
}

func (n *NginxAccess) Collect(ctx context.Context, from, to time.Time, emit func(model.LogRecord)) (Stats, error) {
	return n.files.collect(ctx, from, to, emit, parseAccessLine)
}
