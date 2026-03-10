package mcp

import (
	"context"
	"testing"
)

func TestNewRegistryRejectsDuplicateToolNames(t *testing.T) {
	_, err := NewRegistry([]Tool{
		{Name: "dup", Handler: func(context.Context, map[string]any) (any, error) { return nil, nil }},
		{Name: "dup", Handler: func(context.Context, map[string]any) (any, error) { return nil, nil }},
	})
	if err == nil {
		t.Fatal("expected duplicate tool error")
	}
}
