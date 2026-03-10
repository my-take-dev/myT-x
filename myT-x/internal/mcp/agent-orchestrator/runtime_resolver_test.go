package orchestrator

import (
	"context"
	"errors"
	"testing"
)

type sequenceResolver struct {
	results []resolverResult
	calls   int
}

type resolverResult struct {
	paneID string
	err    error
}

func (r *sequenceResolver) GetPaneID(context.Context) (string, error) {
	if r.calls >= len(r.results) {
		return "", errors.New("unexpected call")
	}
	result := r.results[r.calls]
	r.calls++
	return result.paneID, result.err
}

func TestStickySelfResolverCachesSuccessfulPaneID(t *testing.T) {
	base := &sequenceResolver{
		results: []resolverResult{
			{paneID: "%2"},
			{err: errors.New("should not be reached")},
		},
	}
	resolver := newStickySelfResolver(base, nil)

	first, err := resolver.GetPaneID(context.Background())
	if err != nil {
		t.Fatalf("first GetPaneID: %v", err)
	}
	second, err := resolver.GetPaneID(context.Background())
	if err != nil {
		t.Fatalf("second GetPaneID: %v", err)
	}

	if first != "%2" || second != "%2" {
		t.Fatalf("unexpected pane ids: first=%q second=%q", first, second)
	}
	if base.calls != 1 {
		t.Fatalf("base resolver calls = %d, want 1", base.calls)
	}
}

func TestStickySelfResolverSetPaneIDOverridesBaseLookup(t *testing.T) {
	base := &sequenceResolver{
		results: []resolverResult{
			{err: errors.New("base should not be called")},
		},
	}
	resolver := newStickySelfResolver(base, nil)
	resolver.SetPaneID("%7")

	got, err := resolver.GetPaneID(context.Background())
	if err != nil {
		t.Fatalf("GetPaneID: %v", err)
	}
	if got != "%7" {
		t.Fatalf("pane id = %q, want %%7", got)
	}
	if base.calls != 0 {
		t.Fatalf("base resolver calls = %d, want 0", base.calls)
	}
}
