package domain

import "testing"

func TestIsVirtualPaneID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		paneID string
		want   bool
	}{
		{"virtual task-master", "%virtual-task-master", true},
		{"virtual other", "%virtual-scheduler", true},
		{"prefix only", "%virtual-", true},
		{"real pane", "%0", false},
		{"real pane multi-digit", "%123", false},
		{"empty string", "", false},
		{"similar but no prefix", "virtual-task-master", false},
		{"percent but not virtual", "%not-virtual", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := IsVirtualPaneID(tt.paneID)
			if got != tt.want {
				t.Errorf("IsVirtualPaneID(%q) = %v, want %v", tt.paneID, got, tt.want)
			}
		})
	}
}

func TestVirtualPaneIDPrefix(t *testing.T) {
	t.Parallel()
	if VirtualPaneIDPrefix != "%virtual-" {
		t.Errorf("VirtualPaneIDPrefix = %q, want %q", VirtualPaneIDPrefix, "%virtual-")
	}
}

func TestValidatePaneID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		paneID  string
		wantErr bool
	}{
		{"valid real pane", "%0", false},
		{"valid multi-digit", "%123", false},
		{"virtual task-master", "%virtual-task-master", false},
		{"virtual scheduler", "%virtual-scheduler", false},
		{"empty", "", true},
		{"no percent prefix", "0", true},
		{"invalid format", "%abc", true},
		{"similar but not virtual", "%not-virtual", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidatePaneID(tt.paneID)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePaneID(%q) error = %v, wantErr %v", tt.paneID, err, tt.wantErr)
			}
		})
	}
}
