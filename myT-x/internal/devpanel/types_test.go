package devpanel

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestFileEntryMarshalIncludesFalseBooleans(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		entry FileEntry
	}{
		{
			name: "root directory",
			entry: FileEntry{
				Name:          "empty",
				Path:          "empty",
				IsDir:         true,
				Size:          0,
				HasChildren:   false,
				HasViewTarget: false,
			},
		},
		{
			name: "nested directory",
			entry: FileEntry{
				Name:          "nested",
				Path:          "src/nested",
				IsDir:         true,
				Size:          0,
				HasChildren:   false,
				HasViewTarget: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload, err := json.Marshal(tt.entry)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}
			if string(payload) == "" {
				t.Fatal("marshal returned empty payload")
			}
			if !strings.Contains(string(payload), `"has_children":false`) {
				t.Fatalf("expected has_children=false in payload, got %s", payload)
			}
			if !strings.Contains(string(payload), `"has_view_target":false`) {
				t.Fatalf("expected has_view_target=false in payload, got %s", payload)
			}
		})
	}
}
