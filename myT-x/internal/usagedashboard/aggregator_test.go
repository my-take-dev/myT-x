package usagedashboard

import (
	"testing"
	"time"
)

func TestUsageCounterTopN(t *testing.T) {
	c := NewUsageCounter()
	now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	c.Add("alpha", now.Add(-time.Hour))
	c.Add("alpha", now)
	c.Add("beta", now.Add(-2*time.Hour))
	c.Add("gamma", now)
	c.Add("gamma", now)
	c.Add("gamma", now.Add(time.Hour))
	c.Add("", now) // empty name is ignored

	top := c.TopN(10)
	if len(top) != 3 {
		t.Fatalf("expected 3 entries, got %d: %+v", len(top), top)
	}
	if top[0].Name != "gamma" || top[0].Count != 3 {
		t.Errorf("rank 1 = %+v, want gamma×3", top[0])
	}
	if top[1].Name != "alpha" || top[1].Count != 2 {
		t.Errorf("rank 2 = %+v, want alpha×2", top[1])
	}
	if top[2].Name != "beta" || top[2].Count != 1 {
		t.Errorf("rank 3 = %+v, want beta×1", top[2])
	}
	if !top[0].LastUsedAt.Equal(now.Add(time.Hour)) {
		t.Errorf("gamma.LastUsedAt = %v, want %v", top[0].LastUsedAt, now.Add(time.Hour))
	}
}

func TestUsageCounterTopNLimit(t *testing.T) {
	c := NewUsageCounter()
	for range 5 {
		c.Add("same-count", time.Time{})
	}
	c.Add("other", time.Time{})

	top := c.TopN(1)
	if len(top) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(top))
	}
	if top[0].Name != "same-count" {
		t.Errorf("rank 1 = %q, want same-count", top[0].Name)
	}
}

func TestUsageCounterTopNZeroLimitFallsBackToDefault(t *testing.T) {
	c := NewUsageCounter()
	for i := range TopRankingLimit + 3 {
		c.Add(string(rune('a'+i)), time.Time{})
	}

	top := c.TopN(0)
	if len(top) != TopRankingLimit {
		t.Fatalf("len(top) = %d, want %d", len(top), TopRankingLimit)
	}
}

func TestUsageCounterTopNTieBreakByName(t *testing.T) {
	c := NewUsageCounter()
	ts := time.Now()
	c.Add("banana", ts)
	c.Add("apple", ts)
	c.Add("cherry", ts)

	top := c.TopN(10)
	if len(top) != 3 {
		t.Fatalf("got %d entries", len(top))
	}
	if top[0].Name != "apple" || top[1].Name != "banana" || top[2].Name != "cherry" {
		t.Errorf("alphabetic tie break failed: %+v", top)
	}
}

func TestUsageCounterEmpty(t *testing.T) {
	c := NewUsageCounter()
	got := c.TopN(5)
	if got == nil {
		t.Error("TopN on empty counter returned nil; want non-nil slice")
	}
	if len(got) != 0 {
		t.Errorf("len = %d, want 0", len(got))
	}
}

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
