package usagedashboard

import (
	"sort"
	"strings"
	"time"
)

type dailyUsageAccumulator struct {
	name       string
	totalCount int
	lastUsedAt time.Time
	buckets    map[string]*DailyUsageBucket
}

// NamedDailyAggregator builds per-name usage series for the same rolling
// UTC window used by DailyAggregator.
type NamedDailyAggregator struct {
	buckets map[string]struct{}
	order   []string
	series  map[string]*dailyUsageAccumulator
}

// NewNamedDailyAggregator pre-allocates a rolling date window anchored at
// referenceDay (UTC). Days are listed from oldest to newest.
func NewNamedDailyAggregator(referenceDay time.Time, windowDays int) *NamedDailyAggregator {
	if windowDays <= 0 {
		windowDays = DefaultDailyWindow
	}
	ref := referenceDay.UTC()
	ref = time.Date(ref.Year(), ref.Month(), ref.Day(), 0, 0, 0, 0, time.UTC)
	agg := &NamedDailyAggregator{
		buckets: make(map[string]struct{}, windowDays),
		order:   make([]string, 0, windowDays),
		series:  make(map[string]*dailyUsageAccumulator),
	}
	for i := windowDays - 1; i >= 0; i-- {
		day := ref.AddDate(0, 0, -i)
		key := day.Format("2006-01-02")
		agg.order = append(agg.order, key)
		agg.buckets[key] = struct{}{}
	}
	return agg
}

// Add increments name for the UTC day of ts. Both the rolling window and
// event timestamps are normalized to UTC date keys so local-time records land
// in the same boundary buckets. Empty names and timestamps outside the rolling
// window are ignored.
func (a *NamedDailyAggregator) Add(name string, ts time.Time) {
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	key := ts.UTC().Format("2006-01-02")
	if _, ok := a.buckets[key]; !ok {
		return
	}
	acc, ok := a.series[name]
	if !ok {
		acc = &dailyUsageAccumulator{
			name:    name,
			buckets: make(map[string]*DailyUsageBucket, len(a.order)),
		}
		for _, day := range a.order {
			acc.buckets[day] = &DailyUsageBucket{Date: day}
		}
		a.series[name] = acc
	}
	acc.totalCount++
	if ts.After(acc.lastUsedAt) {
		acc.lastUsedAt = ts
	}
	bucket := acc.buckets[key]
	if bucket == nil {
		return
	}
	bucket.Count++
}

// TopN returns ranking rows sorted by total count, last-used timestamp, then
// name. Counts are limited to events inside the rolling window.
func (a *NamedDailyAggregator) TopN(limit int) []UsageEntry {
	series := a.sortedSeries()
	if limit <= 0 {
		limit = TopRankingLimit
	}
	if len(series) > limit {
		series = series[:limit]
	}
	out := make([]UsageEntry, 0, len(series))
	for _, acc := range series {
		out = append(out, UsageEntry{
			Name:       acc.name,
			Count:      acc.totalCount,
			LastUsedAt: acc.lastUsedAt,
		})
	}
	return out
}

// Series returns all named daily series in ranking order.
func (a *NamedDailyAggregator) Series() []DailyUsageSeries {
	return a.seriesFromAccumulators(a.sortedSeries())
}

// TopSeries returns daily series capped to the same ranked item universe as
// TopN. Use this for API payloads where the ranking table and daily chart must
// describe the same set of items.
func (a *NamedDailyAggregator) TopSeries(limit int) []DailyUsageSeries {
	series := a.sortedSeries()
	if limit <= 0 {
		limit = TopRankingLimit
	}
	if len(series) > limit {
		series = series[:limit]
	}
	return a.seriesFromAccumulators(series)
}

func (a *NamedDailyAggregator) seriesFromAccumulators(series []*dailyUsageAccumulator) []DailyUsageSeries {
	out := make([]DailyUsageSeries, 0, len(series))
	for _, acc := range series {
		buckets := make([]DailyUsageBucket, 0, len(a.order))
		for _, key := range a.order {
			buckets = append(buckets, *acc.buckets[key])
		}
		out = append(out, DailyUsageSeries{
			Name:       acc.name,
			TotalCount: acc.totalCount,
			LastUsedAt: acc.lastUsedAt,
			Buckets:    buckets,
		})
	}
	return out
}

func (a *NamedDailyAggregator) sortedSeries() []*dailyUsageAccumulator {
	out := make([]*dailyUsageAccumulator, 0, len(a.series))
	for _, acc := range a.series {
		out = append(out, acc)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].totalCount != out[j].totalCount {
			return out[i].totalCount > out[j].totalCount
		}
		if !out[i].lastUsedAt.Equal(out[j].lastUsedAt) {
			return out[i].lastUsedAt.After(out[j].lastUsedAt)
		}
		return out[i].name < out[j].name
	})
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
