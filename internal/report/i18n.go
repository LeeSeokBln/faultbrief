package report

// T translates a catalog key; unknown keys fall back to English, then to the
// key itself so nothing ever renders empty.
func T(lang, key string) string {
	if m, ok := catalog[lang]; ok {
		if s, ok := m[key]; ok {
			return s
		}
	}
	if s, ok := catalog["en"][key]; ok {
		return s
	}
	return key
}

// SevName renders a severity in the report language, uppercase for text.
func SevName(lang string, s int) string {
	return T(lang, "sev."+sevKey(s))
}

func sevKey(s int) string {
	switch s {
	case 5:
		return "critical"
	case 4:
		return "error"
	case 3:
		return "warning"
	case 2:
		return "notice"
	case 1:
		return "info"
	default:
		return "debug"
	}
}

var catalog = map[string]map[string]string{
	"en": {
		"title":           "FAULTBRIEF — incident brief",
		"window":          "Window",
		"baseline":        "baseline: previous",
		"sources":         "Sources",
		"records":         "Records",
		"findings":        "Findings",
		"no_findings":     "No findings — logs look healthy in this window.",
		"skipped":         "skipped",
		"parse_failed":    "parse failures",
		"high_parse_fail": "high parse-failure rate (>20%) — check the log format",
		"llm_brief":       "AI Briefing",
		"hint":            "hint",
		"unit":            "unit",
		"first":           "first",
		"last":            "last",
		"kind.signature":  "signature",
		"kind.spike":      "spike",
		"kind.novelty":    "new pattern",
		"sev.critical":    "CRITICAL",
		"sev.error":       "ERROR",
		"sev.warning":     "WARNING",
		"sev.notice":      "NOTICE",
		"sev.info":        "INFO",
		"sev.debug":       "DEBUG",
	},
	"ko": {
		"title":           "FAULTBRIEF — 장애 브리핑",
		"window":          "분석 구간",
		"baseline":        "베이스라인: 직전",
		"sources":         "소스",
		"records":         "레코드",
		"findings":        "발견",
		"no_findings":     "발견 없음 — 이 구간 로그는 정상으로 보입니다.",
		"skipped":         "건너뜀",
		"parse_failed":    "파싱 실패",
		"high_parse_fail": "파싱 실패율 높음(>20%) — 로그 포맷 확인 필요",
		"llm_brief":       "AI 브리핑",
		"hint":            "힌트",
		"unit":            "유닛",
		"first":           "최초",
		"last":            "최종",
		"kind.signature":  "시그니처",
		"kind.spike":      "스파이크",
		"kind.novelty":    "신규 패턴",
		"sev.critical":    "심각",
		"sev.error":       "에러",
		"sev.warning":     "경고",
		"sev.notice":      "주의",
		"sev.info":        "정보",
		"sev.debug":       "디버그",
	},
}
