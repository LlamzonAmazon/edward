package redact

import (
	"regexp"
	"strings"
)

const Mark = "‹redacted›"

var (
	reBearer = regexp.MustCompile(`(?i)(bearer\s+)\S+`)
	reKey    = regexp.MustCompile(`(?i)((?:api[_-]?key|token|secret|password|passwd|authorization)[=:\s]+)\S+`)
	reAKIA   = regexp.MustCompile(`AKIA[0-9A-Z]{16}`)
)

func String(s string) string {
	s = reBearer.ReplaceAllString(s, "${1}"+Mark)
	s = reKey.ReplaceAllStringFunc(s, func(m string) string {
		sub := reKey.FindStringSubmatch(m)
		if len(sub) < 2 {
			return Mark
		}
		val := strings.TrimPrefix(m, sub[1])
		low := strings.ToLower(val)
		if low == strings.ToLower(Mark) || strings.HasPrefix(low, "bearer") || strings.Contains(val, Mark) {
			return m
		}
		return sub[1] + Mark
	})
	s = reAKIA.ReplaceAllString(s, Mark)
	return s
}
