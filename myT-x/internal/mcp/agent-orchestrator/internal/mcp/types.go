package mcp

import (
	"context"
	"fmt"
	"sort"
)

// ToolHandler は tools/call リクエストを処理する。
type ToolHandler func(ctx context.Context, arguments map[string]any) (any, error)

// Tool は 1 つの MCP ツールを定義する。
type Tool struct {
	Name        string
	Description string
	InputSchema map[string]any
	Handler     ToolHandler
}

// Registry は名前でツール定義を保持する。
type Registry struct {
	tools map[string]Tool
}

// NewRegistry は新しいレジストリを構築する。
func NewRegistry(tools []Tool) (*Registry, error) {
	m := make(map[string]Tool, len(tools))
	for _, tool := range tools {
		if _, exists := m[tool.Name]; exists {
			return nil, fmt.Errorf("duplicate tool name %q", tool.Name)
		}
		m[tool.Name] = tool
	}
	return &Registry{tools: m}, nil
}

// MustNewRegistry は新しいレジストリを構築し、重複があれば panic する。
func MustNewRegistry(tools []Tool) *Registry {
	registry, err := NewRegistry(tools)
	if err != nil {
		panic(err)
	}
	return registry
}

// Get は名前でツールを取得する。
func (r *Registry) Get(name string) (Tool, bool) {
	tool, ok := r.tools[name]
	return tool, ok
}

// List は名前でソートしたツール一覧を返す。
func (r *Registry) List() []Tool {
	out := make([]Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		out = append(out, tool)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}
