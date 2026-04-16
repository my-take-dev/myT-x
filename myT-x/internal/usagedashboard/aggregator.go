package usagedashboard

import (
	"sort"
	"time"
)

// UsageCounter accumulates counts and last-used timestamps for a single key
// dimension (e.g., skill name, agent type).
type UsageCounter struct {
	counts map[string]*UsageEntry
}

// NewUsageCounter returns an empty counter.
func NewUsageCounter() *UsageCounter {
	return &UsageCounter{counts: make(map[string]*UsageEntry)}
}

// Add increments the count for name and updates LastUsedAt if ts is newer.
// Empty names are ignored.
func (c *UsageCounter) Add(name string, ts time.Time) {
	if name == "" {
		return
	}
	entry, ok := c.counts[name]
	if !ok {
		entry = &UsageEntry{Name: name}
		c.counts[name] = entry
	}
	entry.Count++
	if ts.After(entry.LastUsedAt) {
		entry.LastUsedAt = ts
	}
}

// TopN returns up to limit entries sorted by Count desc, LastUsedAt desc,
// then Name asc. Returns a non-nil empty slice when the counter is empty.
func (c *UsageCounter) TopN(limit int) []UsageEntry {
	if limit <= 0 {
		limit = TopRankingLimit
	}
	out := make([]UsageEntry, 0, len(c.counts))
	for _, entry := range c.counts {
		out = append(out, *entry)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		if !out[i].LastUsedAt.Equal(out[j].LastUsedAt) {
			return out[i].LastUsedAt.After(out[j].LastUsedAt)
		}
		return out[i].Name < out[j].Name
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

// DailyAggregator builds a rolling N-day activity window anchored at the
// reference day. Dates are formatted as "YYYY-MM-DD" in UTC.
type DailyAggregator struct {
	buckets map[string]*DailyBucket
	order   []string
}

// NewDailyAggregator pre-allocates windowDays buckets anchored at referenceDay
// (UTC). Days are listed from oldest to newest.
func NewDailyAggregator(referenceDay time.Time, windowDays int) *DailyAggregator {
	if windowDays <= 0 {
		windowDays = DefaultDailyWindow
	}
	ref := referenceDay.UTC()
	ref = time.Date(ref.Year(), ref.Month(), ref.Day(), 0, 0, 0, 0, time.UTC)
	agg := &DailyAggregator{
		buckets: make(map[string]*DailyBucket, windowDays),
		order:   make([]string, 0, windowDays),
	}
	for i := windowDays - 1; i >= 0; i-- {
		day := ref.AddDate(0, 0, -i)
		key := day.Format("2006-01-02")
		agg.order = append(agg.order, key)
		agg.buckets[key] = &DailyBucket{Date: key}
	}
	return agg
}

// AddSession increments the sessions column for the UTC day of ts.
// Timestamps outside the window are silently dropped.
func (a *DailyAggregator) AddSession(ts time.Time) {
	if bucket := a.bucketFor(ts); bucket != nil {
		bucket.Sessions++
	}
}

// AddSecondary increments the "secondary" column (Messages for Claude,
// Prompts for Codex) for the UTC day of ts.
func (a *DailyAggregator) AddSecondary(ts time.Time) {
	if bucket := a.bucketFor(ts); bucket != nil {
		bucket.Secondary++
	}
}

// AddToolCall increments the "tool calls" column (ToolCalls for Claude,
// SpawnedAgents for Codex) for the UTC day of ts.
func (a *DailyAggregator) AddToolCall(ts time.Time) {
	if bucket := a.bucketFor(ts); bucket != nil {
		bucket.ToolCalls++
	}
}

func (a *DailyAggregator) bucketFor(ts time.Time) *DailyBucket {
	key := ts.UTC().Format("2006-01-02")
	return a.buckets[key]
}

// Buckets returns the buckets in chronological order (oldest first).
// The returned slice is freshly allocated and independent of internal state.
func (a *DailyAggregator) Buckets() []DailyBucket {
	out := make([]DailyBucket, 0, len(a.order))
	for _, key := range a.order {
		if b, ok := a.buckets[key]; ok {
			out = append(out, *b)
		}
	}
	return out
}

// ActiveDays returns the number of days with any activity in the window.
func (a *DailyAggregator) ActiveDays() int {
	n := 0
	for _, b := range a.buckets {
		if b.Sessions > 0 || b.Secondary > 0 || b.ToolCalls > 0 {
			n++
		}
	}
	return n
}
