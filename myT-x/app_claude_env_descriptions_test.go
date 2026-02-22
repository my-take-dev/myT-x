package main

import (
	"regexp"
	"testing"
)

// TestClaudeEnvVarDescriptionsNonEmpty verifies that claudeEnvVarDescriptions
// is not empty at initialization. This guards against accidental map clearance
// or erroneous var re-declaration that would silently drop all entries.
func TestClaudeEnvVarDescriptionsNonEmpty(t *testing.T) {
	if len(claudeEnvVarDescriptions) == 0 {
		t.Fatal("claudeEnvVarDescriptions is empty; expected non-empty map")
	}
}

// TestClaudeEnvVarDescriptionsValuesNonEmpty verifies that every entry in
// claudeEnvVarDescriptions has a non-empty description string. An empty
// description would cause the settings UI to render a blank tooltip.
func TestClaudeEnvVarDescriptionsValuesNonEmpty(t *testing.T) {
	for key, desc := range claudeEnvVarDescriptions {
		if desc == "" {
			t.Errorf("claudeEnvVarDescriptions[%q] is empty string; all entries must have descriptions", key)
		}
	}
}

// TestClaudeEnvVarDescriptionsKeyFormat verifies that every key in
// claudeEnvVarDescriptions follows UPPER_SNAKE_CASE naming convention
// (starts with an uppercase letter, contains only uppercase letters,
// digits, and underscores). This prevents accidental typos or
// lowercase keys from slipping in.
func TestClaudeEnvVarDescriptionsKeyFormat(t *testing.T) {
	upperSnake := regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)
	for key := range claudeEnvVarDescriptions {
		if !upperSnake.MatchString(key) {
			t.Errorf("key %q does not match UPPER_SNAKE_CASE pattern (^[A-Z][A-Z0-9_]*$)", key)
		}
	}
}
