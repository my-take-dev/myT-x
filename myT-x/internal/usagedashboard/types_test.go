package usagedashboard

import (
	"reflect"
	"testing"
)

func TestUsageDashboardStructFieldCounts(t *testing.T) {
	tests := []struct {
		name string
		typ  any
		want int
	}{
		{name: "UsageDashboardSnapshot", typ: UsageDashboardSnapshot{}, want: 4},
		{name: "ClaudeUsageStats", typ: ClaudeUsageStats{}, want: 12},
		{name: "CodexUsageStats", typ: CodexUsageStats{}, want: 10},
		{name: "DailyBucket", typ: DailyBucket{}, want: 4},
		{name: "DailyUsageSeries", typ: DailyUsageSeries{}, want: 4},
		{name: "DailyUsageBucket", typ: DailyUsageBucket{}, want: 2},
		{name: "SourceHealth", typ: SourceHealth{}, want: 5},
		{name: "UsageEntry", typ: UsageEntry{}, want: 3},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := reflect.TypeOf(tc.typ).NumField(); got != tc.want {
				t.Fatalf("NumField() = %d, want %d", got, tc.want)
			}
		})
	}
}
