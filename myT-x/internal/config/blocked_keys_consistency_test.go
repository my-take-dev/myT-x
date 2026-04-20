package config

import (
	"sort"
	"testing"

	"myT-x/internal/tmux"
)

// TestBlockedKeyListsMatch ensures config.warnOnlyBlockedKeys and
// tmux.blockedEnvironmentKeys remain identical without introducing an import
// cycle in the tmux test package.
func TestBlockedKeyListsMatch(t *testing.T) {
	configKeys := BlockedKeyNames()
	tmuxKeys := tmux.BlockedEnvironmentKeyNames()

	configSorted := sortedKeyNames(configKeys)
	tmuxSorted := sortedKeyNames(tmuxKeys)

	if len(configSorted) != len(tmuxSorted) {
		t.Fatalf("blocked key count mismatch: config=%d (%v), tmux=%d (%v)",
			len(configSorted), configSorted, len(tmuxSorted), tmuxSorted)
	}
	for i := range configSorted {
		if configSorted[i] != tmuxSorted[i] {
			t.Errorf("blocked key mismatch at index %d: config=%q, tmux=%q\nconfig keys: %v\ntmux keys:   %v",
				i, configSorted[i], tmuxSorted[i], configSorted, tmuxSorted)
		}
	}
}

func sortedKeyNames(keys map[string]struct{}) []string {
	names := make([]string, 0, len(keys))
	for key := range keys {
		names = append(names, key)
	}
	sort.Strings(names)
	return names
}
