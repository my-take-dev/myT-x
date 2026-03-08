package mcp

import (
	"context"
	"sort"
)

// ToolHandler は tools/call リクエストを処理する。
type ToolHandler func(ctx context.Context, arguments map[string]any) (any, error)

// Tool は 1 つの MCP ツールを定義する。
type Tool struct {
	Name        string         // ツール名。tools/list と tools/call で使用する。
	Description string         // ツールの説明文。
	InputSchema map[string]any // JSON Schema 形式の入力パラメータ定義。
	Handler     ToolHandler    // tools/call 実行時に呼ばれるハンドラ。
}

// Registry は名前でツール定義を保持する。
type Registry struct {
	tools map[string]Tool
}

// NewRegistry は新しいレジストリを構築する。
// 同名ツールが複数ある場合は後勝ちで上書きされる。
func NewRegistry(tools []Tool) *Registry {
	m := make(map[string]Tool, len(tools))
	for _, tool := range tools {
		m[tool.Name] = tool
	}
	return &Registry{tools: m}
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
