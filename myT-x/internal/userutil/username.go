package userutil

import (
	"regexp"
	"strings"
)

var invalidUsernameRune = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

// SanitizeUsername normalizes username-like values used in pipe/mutex names.
func SanitizeUsername(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	return invalidUsernameRune.ReplaceAllString(value, "_")
}
