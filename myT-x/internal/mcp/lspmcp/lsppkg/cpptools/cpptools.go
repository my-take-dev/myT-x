// Package cpptools は cpptools 向けの MCP 拡張ツールを提供する。
package cpptools

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	"myT-x/internal/mcp/lspmcp/internal/lsp"
	"myT-x/internal/mcp/lspmcp/internal/mcp"
)

type methodSpec struct {
	Name        string `json:"name"`
	When        string `json:"when"`
	Args        string `json:"args"`
	Effect      string `json:"effect"`
	Description string `json:"description"`
}

type methodGuide struct {
	When        string `json:"when"`
	Args        string `json:"args"`
	Effect      string `json:"effect"`
	Description string `json:"description"`
}

// staticMethodCatalog keeps known cpptools-specific request methods explicit.
var staticMethodCatalog = map[string]methodSpec{
	"cpptools/queryCompilerDefaults": newMethodSpec(
		"cpptools/queryCompilerDefaults",
		"Query compiler defaults used by cpptools IntelliSense",
		"method-specific params object",
		"read",
	),
	"cpptools/getDiagnostics": newMethodSpec(
		"cpptools/getDiagnostics",
		"Get cpptools diagnostics summary output",
		"method-specific params object",
		"read",
	),
	"cpptools/getIncludes": newMethodSpec(
		"cpptools/getIncludes",
		"Get include graph via cpptools include analysis",
		"fileUri, maxDepth?",
		"read",
	),
	"cpptools/didSwitchHeaderSource": newMethodSpec(
		"cpptools/didSwitchHeaderSource",
		"Switch between header/source according to cpptools logic",
		"workspaceFolderUri, switchHeaderSourceFileName",
		"exec",
	),
	"cpptools/getDocumentSymbols": newMethodSpec(
		"cpptools/getDocumentSymbols",
		"Get cpptools document symbols",
		"method-specific params object",
		"read",
	),
	"cpptools/getWorkspaceSymbols": newMethodSpec(
		"cpptools/getWorkspaceSymbols",
		"Get cpptools workspace symbols",
		"method-specific params object",
		"read",
	),
	"cpptools/getFoldingRanges": newMethodSpec(
		"cpptools/getFoldingRanges",
		"Get cpptools folding ranges",
		"method-specific params object",
		"read",
	),
	"cpptools/hover": newMethodSpec(
		"cpptools/hover",
		"Get cpptools hover information",
		"method-specific params object",
		"read",
	),
}

// BuildTools は cpptools 言語サーバー向けの拡張ツールを構築する。
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
			Name:        "cpptools_list_extension_methods",
			Description: triadDescription("Inspect cpptools-specific request methods available in the static extension catalog", "none", "read"),
			InputSchema: emptySchema(),
			Handler:     svc.handleListMethods,
		},
		{
			Name:        "cpptools_call_extension_method",
			Description: triadDescription("Call one cpptools-specific request method from the static extension catalog", "method, params?", "exec"),
			InputSchema: callMethodSchema(),
			Handler:     svc.handleCallMethod,
		},
		{
			Name:        "cpptools_get_includes",
			Description: triadDescription("Get include graph for a file via cpptools/getIncludes", "relativePath (maxDepth optional)", "read"),
			InputSchema: getIncludesSchema(),
			Handler:     svc.handleGetIncludes,
		},
		{
			Name:        "cpptools_switch_header_source",
			Description: triadDescription("Switch header/source for a file via cpptools/didSwitchHeaderSource", "relativePath", "exec"),
			InputSchema: fileOnlySchema(),
			Handler:     svc.handleSwitchHeaderSource,
		},
	}
}

// Matches は設定されたコマンド/引数が cpptools を示す場合に true を返す。
func Matches(command string, args []string) bool {
	if looksLikeCppTools(command) {
		return true
	}

	return slices.ContainsFunc(args, looksLikeCppTools)
}

type service struct {
	client  *lsp.Client
	rootDir string
}

func (s *service) handleListMethods(_ context.Context, _ map[string]any) (any, error) {
	methods := sortedMethods()
	out := make([]map[string]any, 0, len(methods))
	for _, spec := range methods {
		out = append(out, map[string]any{
			"name":        spec.Name,
			"description": spec.Description,
			"when":        spec.When,
			"args":        spec.Args,
			"effect":      spec.Effect,
			"source":      "static-catalog",
		})
	}

	return map[string]any{
		"lsp":               "cpptools",
		"root":              s.rootDir,
		"methods":           out,
		"staticMethodCount": len(out),
	}, nil
}

func (s *service) handleCallMethod(ctx context.Context, args map[string]any) (any, error) {
	method, err := requiredString(args, "method")
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(method, "cpptools/") {
		return nil, fmt.Errorf("method must start with cpptools/")
	}

	params := map[string]any{}
	if raw, ok := args["params"]; ok {
		p, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("params must be an object")
		}
		params = p
	}

	raw, err := s.client.Request(ctx, method, params)
	if err != nil {
		return nil, err
	}

	result, err := decodeAny(raw)
	if err != nil {
		return nil, err
	}

	spec, known := staticMethodCatalog[method]
	guide := unknownMethodGuide()
	if known {
		guide = guideFromSpec(spec)
	}

	return map[string]any{
		"lsp":            "cpptools",
		"root":           s.rootDir,
		"method":         method,
		"knownInCatalog": known,
		"catalogEntry":   spec,
		"methodGuide":    guide,
		"params":         params,
		"result":         result,
	}, nil
}

func (s *service) handleGetIncludes(ctx context.Context, args map[string]any) (any, error) {
	target, snapshot, err := s.resolveDocument(ctx, args)
	if err != nil {
		return nil, err
	}

	maxDepth, _, err := optionalInt(args, "maxDepth")
	if err != nil {
		return nil, err
	}
	if maxDepth <= 0 {
		maxDepth = 3
	}

	raw, err := s.client.Request(ctx, "cpptools/getIncludes", map[string]any{
		"fileUri":  snapshot.URI,
		"maxDepth": maxDepth,
	})
	if err != nil {
		return nil, err
	}

	result, err := decodeAny(raw)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"lsp":      "cpptools",
		"path":     target.RelativePath,
		"maxDepth": maxDepth,
		"includes": result,
	}, nil
}

func (s *service) handleSwitchHeaderSource(ctx context.Context, args map[string]any) (any, error) {
	target, _, err := s.resolveDocument(ctx, args)
	if err != nil {
		return nil, err
	}

	rootURI, err := lsp.PathToURI(target.RootDir)
	if err != nil {
		return nil, err
	}

	raw, err := s.client.Request(ctx, "cpptools/didSwitchHeaderSource", map[string]any{
		"workspaceFolderUri":         rootURI,
		"switchHeaderSourceFileName": target.AbsolutePath,
	})
	if err != nil {
		return nil, err
	}

	result, err := decodeAny(raw)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"lsp":       "cpptools",
		"path":      target.RelativePath,
		"requested": target.AbsolutePath,
		"result":    result,
	}, nil
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

func sortedMethods() []methodSpec {
	names := make([]string, 0, len(staticMethodCatalog))
	for name := range staticMethodCatalog {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]methodSpec, 0, len(names))
	for _, name := range names {
		out = append(out, staticMethodCatalog[name])
	}
	return out
}

func newMethodSpec(name string, when string, args string, effect string) methodSpec {
	guide := methodGuide{
		When:   strings.TrimSpace(when),
		Args:   strings.TrimSpace(args),
		Effect: strings.TrimSpace(effect),
	}
	guide.Description = triadDescription(guide.When, guide.Args, guide.Effect)

	return methodSpec{
		Name:        name,
		When:        guide.When,
		Args:        guide.Args,
		Effect:      guide.Effect,
		Description: guide.Description,
	}
}

func guideFromSpec(spec methodSpec) methodGuide {
	return methodGuide{
		When:        spec.When,
		Args:        spec.Args,
		Effect:      spec.Effect,
		Description: spec.Description,
	}
}

func unknownMethodGuide() methodGuide {
	when := "Use when this cpptools build exposes a method outside the static catalog"
	args := "method-specific params object"
	effect := "exec"
	return methodGuide{
		When:        when,
		Args:        args,
		Effect:      effect,
		Description: triadDescription(when, args, effect),
	}
}

func looksLikeCppTools(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	switch base {
	case "cpptools", "cpptools.exe", "cpptools-srv", "cpptools-srv.exe":
		return true
	default:
		return false
	}
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

func triadDescription(when string, args string, effect string) string {
	return "when: " + strings.TrimSpace(when) + " args: " + strings.TrimSpace(args) + " effect: " + strings.TrimSpace(effect) + "."
}

func emptySchema() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}

func callMethodSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"method": map[string]any{
				"type":        "string",
				"description": "cpptools request method name.",
			},
			"params": map[string]any{
				"type":        "object",
				"description": "Method parameters object.",
			},
		},
		"required": []string{"method"},
	}
}

func fileOnlySchema() map[string]any {
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
		},
		"required": []string{"relativePath"},
	}
}

func getIncludesSchema() map[string]any {
	schema := fileOnlySchema()
	props := schema["properties"].(map[string]any)
	props["maxDepth"] = map[string]any{
		"type":        "integer",
		"description": "Maximum include traversal depth.",
	}
	return schema
}
