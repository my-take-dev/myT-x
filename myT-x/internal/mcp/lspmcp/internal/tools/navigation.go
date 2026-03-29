package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"strings"

	"myT-x/internal/mcp/lspmcp/internal/lsp"
)

// --- ナビゲーション系ハンドラ（定義/参照ジャンプ、シンボル、補完、シグネチャヘルプ） ---

func (s *service) handleHover(ctx context.Context, args map[string]any) (any, error) {
	target, snapshot, err := s.resolveDocument(ctx, args)
	if err != nil {
		return nil, err
	}

	line, character, err := resolvePosition(snapshot.Content, args, true)
	if err != nil {
		return nil, err
	}

	raw, err := s.client.Request(ctx, "textDocument/hover", map[string]any{
		"textDocument": map[string]any{"uri": snapshot.URI},
		"position":     map[string]any{"line": line, "character": character},
	})
	if err != nil {
		return nil, err
	}

	hover, err := decodeAny(raw)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"path": target.RelativePath,
		"position": map[string]any{
			"line":      line + 1,
			"character": character,
		},
		"hover": hover,
	}, nil
}

func (s *service) handleDefinitions(ctx context.Context, args map[string]any) (any, error) {
	return s.handleLocationRequest(ctx, args, "textDocument/definition", "definitions")
}

func (s *service) handleDeclarations(ctx context.Context, args map[string]any) (any, error) {
	return s.handleLocationRequest(ctx, args, "textDocument/declaration", "declarations")
}

func (s *service) handleTypeDefinitions(ctx context.Context, args map[string]any) (any, error) {
	return s.handleLocationRequest(ctx, args, "textDocument/typeDefinition", "typeDefinitions")
}

func (s *service) handleImplementations(ctx context.Context, args map[string]any) (any, error) {
	return s.handleLocationRequest(ctx, args, "textDocument/implementation", "implementations")
}

func (s *service) handleLocationRequest(ctx context.Context, args map[string]any, method string, resultKey string) (any, error) {
	target, snapshot, err := s.resolveDocument(ctx, args)
	if err != nil {
		return nil, err
	}

	line, character, err := resolvePosition(snapshot.Content, args, true)
	if err != nil {
		return nil, err
	}

	before, _, err := optionalInt(args, "before")
	if err != nil {
		return nil, err
	}
	if before <= 0 {
		before = 2
	}
	after, _, err := optionalInt(args, "after")
	if err != nil {
		return nil, err
	}
	if after <= 0 {
		after = 2
	}

	raw, err := s.client.Request(ctx, method, map[string]any{
		"textDocument": map[string]any{"uri": snapshot.URI},
		"position":     map[string]any{"line": line, "character": character},
	})
	if err != nil {
		return nil, err
	}

	locations, err := extractLocations(raw)
	if err != nil {
		return nil, err
	}

	items := make([]map[string]any, 0, len(locations))
	var warnings []string
	for _, loc := range locations {
		path, err := lsp.URIToPath(loc.URI)
		if err != nil {
			s.logToolWarning("URIToPath failed while collecting locations (uri=%q): %v", loc.URI, err)
			warnings = append(warnings, fmt.Sprintf("URIToPath failed (uri=%q): %v", loc.URI, err))
			continue
		}
		preview, err := previewAround(path, loc.Range.Start.Line, before, after)
		if err != nil {
			s.logToolWarning("previewAround failed while collecting locations (path=%q line=%d): %v", path, loc.Range.Start.Line+1, err)
		}
		items = append(items, map[string]any{
			"path":         lsp.RelativePath(target.RootDir, path),
			"line":         loc.Range.Start.Line + 1,
			"character":    loc.Range.Start.Character + 1,
			"preview":      preview,
			"absolutePath": path,
		})
	}

	out := map[string]any{
		"requestedPath": target.RelativePath,
		"count":         len(items),
	}
	out[resultKey] = items
	if len(warnings) > 0 {
		out["skippedCount"] = len(warnings)
		out["warnings"] = warnings
	}
	return out, nil
}

func (s *service) handleReferences(ctx context.Context, args map[string]any) (any, error) {
	target, snapshot, err := s.resolveDocument(ctx, args)
	if err != nil {
		return nil, err
	}

	line, character, err := resolvePosition(snapshot.Content, args, true)
	if err != nil {
		return nil, err
	}

	includeDecl, err := boolArg(args, "includeDeclaration", true)
	if err != nil {
		return nil, err
	}
	before, _, err := optionalInt(args, "before")
	if err != nil {
		return nil, err
	}
	if before <= 0 {
		before = 1
	}
	after, _, err := optionalInt(args, "after")
	if err != nil {
		return nil, err
	}
	if after <= 0 {
		after = 1
	}

	raw, err := s.client.Request(ctx, "textDocument/references", map[string]any{
		"textDocument": map[string]any{"uri": snapshot.URI},
		"position":     map[string]any{"line": line, "character": character},
		"context": map[string]any{
			"includeDeclaration": includeDecl,
		},
	})
	if err != nil {
		return nil, err
	}

	locations, err := extractLocations(raw)
	if err != nil {
		return nil, err
	}

	items := make([]map[string]any, 0, len(locations))
	var warnings []string
	for _, loc := range locations {
		path, err := lsp.URIToPath(loc.URI)
		if err != nil {
			s.logToolWarning("URIToPath failed while collecting references (uri=%q): %v", loc.URI, err)
			warnings = append(warnings, fmt.Sprintf("URIToPath failed (uri=%q): %v", loc.URI, err))
			continue
		}
		preview, err := previewAround(path, loc.Range.Start.Line, before, after)
		if err != nil {
			s.logToolWarning("previewAround failed while collecting references (path=%q line=%d): %v", path, loc.Range.Start.Line+1, err)
		}
		items = append(items, map[string]any{
			"path":      lsp.RelativePath(target.RootDir, path),
			"line":      loc.Range.Start.Line + 1,
			"character": loc.Range.Start.Character + 1,
			"preview":   preview,
		})
	}

	result := map[string]any{
		"requestedPath": target.RelativePath,
		"count":         len(items),
		"references":    items,
	}
	if len(warnings) > 0 {
		result["skippedCount"] = len(warnings)
		result["warnings"] = warnings
	}
	return result, nil
}

func (s *service) handleDocumentSymbols(ctx context.Context, args map[string]any) (any, error) {
	target, snapshot, err := s.resolveDocument(ctx, args)
	if err != nil {
		return nil, err
	}

	raw, err := s.client.Request(ctx, "textDocument/documentSymbol", map[string]any{
		"textDocument": map[string]any{"uri": snapshot.URI},
	})
	if err != nil {
		return nil, err
	}

	symbols, err := decodeAny(raw)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"path":    target.RelativePath,
		"symbols": symbols,
	}, nil
}

func (s *service) handleWorkspaceSymbols(ctx context.Context, args map[string]any) (any, error) {
	query, err := requiredString(args, "query")
	if err != nil {
		return nil, err
	}

	raw, err := s.client.Request(ctx, "workspace/symbol", map[string]any{
		"query": query,
	})
	if err != nil {
		return nil, err
	}

	symbols, err := decodeAny(raw)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"query":   query,
		"symbols": symbols,
	}, nil
}

func (s *service) handleResolveWorkspaceSymbol(ctx context.Context, args map[string]any) (any, error) {
	symbol, err := requiredAny(args, "symbol")
	if err != nil {
		return nil, err
	}

	raw, err := s.client.Request(ctx, "workspaceSymbol/resolve", symbol)
	if err != nil {
		return nil, err
	}

	resolved, err := decodeAny(raw)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"symbol": resolved,
	}, nil
}

func (s *service) handleCompletion(ctx context.Context, args map[string]any) (any, error) {
	target, snapshot, err := s.resolveDocument(ctx, args)
	if err != nil {
		return nil, err
	}

	line, character, err := resolvePosition(snapshot.Content, args, true)
	if err != nil {
		return nil, err
	}

	maxItems, _, err := optionalInt(args, "maxItems")
	if err != nil {
		return nil, err
	}
	if maxItems <= 0 {
		maxItems = 50
	}

	raw, err := s.client.Request(ctx, "textDocument/completion", map[string]any{
		"textDocument": map[string]any{"uri": snapshot.URI},
		"position":     map[string]any{"line": line, "character": character},
	})
	if err != nil {
		return nil, err
	}

	completionValue, err := decodeAny(raw)
	if err != nil {
		return nil, err
	}
	trimmed, total, truncated := truncateCompletions(completionValue, maxItems)

	return map[string]any{
		"path":      target.RelativePath,
		"position":  map[string]any{"line": line + 1, "character": character},
		"total":     total,
		"truncated": truncated,
		"result":    trimmed,
	}, nil
}

func (s *service) handleResolveCompletionItem(ctx context.Context, args map[string]any) (any, error) {
	item, err := requiredAny(args, "item")
	if err != nil {
		return nil, err
	}

	raw, err := s.client.Request(ctx, "completionItem/resolve", item)
	if err != nil {
		return nil, err
	}

	resolved, err := decodeAny(raw)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"item": resolved,
	}, nil
}

func (s *service) handleSignatureHelp(ctx context.Context, args map[string]any) (any, error) {
	target, snapshot, err := s.resolveDocument(ctx, args)
	if err != nil {
		return nil, err
	}

	line, character, err := resolvePosition(snapshot.Content, args, true)
	if err != nil {
		return nil, err
	}

	raw, err := s.client.Request(ctx, "textDocument/signatureHelp", map[string]any{
		"textDocument": map[string]any{"uri": snapshot.URI},
		"position":     map[string]any{"line": line, "character": character},
	})
	if err != nil {
		return nil, err
	}

	signatures, err := decodeAny(raw)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"path":      target.RelativePath,
		"position":  map[string]any{"line": line + 1, "character": character},
		"signature": signatures,
	}, nil
}

// --- ナビゲーション系プライベートヘルパー ---

func extractLocations(raw json.RawMessage) ([]lsp.Location, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return nil, nil
	}

	if strings.HasPrefix(trimmed, "[") {
		var items []json.RawMessage
		if err := json.Unmarshal(raw, &items); err != nil {
			return nil, err
		}
		out := make([]lsp.Location, 0, len(items))
		for _, item := range items {
			loc, ok := parseLocation(item)
			if ok {
				out = append(out, loc)
			}
		}
		return out, nil
	}

	loc, ok := parseLocation(raw)
	if !ok {
		return nil, nil
	}
	return []lsp.Location{loc}, nil
}

func parseLocation(raw json.RawMessage) (lsp.Location, bool) {
	var candidate struct {
		URI                  string     `json:"uri"`
		Range                *lsp.Range `json:"range"`
		TargetURI            string     `json:"targetUri"`
		TargetRange          *lsp.Range `json:"targetRange"`
		TargetSelectionRange *lsp.Range `json:"targetSelectionRange"`
	}
	if err := json.Unmarshal(raw, &candidate); err != nil {
		return lsp.Location{}, false
	}

	if candidate.URI != "" && candidate.Range != nil {
		return lsp.Location{URI: candidate.URI, Range: *candidate.Range}, true
	}
	if candidate.TargetURI != "" {
		if candidate.TargetSelectionRange != nil {
			return lsp.Location{URI: candidate.TargetURI, Range: *candidate.TargetSelectionRange}, true
		}
		if candidate.TargetRange != nil {
			return lsp.Location{URI: candidate.TargetURI, Range: *candidate.TargetRange}, true
		}
	}
	return lsp.Location{}, false
}

func previewAround(path string, line int, before int, after int) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(content), "\n")
	if len(lines) == 0 {
		return "", nil
	}
	line = min(max(line, 0), len(lines)-1)

	start := max(line-before, 0)
	end := min(line+after, len(lines)-1)

	var parts []string
	for i := start; i <= end; i++ {
		parts = append(parts, fmt.Sprintf("%d: %s", i+1, lines[i]))
	}
	return strings.Join(parts, "\n"), nil
}

func truncateCompletions(value any, maxItems int) (any, int, bool) {
	switch v := value.(type) {
	case []any:
		total := len(v)
		if total > maxItems {
			return v[:maxItems], total, true
		}
		return v, total, false
	case map[string]any:
		items, ok := v["items"].([]any)
		if !ok {
			return v, 0, false
		}
		total := len(items)
		if total <= maxItems {
			return v, total, false
		}
		copied := make(map[string]any, len(v))
		maps.Copy(copied, v)
		copied["items"] = items[:maxItems]
		return copied, total, true
	default:
		return value, 0, false
	}
}
