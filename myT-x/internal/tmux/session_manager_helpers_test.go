package tmux

import "testing"

func TestParseWindowIDTarget(t *testing.T) {
	tests := []struct {
		name           string
		target         string
		wantID         int
		wantIsWindowID bool
		wantErr        string
	}{
		{
			name:           "valid zero",
			target:         "@0",
			wantID:         0,
			wantIsWindowID: true,
		},
		{
			name:           "valid positive",
			target:         "@5",
			wantID:         5,
			wantIsWindowID: true,
		},
		{
			name:           "trimmed target",
			target:         "  @7  ",
			wantID:         7,
			wantIsWindowID: true,
		},
		{
			name:           "empty suffix",
			target:         "@",
			wantID:         -1,
			wantIsWindowID: true,
			wantErr:        "invalid window id: @",
		},
		{
			name:           "non numeric",
			target:         "@abc",
			wantID:         -1,
			wantIsWindowID: true,
			wantErr:        "invalid window id: @abc",
		},
		{
			name:           "negative",
			target:         "@-1",
			wantID:         -1,
			wantIsWindowID: true,
			wantErr:        "invalid window id: @-1",
		},
		{
			name:           "session name",
			target:         "demo",
			wantID:         -1,
			wantIsWindowID: false,
		},
		{
			name:           "pane target",
			target:         "%0",
			wantID:         -1,
			wantIsWindowID: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotID, gotIsWindowID, err := parseWindowIDTarget(tt.target)
			if gotID != tt.wantID {
				t.Fatalf("parseWindowIDTarget(%q) id = %d, want %d", tt.target, gotID, tt.wantID)
			}
			if gotIsWindowID != tt.wantIsWindowID {
				t.Fatalf("parseWindowIDTarget(%q) isWindowID = %v, want %v", tt.target, gotIsWindowID, tt.wantIsWindowID)
			}
			switch {
			case tt.wantErr == "" && err != nil:
				t.Fatalf("parseWindowIDTarget(%q) error = %v, want nil", tt.target, err)
			case tt.wantErr != "" && err == nil:
				t.Fatalf("parseWindowIDTarget(%q) error = nil, want %q", tt.target, tt.wantErr)
			case tt.wantErr != "" && err.Error() != tt.wantErr:
				t.Fatalf("parseWindowIDTarget(%q) error = %q, want %q", tt.target, err.Error(), tt.wantErr)
			}
		})
	}
}
