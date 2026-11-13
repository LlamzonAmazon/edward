// Package skill embeds the EDwaRd agent skill so the binary carries the only copy.
package skill

import (
	_ "embed"
	"strings"
)

//go:embed SKILL.md
var template string

// Body returns SKILL.md with {version} replaced.
func Body(version string) string {
	return strings.ReplaceAll(template, "{version}", version)
}
