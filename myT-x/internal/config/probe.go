package config

import (
	"log/slog"

	"go.yaml.in/yaml/v3"
)

// parseRawConfigMetadata unmarshals raw YAML into a generic map used only
// for metadata checks (deprecated fields and missing option detection).
func parseRawConfigMetadata(raw []byte) (map[string]any, error) {
	return parseRawConfigMetadataWith(func(data []byte, out *map[string]any) error {
		return yaml.Unmarshal(data, out)
	}, raw)
}

// parseRawConfigMetadataWith is the parameterized implementation of parseRawConfigMetadata.
// Tests call loadWith, which delegates to this function, to inject a custom
// unmarshalFn that simulates YAML parsing failures.
func parseRawConfigMetadataWith(unmarshalFn func([]byte, *map[string]any) error, raw []byte) (map[string]any, error) {
	var rawMap map[string]any
	if err := unmarshalFn(raw, &rawMap); err != nil {
		return nil, err
	}
	return rawMap, nil
}

type rawWorktreeEnabledProbe struct {
	Worktree *struct {
		Enabled *bool `yaml:"enabled"`
	} `yaml:"worktree"`
}

func probeRawWorktreeEnabled(raw []byte) (bool, error) {
	var probe rawWorktreeEnabledProbe
	if err := yaml.Unmarshal(raw, &probe); err != nil {
		return false, err
	}
	if probe.Worktree == nil {
		return false, nil
	}
	return probe.Worktree.Enabled != nil, nil
}

func resolveWorktreeEnabled(raw []byte, rawMap map[string]any) (bool, error) {
	if rawMap != nil {
		wt, ok := rawMap["worktree"].(map[string]any)
		if !ok {
			return false, nil
		}
		_, hasEnabled := wt["enabled"]
		return hasEnabled, nil
	}
	return probeRawWorktreeEnabled(raw)
}

func warnDeprecatedFields(rawMap map[string]any) {
	wt, ok := rawMap["worktree"].(map[string]any)
	if !ok {
		return
	}
	if _, has := wt["auto_cleanup"]; has {
		slog.Warn("[WARN-CONFIG] deprecated field ignored: worktree.auto_cleanup is no longer used")
	}
}
