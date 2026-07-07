package source

import (
	"context"
	"regexp"
	"time"

	"github.com/LeeSeokBln/faultbrief/internal/model"
)

// NginxError parses nginx error logs:
// 2026/07/07 08:31:02 [error] 1234#0: *567 message...
type NginxError struct {
	files fileLines
}

func NewNginxError(paths []string) *NginxError {
	return &NginxError{files: fileLines{paths: paths}}
}

func (n *NginxError) Name() string { return "nginx-error" }

var reNginxError = regexp.MustCompile(`^(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}) \[(\w+)\] \d+#\d+: (?:\*\d+ )?(.*)$`)

const nginxErrorTimeLayout = "2006/01/02 15:04:05"

func parseErrorLine(line string) (model.LogRecord, error) {
	m := reNginxError.FindStringSubmatch(line)
	if m == nil {
		return model.LogRecord{}, errUnparsable
	}
	ts, err := time.ParseInLocation(nginxErrorTimeLayout, m[1], time.Local)
	if err != nil {
		return model.LogRecord{}, errUnparsable
	}
	return model.LogRecord{
		TS: ts, Source: "nginx-error", Unit: "nginx",
		Severity: mapNginxLevel(m[2]), Message: m[3],
	}, nil
}

func mapNginxLevel(lvl string) model.Severity {
	switch lvl {
	case "emerg", "alert", "crit":
		return model.SevCritical
	case "error":
		return model.SevError
	case "warn":
		return model.SevWarning
	case "notice":
		return model.SevNotice
	case "debug":
		return model.SevDebug
	default:
		return model.SevInfo
	}
}

func (n *NginxError) Collect(ctx context.Context, from, to time.Time, emit func(model.LogRecord)) (Stats, error) {
	return n.files.collect(ctx, from, to, emit, parseErrorLine)
}
