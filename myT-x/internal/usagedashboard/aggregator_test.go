package usagedashboard

import (
	"testing"
	"time"
)

func TestDailyAggregatorBuckets(t *testing.T) {
	ref := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)
	agg := NewDailyAggregator(ref, 7)
	buckets := agg.Buckets()
	if len(buckets) != 7 {
		t.Fatalf("expected 7 buckets, got %d", len(buckets))
	}
	if buckets[0].Date != "2026-04-09" {
		t.Errorf("oldest = %q, want 2026-04-09", buckets[0].Date)
	}
	if buckets[6].Date != "2026-04-15" {
		t.Errorf("newest = %q, want 2026-04-15", buckets[6].Date)
	}
}

func TestDailyAggregatorDefaultWindow(t *testing.T) {
	ref := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)
	agg := NewDailyAggregator(ref, 0)
	if len(agg.Buckets()) != DefaultDailyWindow {
		t.Errorf("buckets = %d, want %d", len(agg.Buckets()), DefaultDailyWindow)
	}
}

func TestDailyAggregatorAdd(t *testing.T) {
	ref := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)
	agg := NewDailyAggregator(ref, 3)
	agg.AddSession(ref)
	agg.AddSecondary(ref.AddDate(0, 0, -1))
	agg.AddToolCall(ref.AddDate(0, 0, -2))
	// Outside window — dropped silently.
	agg.AddSession(ref.AddDate(0, 0, -5))

	buckets := agg.Buckets()
	if buckets[0].ToolCalls != 1 {
		t.Errorf("bucket[-2].ToolCalls = %d, want 1", buckets[0].ToolCalls)
	}
	if buckets[1].Secondary != 1 {
		t.Errorf("bucket[-1].Secondary = %d, want 1", buckets[1].Secondary)
	}
	if buckets[2].Sessions != 1 {
		t.Errorf("bucket[0].Sessions = %d, want 1", buckets[2].Sessions)
	}
	if got := agg.ActiveDays(); got != 3 {
		t.Errorf("ActiveDays = %d, want 3", got)
	}
}

func TestDailyAggregatorUTCBoundary(t *testing.T) {
	// Timestamp that is 23:59 UTC but 08:59 next day in UTC+9 should land in
	// the UTC day's bucket, not the local day's.
	ref := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)
	agg := NewDailyAggregator(ref, 2)

	lateUTC := time.Date(2026, 4, 14, 23, 59, 0, 0, time.UTC)
	agg.AddSession(lateUTC)

	buckets := agg.Buckets()
	// Expected window: 2026-04-14, 2026-04-15.
	if buckets[0].Date != "2026-04-14" || buckets[0].Sessions != 1 {
		t.Errorf("UTC boundary mishandled: %+v", buckets)
	}
}

func TestDailyAggregatorUTCWindowEdgesAndOffsets(t *testing.T) {
	ref := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)
	agg := NewDailyAggregator(ref, DefaultDailyWindow)

	oldestIncluded := time.Date(2026, 3, 17, 0, 0, 0, 0, time.UTC)
	justOutside := oldestIncluded.Add(-time.Nanosecond)
	jst := time.FixedZone("JST", 9*60*60)
	pdt := time.FixedZone("PDT", -7*60*60)

	agg.AddSession(oldestIncluded)
	agg.AddSession(justOutside)
	agg.AddSession(time.Date(2026, 4, 16, 0, 30, 0, 0, jst))
	agg.AddSession(time.Date(2026, 4, 14, 17, 30, 0, 0, pdt))

	buckets := agg.Buckets()
	if buckets[0].Date != "2026-03-17" || buckets[0].Sessions != 1 {
		t.Fatalf("oldest bucket = %+v, want 2026-03-17 sessions=1", buckets[0])
	}
	last := buckets[len(buckets)-1]
	if last.Date != "2026-04-15" || last.Sessions != 2 {
		t.Fatalf("newest bucket = %+v, want 2026-04-15 sessions=2", last)
	}
}

func TestDailyAggregatorEmptyIsZero(t *testing.T) {
	ref := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)
	agg := NewDailyAggregator(ref, 3)
	for _, b := range agg.Buckets() {
		if b.Sessions != 0 || b.Secondary != 0 || b.ToolCalls != 0 {
			t.Errorf("empty aggregator has non-zero counts: %+v", b)
		}
	}
	if agg.ActiveDays() != 0 {
		t.Errorf("ActiveDays on empty = %d, want 0", agg.ActiveDays())
	}
}

func TestNamedDailyAggregatorSeries(t *testing.T) {
	ref := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	agg := NewNamedDailyAggregator(ref, 3)
	agg.Add("alpha", ref.AddDate(0, 0, -2))
	agg.Add("alpha", ref.AddDate(0, 0, -1))
	agg.Add("beta", ref)
	agg.Add("outside", ref.AddDate(0, 0, -3))
	agg.Add("", ref)

	series := agg.Series()
	if len(series) != 2 {
		t.Fatalf("len(series) = %d, want 2: %+v", len(series), series)
	}
	if series[0].Name != "alpha" || series[0].TotalCount != 2 {
		t.Fatalf("series[0] = %+v, want alpha total=2", series[0])
	}
	if len(series[0].Buckets) != 3 {
		t.Fatalf("len(alpha buckets) = %d, want 3", len(series[0].Buckets))
	}
	assertDailyUsageBucket(t, series[0].Buckets[0], "2026-04-13", 1)
	assertDailyUsageBucket(t, series[0].Buckets[1], "2026-04-14", 1)
	assertDailyUsageBucket(t, series[0].Buckets[2], "2026-04-15", 0)
	if series[1].Name != "beta" || series[1].TotalCount != 1 {
		t.Fatalf("series[1] = %+v, want beta total=1", series[1])
	}
}

func TestNamedDailyAggregatorTopNSortsLikeRanking(t *testing.T) {
	ref := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	agg := NewNamedDailyAggregator(ref, 3)
	agg.Add("bravo", ref.Add(-3*time.Hour))
	agg.Add("alpha", ref.Add(-3*time.Hour))
	agg.Add("charlie", ref.Add(-4*time.Hour))
	agg.Add("delta", ref.Add(-2*time.Hour))
	agg.Add("delta", ref.Add(-1*time.Hour))

	top := agg.TopN(4)
	if len(top) != 4 {
		t.Fatalf("len(top) = %d, want 4: %+v", len(top), top)
	}
	wantNames := []string{"delta", "alpha", "bravo", "charlie"}
	for i, want := range wantNames {
		if top[i].Name != want {
			t.Fatalf("top[%d].Name = %q, want %q: %+v", i, top[i].Name, want, top)
		}
	}
	if top[0].Count != 2 {
		t.Fatalf("delta count = %d, want 2", top[0].Count)
	}
}

func TestNamedDailyAggregatorTopNMatchesSeriesTotals(t *testing.T) {
	ref := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	agg := NewNamedDailyAggregator(ref, 3)
	agg.Add("alpha", ref)
	agg.Add("alpha", ref)
	agg.Add("beta", ref.AddDate(0, 0, -1))

	top := agg.TopN(10)
	series := agg.Series()
	if len(top) != len(series) {
		t.Fatalf("len(top) = %d, len(series) = %d", len(top), len(series))
	}
	for i := range top {
		if top[i].Name != series[i].Name {
			t.Fatalf("name mismatch at %d: top=%+v series=%+v", i, top[i], series[i])
		}
		if top[i].Count != series[i].TotalCount {
			t.Fatalf("count mismatch for %s: ranking=%d series=%d", top[i].Name, top[i].Count, series[i].TotalCount)
		}
		if !top[i].LastUsedAt.Equal(series[i].LastUsedAt) {
			t.Fatalf("last-used mismatch for %s: ranking=%v series=%v", top[i].Name, top[i].LastUsedAt, series[i].LastUsedAt)
		}
	}
}

func TestNamedDailyAggregatorTopSeriesMatchesTopNUniverse(t *testing.T) {
	ref := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	agg := NewNamedDailyAggregator(ref, 3)
	agg.Add("alpha", ref.Add(-3*time.Hour))
	agg.Add("bravo", ref.Add(-2*time.Hour))
	agg.Add("charlie", ref.Add(-1*time.Hour))

	top := agg.TopN(2)
	series := agg.TopSeries(2)
	if len(top) != 2 || len(series) != 2 {
		t.Fatalf("len(top)=%d len(series)=%d, want both 2", len(top), len(series))
	}
	for i := range top {
		if top[i].Name != series[i].Name {
			t.Fatalf("series[%d].Name = %q, want ranked item %q", i, series[i].Name, top[i].Name)
		}
	}
	for _, item := range series {
		if item.Name == "alpha" {
			t.Fatalf("TopSeries included item outside the top 2: %+v", series)
		}
	}
}

func TestNamedDailyAggregatorEmptyIsNonNil(t *testing.T) {
	ref := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	agg := NewNamedDailyAggregator(ref, DefaultDailyWindow)

	if got := agg.TopN(10); got == nil || len(got) != 0 {
		t.Fatalf("TopN() = %+v, want non-nil empty slice", got)
	}
	if got := agg.Series(); got == nil || len(got) != 0 {
		t.Fatalf("Series() = %+v, want non-nil empty slice", got)
	}
}

func assertDailyUsageBucket(t *testing.T, got DailyUsageBucket, wantDate string, wantCount int) {
	t.Helper()
	if got.Date != wantDate || got.Count != wantCount {
		t.Fatalf("bucket = %+v, want date=%s count=%d", got, wantDate, wantCount)
	}
}
