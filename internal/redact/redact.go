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

	// Token shapes that carry no key= prefix on a command line.
	reTokens = []*regexp.Regexp{
		regexp.MustCompile(`\b(?:ghp|gho|ghu|ghs|ghr)_[A-Za-z0-9]{20,}`),
		regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{20,}`),
		regexp.MustCompile(`\bsk-(?:ant-|proj-)?[A-Za-z0-9_-]{20,}`),
		regexp.MustCompile(`\bxox[abposr]-[A-Za-z0-9-]{10,}`),
		regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}`),
		regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`),
	}
	// Password in a URL: scheme://user:password@host keeps the user.
	reURLPass = regexp.MustCompile(`(\b[a-zA-Z][a-zA-Z0-9+.-]*://[^/\s:@]+:)[^/\s@]+@`)
	// mysql-style -pSECRET, attached to the flag. Only after a mysql client name.
	reMySQL = regexp.MustCompile(`(\b(?:mysql|mysqldump|mysqladmin|mariadb)\b.*?\s-p)([^\s]+)`)
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
	for _, re := range reTokens {
		s = re.ReplaceAllString(s, Mark)
	}
	s = reURLPass.ReplaceAllString(s, "${1}"+Mark+"@")
	s = reMySQL.ReplaceAllString(s, "${1}"+Mark)
	return s
}
