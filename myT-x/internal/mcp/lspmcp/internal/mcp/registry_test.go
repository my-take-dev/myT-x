package mcp

import (
	"context"
	"testing"
)

func TestRegistryGet(t *testing.T) {
	reg := NewRegistry([]Tool{
		{Name: "tool_a", Description: "A", Handler: func(ctx context.Context, args map[string]any) (any, error) { return "a", nil }},
		{Name: "tool_b", Description: "B", Handler: func(ctx context.Context, args map[string]any) (any, error) { return "b", nil }},
	})

	tool, ok := reg.Get("tool_a")
	if !ok {
		t.Fatal("Get(tool_a) should find tool")
	}
	if tool.Name != "tool_a" {
		t.Errorf("unexpected tool name: %q", tool.Name)
	}

	_, ok = reg.Get("nonexistent")
	if ok {
		t.Error("Get(nonexistent) should return false")
	}
}

func TestRegistryList(t *testing.T) {
	reg := NewRegistry([]Tool{
		{Name: "z_tool", Description: "Z"},
		{Name: "a_tool", Description: "A"},
		{Name: "m_tool", Description: "M"},
	})

	tools := reg.List()
	if len(tools) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(tools))
	}
	// List は名前でソートされる
	if tools[0].Name != "a_tool" || tools[1].Name != "m_tool" || tools[2].Name != "z_tool" {
		t.Errorf("unexpected order: %q, %q, %q", tools[0].Name, tools[1].Name, tools[2].Name)
	}
}

func TestRegistryDuplicateNameLastWins(t *testing.T) {
	reg := NewRegistry([]Tool{
		{Name: "dup", Description: "first", Handler: func(ctx context.Context, args map[string]any) (any, error) { return "first", nil }},
		{Name: "dup", Description: "second", Handler: func(ctx context.Context, args map[string]any) (any, error) { return "second", nil }},
	})

	tool, ok := reg.Get("dup")
	if !ok {
		t.Fatal("Get(dup) should find tool")
	}
	if tool.Description != "second" {
		t.Errorf("expected last tool to win, got Description %q", tool.Description)
	}
	// ハンドラも後勝ち
	result, _ := tool.Handler(context.Background(), nil)
	if result != "second" {
		t.Errorf("expected handler to return 'second', got %v", result)
	}

	tools := reg.List()
	if len(tools) != 1 {
		t.Errorf("expected 1 tool in list (duplicates merged), got %d", len(tools))
	}
}
