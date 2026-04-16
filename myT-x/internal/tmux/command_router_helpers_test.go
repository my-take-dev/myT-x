package tmux

import (
	"strings"
	"testing"
)

func TestResolveResizeDimension(t *testing.T) {
	tests := []struct {
		name         string
		value        any
		reference    int
		defaultValue int
		flag         string
		want         int
		wantErr      string // substring match; empty means no error expected
	}{
		{
			name:         "nil returns default without error",
			value:        nil,
			reference:    120,
			defaultValue: 80,
			flag:         "-x",
			want:         80,
		},
		{
			name:         "positive int is returned as-is",
			value:        42,
			reference:    120,
			defaultValue: 80,
			flag:         "-x",
			want:         42,
		},
		{
			name:         "zero int is rejected",
			value:        0,
			reference:    120,
			defaultValue: 80,
			flag:         "-x",
			wantErr:      "must be positive",
		},
		{
			name:         "negative int is rejected",
			value:        -5,
			reference:    120,
			defaultValue: 80,
			flag:         "-x",
			wantErr:      "must be positive",
		},
		{
			name:         "positive float64 is truncated",
			value:        float64(42.9),
			reference:    120,
			defaultValue: 80,
			flag:         "-x",
			want:         42,
		},
		{
			name:         "zero float64 is rejected",
			value:        float64(0),
			reference:    120,
			defaultValue: 80,
			flag:         "-x",
			wantErr:      "must be positive",
		},
		{
			name:         "valid percentage computes correctly",
			value:        "50%",
			reference:    120,
			defaultValue: 80,
			flag:         "-x",
			want:         60,
		},
		{
			name:         "tiny percentage clamps to 1",
			value:        "1%",
			reference:    50,
			defaultValue: 80,
			flag:         "-y",
			want:         1, // 1% of 50 = 0.5 → truncated to 0 → clamped to 1
		},
		{
			name:         "percentage overflow returns error",
			value:        "200%",
			reference:    int(^uint(0) >> 1),
			defaultValue: 80,
			flag:         "-x",
			wantErr:      "overflow",
		},
		{
			name:         "zero percent is rejected",
			value:        "0%",
			reference:    120,
			defaultValue: 80,
			flag:         "-x",
			wantErr:      "expects a positive integer or percentage",
		},
		{
			name:         "negative percent is rejected",
			value:        "-10%",
			reference:    120,
			defaultValue: 80,
			flag:         "-x",
			wantErr:      "expects a positive integer or percentage",
		},
		{
			name:         "empty string is rejected",
			value:        "",
			reference:    120,
			defaultValue: 80,
			flag:         "-x",
			wantErr:      "requires a value",
		},
		{
			name:         "non-numeric string is rejected",
			value:        "abc",
			reference:    120,
			defaultValue: 80,
			flag:         "-x",
			wantErr:      "expects a positive integer or percentage",
		},
		{
			name:         "percentage with zero reference is rejected",
			value:        "50%",
			reference:    0,
			defaultValue: 80,
			flag:         "-x",
			wantErr:      "cannot use percentage without a positive reference",
		},
		{
			name:         "100 percent returns full reference",
			value:        "100%",
			reference:    120,
			defaultValue: 80,
			flag:         "-x",
			want:         120,
		},
		{
			name:         "200 percent doubles reference",
			value:        "200%",
			reference:    60,
			defaultValue: 80,
			flag:         "-x",
			want:         120,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveResizeDimension(tt.value, tt.reference, tt.defaultValue, tt.flag)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %q, want substring %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("resolveResizeDimension(%v, %d, %d, %q) = %d, want %d",
					tt.value, tt.reference, tt.defaultValue, tt.flag, got, tt.want)
			}
		})
	}
}
