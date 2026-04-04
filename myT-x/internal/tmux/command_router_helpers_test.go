package tmux

import "testing"

func TestResolveDimension(t *testing.T) {
	tests := []struct {
		name         string
		value        any
		reference    int
		defaultValue int
		want         int
	}{
		{
			name:         "nil returns default",
			value:        nil,
			reference:    120,
			defaultValue: 80,
			want:         80,
		},
		{
			name:         "int value is used directly",
			value:        90,
			reference:    120,
			defaultValue: 80,
			want:         90,
		},
		{
			name:         "float64 value is truncated to int",
			value:        float64(90),
			reference:    120,
			defaultValue: 80,
			want:         90,
		},
		{
			name:         "negative float64 falls back to default",
			value:        float64(-5.5),
			reference:    120,
			defaultValue: 80,
			want:         80,
		},
		{
			name:         "string integer is parsed",
			value:        "100",
			reference:    120,
			defaultValue: 80,
			want:         100,
		},
		{
			name:         "percentage uses reference",
			value:        "30%",
			reference:    120,
			defaultValue: 80,
			want:         36,
		},
		{
			name:         "zero percent falls back to default",
			value:        "0%",
			reference:    120,
			defaultValue: 80,
			want:         80,
		},
		{
			name:         "percent sign only falls back to default",
			value:        "%",
			reference:    120,
			defaultValue: 80,
			want:         80,
		},
		{
			name:         "hundred percent returns full reference",
			value:        "100%",
			reference:    120,
			defaultValue: 80,
			want:         120,
		},
		{
			name:         "percentage over 100 exceeds reference",
			value:        "200%",
			reference:    120,
			defaultValue: 80,
			want:         240,
		},
		{
			name:         "invalid string falls back to default",
			value:        "notint",
			reference:    120,
			defaultValue: 80,
			want:         80,
		},
		{
			name:         "whitespace-only string falls back to default",
			value:        "   ",
			reference:    120,
			defaultValue: 80,
			want:         80,
		},
		{
			name:         "negative absolute value falls back to default",
			value:        "-5",
			reference:    120,
			defaultValue: 80,
			want:         80,
		},
		{
			name:         "overflowing percentage falls back to default",
			value:        "200%",
			reference:    int(^uint(0) >> 1),
			defaultValue: 80,
			want:         80,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveDimension(tt.value, tt.reference, tt.defaultValue); got != tt.want {
				t.Fatalf("resolveDimension(%v, %d, %d) = %d, want %d",
					tt.value, tt.reference, tt.defaultValue, got, tt.want)
			}
		})
	}
}
