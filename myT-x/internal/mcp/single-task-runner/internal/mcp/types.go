package mcp

import (
	"context"
	"fmt"
	"sort"
)

// ToolHandler handles a tools/call request.
type ToolHandler func(ctx context.Context, arguments map[string]any) (any, error)

// Tool defines one MCP tool.
type Tool struct {
	Name        string
	Description string
	InputSchema map[string]any
	Handler     ToolHandler
}

// Registry stores tool definitions by name.
type Registry struct {
	tools map[string]Tool
}

// NewRegistry builds a new registry.
func NewRegistry(tools []Tool) (*Registry, error) {
	toolMap := make(map[string]Tool, len(tools))
	for _, tool := range tools {
		if _, exists := toolMap[tool.Name]; exists {
			return nil, fmt.Errorf("duplicate tool name %q", tool.Name)
		}
		toolMap[tool.Name] = tool
	}
	return &Registry{tools: toolMap}, nil
}

// MustNewRegistry builds a new registry and panics on duplicates.
func MustNewRegistry(tools []Tool) *Registry {
	registry, err := NewRegistry(tools)
	if err != nil {
		panic(err)
	}
	return registry
}

// Get returns one tool by name.
func (r *Registry) Get(name string) (Tool, bool) {
	tool, ok := r.tools[name]
	return tool, ok
}

// List returns all tools sorted by name.
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
