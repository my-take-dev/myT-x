// Package ccls は ccls 向けの MCP 拡張ツールを提供する。
package ccls

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"unicode/utf16"

	"myT-x/internal/mcp/lspmcp/internal/lsp"
	"myT-x/internal/mcp/lspmcp/internal/mcp"
)

// BuildTools は ccls 言語サーバー向けの拡張ツールを構築する。
func BuildTools(client *lsp.Client, rootDir string) []mcp.Tool {
	if client == nil {
		return nil
	}
	svc := &service{
		client:  client,
		rootDir: rootDir,
	}

	return []mcp.Tool{
		{
			Name:        "ccls_get_call_hierarchy",
			Description: triadDescription("Get caller/callee hierarchy via ccls $ccls/call", "relativePath, position (line/character or textTarget; options optional)", "read"),
			InputSchema: crossRefSchema(),
			Handler:     svc.handleCallHierarchy,
		},
		{
			Name:        "ccls_get_inheritance_hierarchy",
			Description: triadDescription("Get base/derived hierarchy via ccls $ccls/inheritance", "relativePath, position (line/character or textTarget; options optional)", "read"),
			InputSchema: crossRefSchema(),
			Handler:     svc.handleInheritanceHierarchy,
		},
		{
			Name:        "ccls_get_member_hierarchy",
			Description: triadDescription("Get member/type/function hierarchy via ccls $ccls/member", "relativePath, position (line/character or textTarget; options optional)", "read"),
			InputSchema: crossRefSchema(),
			Handler:     svc.handleMemberHierarchy,
		},
		{
			Name:        "ccls_get_vars",
			Description: triadDescription("Get variable-type relationships via ccls $ccls/vars", "relativePath, position (line/character or textTarget; options optional)", "read"),
			InputSchema: crossRefSchema(),
			Handler:     svc.handleVars,
		},
		{
			Name:        "ccls_navigate",
			Description: triadDescription("Navigate semantically via ccls $ccls/navigate", "relativePath, direction (line/character or textTarget optional; options optional)", "read"),
			InputSchema: navigateSchema(),
			Handler:     svc.handleNavigate,
		},
	}
}

// Matches は設定されたコマンド/引数が ccls を示す場合に true を返す。
func Matches(command string, args []string) bool {
	if looksLikeCcls(command) {
		return true
	}

	return slices.ContainsFunc(args, looksLikeCcls)
}

type service struct {
	client  *lsp.Client
	rootDir string
}

func (s *service) handleCallHierarchy(ctx context.Context, args map[string]any) (any, error) {
	return s.handleCrossReference(ctx, args, "$ccls/call", "call")
}

func (s *service) handleInheritanceHierarchy(ctx context.Context, args map[string]any) (any, error) {
	return s.handleCrossReference(ctx, args, "$ccls/inheritance", "inheritance")
}

func (s *service) handleMemberHierarchy(ctx context.Context, args map[string]any) (any, error) {
	return s.handleCrossReference(ctx, args, "$ccls/member", "member")
}

func (s *service) handleVars(ctx context.Context, args map[string]any) (any, error) {
	return s.handleCrossReference(ctx, args, "$ccls/vars", "vars")
}

func (s *service) handleNavigate(ctx context.Context, args map[string]any) (any, error) {
	direction, err := requiredString(args, "direction")
	if err != nil {
		return nil, err
	}

	target, snapshot, err := s.resolveDocument(ctx, args)
	if err != nil {
		return nil, err
	}

	line, character, err := resolvePosition(snapshot.Content, args, true)
	if err != nil {
		return nil, err
	}

	params := map[string]any{
		"textDocument": map[string]any{"uri": snapshot.URI},
		"position":     map[string]any{"line": line, "character": character},
		"direction":    direction,
	}
	if err := mergeOptionObject(params, args); err != nil {
		return nil, err
	}

	raw, err := s.client.Request(ctx, "$ccls/navigate", params)
	if err != nil {
		return nil, err
	}

	value, err := decodeAny(raw)
	if err != nil {
		return nil, err
	}

	result := map[string]any{
		"lsp":       "ccls",
		"path":      target.RelativePath,
		"position":  map[string]any{"line": line + 1, "character": character},
		"direction": direction,
		"navigate":  value,
	}
	if items, ok := value.([]any); ok {
		result["count"] = len(items)
	}
	return result, nil
}

func (s *service) handleCrossReference(ctx context.Context, args map[string]any, method string, resultKey string) (any, error) {
	target, snapshot, err := s.resolveDocument(ctx, args)
	if err != nil {
		return nil, err
	}

	line, character, err := resolvePosition(snapshot.Content, args, true)
	if err != nil {
		return nil, err
	}

	params := map[string]any{
		"textDocument": map[string]any{"uri": snapshot.URI},
		"position":     map[string]any{"line": line, "character": character},
	}
	if err := mergeOptionObject(params, args); err != nil {
		return nil, err
	}

	raw, err := s.client.Request(ctx, method, params)
	if err != nil {
		return nil, err
	}

	value, err := decodeAny(raw)
	if err != nil {
		return nil, err
	}

	out := map[string]any{
		"lsp":      "ccls",
		"path":     target.RelativePath,
		"position": map[string]any{"line": line + 1, "character": character},
	}
	out[resultKey] = value
	if items, ok := value.([]any); ok {
		out["count"] = len(items)
	}
	return out, nil
}

func mergeOptionObject(params map[string]any, args map[string]any) error {
	raw, ok := args["options"]
	if !ok {
		return nil
	}
	options, ok := raw.(map[string]any)
	if !ok {
		return fmt.Errorf("options must be an object")
	}
	for k, v := range options {
		if k == "textDocument" || k == "position" {
			continue
		}
		params[k] = v
	}
	return nil
}

type documentTarget struct {
	RootDir      string
	RelativePath string
	AbsolutePath string
}

func (s *service) resolveDocument(ctx context.Context, args map[string]any) (documentTarget, lsp.DocumentSnapshot, error) {
	rootDir := s.rootDir
	if rootArg, ok := args["root"]; ok {
		rootStr, ok := rootArg.(string)
		if !ok || strings.TrimSpace(rootStr) == "" {
			return documentTarget{}, lsp.DocumentSnapshot{}, fmt.Errorf("root must be a non-empty string")
		}
		rootAbs, err := filepath.Abs(rootStr)
		if err != nil {
			return documentTarget{}, lsp.DocumentSnapshot{}, err
		}
		rootDir = filepath.Clean(rootAbs)
	}

	relativePath, err := requiredString(args, "relativePath")
	if err != nil {
		return documentTarget{}, lsp.DocumentSnapshot{}, err
	}

	absolutePath := filepath.Clean(relativePath)
	if !filepath.IsAbs(relativePath) {
		absolutePath = filepath.Clean(filepath.Join(rootDir, relativePath))
	}

	snapshot, err := s.client.EnsureDocument(ctx, absolutePath)
	if err != nil {
		return documentTarget{}, lsp.DocumentSnapshot{}, err
	}

	return documentTarget{
		RootDir:      rootDir,
		RelativePath: lsp.RelativePath(rootDir, absolutePath),
		AbsolutePath: absolutePath,
	}, snapshot, nil
}

func resolvePosition(content string, args map[string]any, requirePosition bool) (int, int, error) {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		lines = []string{""}
	}

	lineIndex, hasLine, err := parseLineArg(args, lines)
	if err != nil {
		return 0, 0, err
	}

	character, hasCharacter, err := parseCharacterArg(args)
	if err != nil {
		return 0, 0, err
	}

	textTarget := optionalString(args, "textTarget")

	if hasLine {
		if lineIndex < 0 || lineIndex >= len(lines) {
			return 0, 0, fmt.Errorf("line out of range: %d", lineIndex+1)
		}
		if !hasCharacter {
			if textTarget != "" {
				offset := strings.Index(lines[lineIndex], textTarget)
				if offset < 0 {
					return 0, 0, fmt.Errorf("textTarget not found on line %d", lineIndex+1)
				}
				character = byteOffsetToUTF16(lines[lineIndex], offset)
			} else {
				character = 0
			}
		}
		return lineIndex, character, nil
	}

	if textTarget != "" {
		for i, line := range lines {
			offset := strings.Index(line, textTarget)
			if offset >= 0 {
				return i, byteOffsetToUTF16(line, offset), nil
			}
		}
		return 0, 0, fmt.Errorf("textTarget not found in file")
	}

	if requirePosition {
		return 0, 0, fmt.Errorf("line or textTarget is required")
	}

	if hasCharacter {
		return 0, character, nil
	}
	return 0, 0, nil
}

func parseLineArg(args map[string]any, lines []string) (int, bool, error) {
	raw, ok := args["line"]
	if !ok {
		return 0, false, nil
	}

	switch v := raw.(type) {
	case float64:
		if math.Trunc(v) != v {
			return 0, false, fmt.Errorf("line must be an integer")
		}
		line := int(v)
		if line <= 0 {
			return 0, false, fmt.Errorf("line must be >= 1")
		}
		return line - 1, true, nil
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return 0, false, fmt.Errorf("line string cannot be empty")
		}
		if n, err := strconv.Atoi(trimmed); err == nil {
			if n <= 0 {
				return 0, false, fmt.Errorf("line must be >= 1")
			}
			return n - 1, true, nil
		}
		for i, lineText := range lines {
			if strings.Contains(lineText, trimmed) {
				return i, true, nil
			}
		}
		return 0, false, fmt.Errorf("line containing %q not found", trimmed)
	default:
		return 0, false, fmt.Errorf("line must be integer or string")
	}
}

func parseCharacterArg(args map[string]any) (int, bool, error) {
	if value, ok, err := optionalInt(args, "character"); err != nil {
		return 0, false, err
	} else if ok {
		if value < 0 {
			return 0, false, fmt.Errorf("character must be >= 0")
		}
		return value, true, nil
	}
	if value, ok, err := optionalInt(args, "column"); err != nil {
		return 0, false, err
	} else if ok {
		if value < 0 {
			return 0, false, fmt.Errorf("column must be >= 0")
		}
		return value, true, nil
	}
	return 0, false, nil
}

func byteOffsetToUTF16(line string, offset int) int {
	if offset <= 0 {
		return 0
	}
	if offset > len(line) {
		offset = len(line)
	}
	col := 0
	for i, r := range line {
		if i >= offset {
			break
		}
		col += len(utf16.Encode([]rune{r}))
	}
	return col
}

func requiredString(args map[string]any, key string) (string, error) {
	raw, ok := args[key]
	if !ok {
		return "", fmt.Errorf("%s is required", key)
	}
	value, ok := raw.(string)
	if !ok || strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("%s must be a non-empty string", key)
	}
	return value, nil
}

func triadDescription(when string, args string, effect string) string {
	return "when: " + strings.TrimSpace(when) + " args: " + strings.TrimSpace(args) + " effect: " + strings.TrimSpace(effect) + "."
}

func optionalString(args map[string]any, key string) string {
	raw, ok := args[key]
	if !ok {
		return ""
	}
	value, ok := raw.(string)
	if !ok {
		return ""
	}
	return value
}

func optionalInt(args map[string]any, key string) (int, bool, error) {
	raw, ok := args[key]
	if !ok {
		return 0, false, nil
	}
	switch v := raw.(type) {
	case float64:
		if math.Trunc(v) != v {
			return 0, false, fmt.Errorf("%s must be an integer", key)
		}
		return int(v), true, nil
	case int:
		return v, true, nil
	case int64:
		return int(v), true, nil
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			return 0, false, fmt.Errorf("%s must be an integer string", key)
		}
		return n, true, nil
	default:
		return 0, false, fmt.Errorf("%s must be an integer", key)
	}
}

func decodeAny(raw json.RawMessage) (any, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return nil, nil
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	return value, nil
}

func looksLikeCcls(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	return base == "ccls" || base == "ccls.exe"
}

func crossRefSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"root": map[string]any{
				"type":        "string",
				"description": "Optional root directory override.",
			},
			"relativePath": map[string]any{
				"type":        "string",
				"description": "Target file path relative to root (or absolute path).",
			},
			"line": map[string]any{
				"anyOf": []map[string]any{
					{"type": "integer"},
					{"type": "string"},
				},
				"description": "Line number (1-based) or a line snippet to search.",
			},
			"character": map[string]any{
				"type":        "integer",
				"description": "Character offset in UTF-16 (0-based).",
			},
			"column": map[string]any{
				"type":        "integer",
				"description": "Alias for character.",
			},
			"textTarget": map[string]any{
				"type":        "string",
				"description": "Optional text to locate symbol when character is omitted.",
			},
			"options": map[string]any{
				"type":        "object",
				"description": "Method-specific ccls options (e.g. callee, derived, levels, kind, hierarchy).",
			},
		},
		"required": []string{"relativePath"},
	}
}

func navigateSchema() map[string]any {
	schema := crossRefSchema()
	props := schema["properties"].(map[string]any)
	props["direction"] = map[string]any{
		"type":        "string",
		"description": "Navigation direction for $ccls/navigate (e.g. D, L, R, U).",
	}
	schema["required"] = []string{"relativePath", "direction"}
	return schema
}
