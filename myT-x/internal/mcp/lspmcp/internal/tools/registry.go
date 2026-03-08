package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"maps"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"

	"myT-x/internal/mcp/lspmcp/internal/lsp"
	"myT-x/internal/mcp/lspmcp/internal/mcp"
	lsppkg "myT-x/internal/mcp/lspmcp/lsppkg"
)

// service は汎用LSPツールの実処理を提供する。
type service struct {
	client  *lsp.Client
	rootDir string
	logger  *log.Logger
}

// triadDescription は AGENTS.md の when/args/effect 形式でツール説明を生成する。
func triadDescription(when string, args string, effect string) string {
	return "when: " + strings.TrimSpace(when) + " args: " + strings.TrimSpace(args) + " effect: " + strings.TrimSpace(effect) + "."
}

// logToolWarning はツール実行時の警告を Logger 経由で出力する。
func (s *service) logToolWarning(format string, args ...any) {
	s.logger.Printf("[tools] "+format, args...)
}

// BuildRegistry は汎用 LSP ツールを作成し、言語サーバー固有のツールを追加する。
func BuildRegistry(client *lsp.Client, rootDir string, lspCommand string, lspArgs []string) *mcp.Registry {
	s := &service{
		client:  client,
		rootDir: rootDir,
		logger:  client.Logger(),
	}
	scopeNote := lsppkg.DescribeCommonToolScope(lspCommand, lspArgs)
	positionArgs := "relativePath, position(line=1-based; character/column=UTF-16 0-based; textTarget optional)"
	positionWithPreviewArgs := positionArgs + ", before?, after?"
	referencesArgs := positionArgs + ", includeDeclaration?, before?, after?"
	rangeArgs := "relativePath, range(start line=1-based; character/column=UTF-16 0-based; endLine/endCharacter optional)"
	formatDocumentArgs := "relativePath (applyEdits optional, default=false)"
	formatRangeArgs := rangeArgs + ", applyEdits optional (default=false)"
	formatOnTypeArgs := positionArgs + ", ch, applyEdits optional (default=false)"
	renameArgs := positionArgs + ", newName, applyEdits optional (default=true)"

	tools := []mcp.Tool{
		{
			Name:        "lsp_check_capabilities",
			Description: triadDescription("Validate server feature support before selecting tools", "none", "read"),
			InputSchema: emptySchema(),
			Handler:     s.handleCheckCapabilities,
		},
		{
			Name:        "lsp_get_hover",
			Description: triadDescription("Read symbol documentation or inferred type at a position", positionArgs, "read"),
			InputSchema: filePositionSchema(false),
			Handler:     s.handleHover,
		},
		{
			Name:        "lsp_get_definitions",
			Description: triadDescription("Jump from usage to symbol definition", positionWithPreviewArgs, "read"),
			InputSchema: filePositionWithContextSchema(),
			Handler:     s.handleDefinitions,
		},
		{
			Name:        "lsp_get_declarations",
			Description: triadDescription("Locate symbol declarations", positionWithPreviewArgs, "read"),
			InputSchema: filePositionWithContextSchema(),
			Handler:     s.handleDeclarations,
		},
		{
			Name:        "lsp_get_type_definitions",
			Description: triadDescription("Locate declared type definitions for a symbol", positionWithPreviewArgs, "read"),
			InputSchema: filePositionWithContextSchema(),
			Handler:     s.handleTypeDefinitions,
		},
		{
			Name:        "lsp_get_implementations",
			Description: triadDescription("Find concrete implementations for an interface or method", positionWithPreviewArgs, "read"),
			InputSchema: filePositionWithContextSchema(),
			Handler:     s.handleImplementations,
		},
		{
			Name:        "lsp_find_references",
			Description: triadDescription("Find callsites or usages of a symbol", referencesArgs, "read"),
			InputSchema: referencesSchema(),
			Handler:     s.handleReferences,
		},
		{
			Name:        "lsp_get_document_symbols",
			Description: triadDescription("Inspect symbols declared in one file", "relativePath", "read"),
			InputSchema: fileOnlySchema(),
			Handler:     s.handleDocumentSymbols,
		},
		{
			Name:        "lsp_get_workspace_symbols",
			Description: triadDescription("Search symbol names across the workspace", "query", "read"),
			InputSchema: workspaceSymbolSchema(),
			Handler:     s.handleWorkspaceSymbols,
		},
		{
			Name:        "lsp_resolve_workspace_symbol",
			Description: triadDescription("Fetch additional details for one workspace symbol result", "symbol", "read"),
			InputSchema: resolveWorkspaceSymbolSchema(),
			Handler:     s.handleResolveWorkspaceSymbol,
		},
		{
			Name:        "lsp_get_completion",
			Description: triadDescription("Request completion candidates at cursor", positionArgs+", maxItems?", "read"),
			InputSchema: completionSchema(),
			Handler:     s.handleCompletion,
		},
		{
			Name:        "lsp_resolve_completion_item",
			Description: triadDescription("Fetch expanded detail for one completion candidate", "item", "read"),
			InputSchema: resolveCompletionItemSchema(),
			Handler:     s.handleResolveCompletionItem,
		},
		{
			Name:        "lsp_get_signature_help",
			Description: triadDescription("Show function signature and active parameter at callsite", positionArgs, "read"),
			InputSchema: filePositionSchema(false),
			Handler:     s.handleSignatureHelp,
		},
		{
			Name:        "lsp_get_diagnostics",
			Description: triadDescription("Check errors and warnings for a file", "relativePath (usePull/waitMs optional)", "read"),
			InputSchema: diagnosticsSchema(),
			Handler:     s.handleDiagnostics,
		},
		{
			Name:        "lsp_get_workspace_diagnostics",
			Description: triadDescription("Collect diagnostics across workspace files", "identifier?, previousResultIds?", "read"),
			InputSchema: workspaceDiagnosticsSchema(),
			Handler:     s.handleWorkspaceDiagnostics,
		},
		{
			Name:        "lsp_get_code_actions",
			Description: triadDescription("List quick fixes or refactors for a selection", rangeArgs, "read"),
			InputSchema: codeActionSchema(),
			Handler:     s.handleCodeActions,
		},
		{
			Name:        "lsp_resolve_code_action",
			Description: triadDescription("Fetch full edit or command payload for a code action", "action", "read"),
			InputSchema: resolveCodeActionSchema(),
			Handler:     s.handleResolveCodeAction,
		},
		{
			Name:        "lsp_format_document",
			Description: triadDescription("Format a full document", formatDocumentArgs, "read or edit"),
			InputSchema: formattingSchema(),
			Handler:     s.handleFormatting,
		},
		{
			Name:        "lsp_format_range",
			Description: triadDescription("Format a selected range", formatRangeArgs, "read or edit"),
			InputSchema: rangeFormattingSchema(),
			Handler:     s.handleRangeFormatting,
		},
		{
			Name:        "lsp_format_on_type",
			Description: triadDescription("Trigger format-on-type behavior", formatOnTypeArgs, "read or edit"),
			InputSchema: onTypeFormattingSchema(),
			Handler:     s.handleOnTypeFormatting,
		},
		{
			Name:        "lsp_rename_symbol",
			Description: triadDescription("Rename a symbol across the workspace", renameArgs, "read or edit"),
			InputSchema: renameSchema(),
			Handler:     s.handleRename,
		},
		{
			Name:        "lsp_prepare_rename",
			Description: triadDescription("Validate whether rename is allowed at position", positionArgs, "read"),
			InputSchema: filePositionSchema(false),
			Handler:     s.handlePrepareRename,
		},
		{
			Name:        "lsp_execute_command",
			Description: triadDescription("Run a server-specific workspace command (prefer gopls_execute_command when gopls extension is available)", "command, arguments?", "exec"),
			InputSchema: executeCommandSchema(),
			Handler:     s.handleExecuteCommand,
		},
	}
	for i := range tools {
		tools[i].Description = withScopeNote(tools[i].Description, scopeNote)
	}

	tools = append(tools, lsppkg.BuildTools(lspCommand, lspArgs, client, rootDir)...)

	return mcp.NewRegistry(tools)
}

func withScopeNote(description string, scopeNote string) string {
	trimmedDesc := strings.TrimSpace(description)
	trimmedScope := strings.TrimSpace(scopeNote)
	if trimmedScope == "" {
		return trimmedDesc
	}
	if trimmedDesc == "" {
		return trimmedScope
	}
	return trimmedDesc + " " + trimmedScope
}

func (s *service) handleCheckCapabilities(_ context.Context, _ map[string]any) (any, error) {
	rawCaps := s.client.Capabilities()
	caps := make(map[string]any, len(rawCaps))
	for k, raw := range rawCaps {
		var decoded any
		if err := json.Unmarshal(raw, &decoded); err != nil {
			caps[k] = string(raw)
		} else {
			caps[k] = decoded
		}
	}

	return map[string]any{
		"root":         s.rootDir,
		"capabilities": caps,
		"supportedFeatures": map[string]bool{
			"hover":                   supportsTopLevelCapability(rawCaps, "hoverProvider"),
			"definition":              supportsTopLevelCapability(rawCaps, "definitionProvider"),
			"declaration":             supportsTopLevelCapability(rawCaps, "declarationProvider"),
			"typeDefinition":          supportsTopLevelCapability(rawCaps, "typeDefinitionProvider"),
			"implementation":          supportsTopLevelCapability(rawCaps, "implementationProvider"),
			"references":              supportsTopLevelCapability(rawCaps, "referencesProvider"),
			"documentSymbols":         supportsTopLevelCapability(rawCaps, "documentSymbolProvider"),
			"workspaceSymbols":        supportsTopLevelCapability(rawCaps, "workspaceSymbolProvider"),
			"workspaceSymbolResolve":  supportsCapabilityField(rawCaps, "workspaceSymbolProvider", "resolveProvider"),
			"completion":              supportsTopLevelCapability(rawCaps, "completionProvider"),
			"completionResolve":       supportsCapabilityField(rawCaps, "completionProvider", "resolveProvider"),
			"signatureHelp":           supportsTopLevelCapability(rawCaps, "signatureHelpProvider"),
			"codeActions":             supportsTopLevelCapability(rawCaps, "codeActionProvider"),
			"codeActionResolve":       supportsCapabilityField(rawCaps, "codeActionProvider", "resolveProvider"),
			"formatting":              supportsTopLevelCapability(rawCaps, "documentFormattingProvider"),
			"rangeFormatting":         supportsTopLevelCapability(rawCaps, "documentRangeFormattingProvider"),
			"onTypeFormatting":        supportsTopLevelCapability(rawCaps, "documentOnTypeFormattingProvider"),
			"rename":                  supportsTopLevelCapability(rawCaps, "renameProvider"),
			"prepareRename":           supportsCapabilityField(rawCaps, "renameProvider", "prepareProvider"),
			"pullDiagnostics":         supportsTopLevelCapability(rawCaps, "diagnosticProvider"),
			"workspaceDiagnostics":    supportsCapabilityField(rawCaps, "diagnosticProvider", "workspaceDiagnostics"),
			"executeCommand":          supportsTopLevelCapability(rawCaps, "executeCommandProvider"),
			"workspaceExecuteCommand": supportsTopLevelCapability(rawCaps, "executeCommandProvider"),
		},
	}, nil
}

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

	raw, err := s.client.Request(ctx, "textDocument/definition", map[string]any{
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
	for _, loc := range locations {
		path, err := lsp.URIToPath(loc.URI)
		if err != nil {
			s.logToolWarning("URIToPath failed while collecting definitions (uri=%q): %v", loc.URI, err)
			continue
		}
		preview, err := previewAround(path, loc.Range.Start.Line, before, after)
		if err != nil {
			s.logToolWarning("previewAround failed while collecting definitions (path=%q line=%d): %v", path, loc.Range.Start.Line+1, err)
		}
		items = append(items, map[string]any{
			"path":         lsp.RelativePath(target.RootDir, path),
			"line":         loc.Range.Start.Line + 1,
			"character":    loc.Range.Start.Character + 1,
			"preview":      preview,
			"absolutePath": path,
		})
	}

	return map[string]any{
		"requestedPath": target.RelativePath,
		"count":         len(items),
		"definitions":   items,
	}, nil
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
	for _, loc := range locations {
		path, err := lsp.URIToPath(loc.URI)
		if err != nil {
			s.logToolWarning("URIToPath failed while collecting locations (uri=%q): %v", loc.URI, err)
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
	for _, loc := range locations {
		path, err := lsp.URIToPath(loc.URI)
		if err != nil {
			s.logToolWarning("URIToPath failed while collecting references (uri=%q): %v", loc.URI, err)
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

	return map[string]any{
		"requestedPath": target.RelativePath,
		"count":         len(items),
		"references":    items,
	}, nil
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

func (s *service) handleDiagnostics(ctx context.Context, args map[string]any) (any, error) {
	target, snapshot, err := s.resolveDocument(ctx, args)
	if err != nil {
		return nil, err
	}

	usePull, err := boolArg(args, "usePull", true)
	if err != nil {
		return nil, err
	}

	// textDocument/diagnostic の応答が map の場合のみ pull 結果として返す。
	// items が空でも pull 成功として返し、それ以外の型の場合は push-cache にフォールバックする。
	if usePull && s.client.SupportsCapability("diagnosticProvider") {
		raw, reqErr := s.client.Request(ctx, "textDocument/diagnostic", map[string]any{
			"textDocument": map[string]any{"uri": snapshot.URI},
		})
		if reqErr != nil {
			s.logToolWarning("pull diagnostics request failed (path=%q), falling back to push-cache: %v", target.RelativePath, reqErr)
		} else {
			report, err := decodeAny(raw)
			if err != nil {
				return nil, err
			}
			if reportMap, ok := report.(map[string]any); ok {
				items := extractDiagnosticsFromPullReport(reportMap)
				if items == nil {
					items = []any{}
				}
				return map[string]any{
					"path":        target.RelativePath,
					"diagnostics": items,
					"count":       len(items),
					"source":      "pull",
					"pullReport":  reportMap,
				}, nil
			}
			s.logToolWarning("pull diagnostics report is not a JSON object (path=%q, got %T), falling back to push-cache", target.RelativePath, report)
		}
	}

	waitMs, _, err := optionalInt(args, "waitMs")
	if err != nil {
		return nil, err
	}
	if waitMs <= 0 {
		waitMs = 250
	}
	if err := sleep(ctx, time.Duration(waitMs)*time.Millisecond); err != nil {
		return nil, err
	}

	diagnostics := s.client.Diagnostics(snapshot.URI)
	out := make([]any, len(diagnostics))
	for i := range diagnostics {
		out[i] = diagnostics[i]
	}

	return map[string]any{
		"path":        target.RelativePath,
		"diagnostics": out,
		"count":       len(out),
		"source":      "push-cache",
	}, nil
}

func (s *service) handleWorkspaceDiagnostics(ctx context.Context, args map[string]any) (any, error) {
	rawCaps := s.client.Capabilities()
	if !supportsCapabilityField(rawCaps, "diagnosticProvider", "workspaceDiagnostics") {
		return nil, fmt.Errorf("workspace diagnostics not supported by server (diagnosticProvider.workspaceDiagnostics required)")
	}

	previousResultIDs, err := optionalArrayArg(args, "previousResultIds")
	if err != nil {
		return nil, err
	}
	identifier := optionalString(args, "identifier")

	params := map[string]any{
		"previousResultIds": previousResultIDs,
	}
	if strings.TrimSpace(identifier) != "" {
		params["identifier"] = identifier
	}

	raw, err := s.client.Request(ctx, "workspace/diagnostic", params)
	if err != nil {
		return nil, err
	}

	report, err := decodeAny(raw)
	if err != nil {
		return nil, err
	}
	items := extractDiagnosticsFromWorkspaceReport(report)

	return map[string]any{
		"diagnostics":       items,
		"count":             len(items),
		"source":            "workspace-pull",
		"workspaceReport":   report,
		"previousResultIds": previousResultIDs,
		"identifier":        identifier,
	}, nil
}

func (s *service) handleCodeActions(ctx context.Context, args map[string]any) (any, error) {
	target, snapshot, err := s.resolveDocument(ctx, args)
	if err != nil {
		return nil, err
	}

	line, character, err := resolvePosition(snapshot.Content, args, true)
	if err != nil {
		return nil, err
	}

	endLine := line
	if v, ok, err := optionalInt(args, "endLine"); err != nil {
		return nil, err
	} else if ok {
		endLine = v - 1
	}
	endCharacter := character
	if v, ok, err := optionalInt(args, "endCharacter"); err != nil {
		return nil, err
	} else if ok {
		endCharacter = v
	}
	if endLine < line || (endLine == line && endCharacter < character) {
		return nil, fmt.Errorf("invalid range: end before start")
	}

	pushDiagnostics := s.client.Diagnostics(snapshot.URI)
	rawDiagnostics := make([]any, len(pushDiagnostics))
	for i := range pushDiagnostics {
		rawDiagnostics[i] = pushDiagnostics[i]
	}

	raw, err := s.client.Request(ctx, "textDocument/codeAction", map[string]any{
		"textDocument": map[string]any{"uri": snapshot.URI},
		"range": map[string]any{
			"start": map[string]any{"line": line, "character": character},
			"end":   map[string]any{"line": endLine, "character": endCharacter},
		},
		"context": map[string]any{
			"diagnostics": rawDiagnostics,
		},
	})
	if err != nil {
		return nil, err
	}

	actions, err := decodeAny(raw)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"path":    target.RelativePath,
		"actions": actions,
		"range": map[string]any{
			"start": map[string]any{"line": line + 1, "character": character},
			"end":   map[string]any{"line": endLine + 1, "character": endCharacter},
		},
	}, nil
}

func (s *service) handleResolveCodeAction(ctx context.Context, args map[string]any) (any, error) {
	action, err := requiredAny(args, "action")
	if err != nil {
		return nil, err
	}

	raw, err := s.client.Request(ctx, "codeAction/resolve", action)
	if err != nil {
		return nil, err
	}

	resolved, err := decodeAny(raw)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"action": resolved,
	}, nil
}

func (s *service) handleFormatting(ctx context.Context, args map[string]any) (any, error) {
	target, snapshot, err := s.resolveDocument(ctx, args)
	if err != nil {
		return nil, err
	}

	tabSize, _, err := optionalInt(args, "tabSize")
	if err != nil {
		return nil, err
	}
	if tabSize <= 0 {
		tabSize = 2
	}
	insertSpaces, err := boolArg(args, "insertSpaces", true)
	if err != nil {
		return nil, err
	}
	applyEdits, err := boolArg(args, "applyEdits", false)
	if err != nil {
		return nil, err
	}

	raw, err := s.client.Request(ctx, "textDocument/formatting", map[string]any{
		"textDocument": map[string]any{"uri": snapshot.URI},
		"options": map[string]any{
			"tabSize":      tabSize,
			"insertSpaces": insertSpaces,
		},
	})
	if err != nil {
		return nil, err
	}

	var edits []lsp.TextEdit
	if err := json.Unmarshal(raw, &edits); err != nil {
		return nil, fmt.Errorf("parse formatting edits: %w", err)
	}

	if !applyEdits {
		return map[string]any{
			"path":  target.RelativePath,
			"count": len(edits),
			"edits": edits,
		}, nil
	}

	updated, err := lsp.ApplyTextEdits(snapshot.Content, edits)
	if err != nil {
		return nil, err
	}
	if err := lsp.WriteFilePreserveMode(target.AbsolutePath, []byte(updated)); err != nil {
		return nil, err
	}
	if _, err := s.client.EnsureDocument(ctx, target.AbsolutePath); err != nil {
		return nil, err
	}

	return map[string]any{
		"path":         target.RelativePath,
		"count":        len(edits),
		"applied":      true,
		"absolutePath": target.AbsolutePath,
	}, nil
}

func (s *service) handleRangeFormatting(ctx context.Context, args map[string]any) (any, error) {
	target, snapshot, err := s.resolveDocument(ctx, args)
	if err != nil {
		return nil, err
	}

	line, character, err := resolvePosition(snapshot.Content, args, true)
	if err != nil {
		return nil, err
	}
	endLine, endCharacter, err := resolveRangeEnd(args, line, character)
	if err != nil {
		return nil, err
	}

	tabSize, _, err := optionalInt(args, "tabSize")
	if err != nil {
		return nil, err
	}
	if tabSize <= 0 {
		tabSize = 2
	}
	insertSpaces, err := boolArg(args, "insertSpaces", true)
	if err != nil {
		return nil, err
	}
	applyEdits, err := boolArg(args, "applyEdits", false)
	if err != nil {
		return nil, err
	}

	raw, err := s.client.Request(ctx, "textDocument/rangeFormatting", map[string]any{
		"textDocument": map[string]any{"uri": snapshot.URI},
		"range": map[string]any{
			"start": map[string]any{"line": line, "character": character},
			"end":   map[string]any{"line": endLine, "character": endCharacter},
		},
		"options": map[string]any{
			"tabSize":      tabSize,
			"insertSpaces": insertSpaces,
		},
	})
	if err != nil {
		return nil, err
	}

	var edits []lsp.TextEdit
	if err := json.Unmarshal(raw, &edits); err != nil {
		return nil, fmt.Errorf("parse range formatting edits: %w", err)
	}

	if !applyEdits {
		return map[string]any{
			"path":  target.RelativePath,
			"count": len(edits),
			"edits": edits,
			"range": map[string]any{
				"start": map[string]any{"line": line + 1, "character": character},
				"end":   map[string]any{"line": endLine + 1, "character": endCharacter},
			},
		}, nil
	}

	updated, err := lsp.ApplyTextEdits(snapshot.Content, edits)
	if err != nil {
		return nil, err
	}
	if err := lsp.WriteFilePreserveMode(target.AbsolutePath, []byte(updated)); err != nil {
		return nil, err
	}
	if _, err := s.client.EnsureDocument(ctx, target.AbsolutePath); err != nil {
		return nil, err
	}

	return map[string]any{
		"path":         target.RelativePath,
		"count":        len(edits),
		"applied":      true,
		"absolutePath": target.AbsolutePath,
		"range": map[string]any{
			"start": map[string]any{"line": line + 1, "character": character},
			"end":   map[string]any{"line": endLine + 1, "character": endCharacter},
		},
	}, nil
}

func (s *service) handleOnTypeFormatting(ctx context.Context, args map[string]any) (any, error) {
	target, snapshot, err := s.resolveDocument(ctx, args)
	if err != nil {
		return nil, err
	}

	line, character, err := resolvePosition(snapshot.Content, args, true)
	if err != nil {
		return nil, err
	}
	ch, err := requiredString(args, "ch")
	if err != nil {
		return nil, err
	}

	tabSize, _, err := optionalInt(args, "tabSize")
	if err != nil {
		return nil, err
	}
	if tabSize <= 0 {
		tabSize = 2
	}
	insertSpaces, err := boolArg(args, "insertSpaces", true)
	if err != nil {
		return nil, err
	}
	applyEdits, err := boolArg(args, "applyEdits", false)
	if err != nil {
		return nil, err
	}

	raw, err := s.client.Request(ctx, "textDocument/onTypeFormatting", map[string]any{
		"textDocument": map[string]any{"uri": snapshot.URI},
		"position":     map[string]any{"line": line, "character": character},
		"ch":           ch,
		"options": map[string]any{
			"tabSize":      tabSize,
			"insertSpaces": insertSpaces,
		},
	})
	if err != nil {
		return nil, err
	}

	var edits []lsp.TextEdit
	if err := json.Unmarshal(raw, &edits); err != nil {
		return nil, fmt.Errorf("parse on-type formatting edits: %w", err)
	}

	if !applyEdits {
		return map[string]any{
			"path":     target.RelativePath,
			"count":    len(edits),
			"edits":    edits,
			"position": map[string]any{"line": line + 1, "character": character},
			"ch":       ch,
		}, nil
	}

	updated, err := lsp.ApplyTextEdits(snapshot.Content, edits)
	if err != nil {
		return nil, err
	}
	if err := lsp.WriteFilePreserveMode(target.AbsolutePath, []byte(updated)); err != nil {
		return nil, err
	}
	if _, err := s.client.EnsureDocument(ctx, target.AbsolutePath); err != nil {
		return nil, err
	}

	return map[string]any{
		"path":         target.RelativePath,
		"count":        len(edits),
		"applied":      true,
		"absolutePath": target.AbsolutePath,
		"position":     map[string]any{"line": line + 1, "character": character},
		"ch":           ch,
	}, nil
}

func (s *service) handleRename(ctx context.Context, args map[string]any) (any, error) {
	target, snapshot, err := s.resolveDocument(ctx, args)
	if err != nil {
		return nil, err
	}

	line, character, err := resolvePosition(snapshot.Content, args, true)
	if err != nil {
		return nil, err
	}
	newName, err := requiredString(args, "newName")
	if err != nil {
		return nil, err
	}
	applyEdits, err := boolArg(args, "applyEdits", true)
	if err != nil {
		return nil, err
	}

	raw, err := s.client.Request(ctx, "textDocument/rename", map[string]any{
		"textDocument": map[string]any{"uri": snapshot.URI},
		"position":     map[string]any{"line": line, "character": character},
		"newName":      newName,
	})
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(string(raw)) == "" || strings.TrimSpace(string(raw)) == "null" {
		return map[string]any{
			"path":    target.RelativePath,
			"applied": false,
			"message": "No workspace edit returned by language server.",
		}, nil
	}

	var edit lsp.WorkspaceEdit
	if err := json.Unmarshal(raw, &edit); err != nil {
		return nil, fmt.Errorf("parse rename workspace edit: %w", err)
	}

	if !applyEdits {
		return map[string]any{
			"path":          target.RelativePath,
			"applied":       false,
			"workspaceEdit": edit,
		}, nil
	}

	summary, err := lsp.ApplyWorkspaceEdit(target.RootDir, edit)
	if err != nil {
		return nil, err
	}

	for _, file := range summary.Files {
		if _, err := s.client.EnsureDocument(ctx, file.Path); err != nil {
			s.logToolWarning("EnsureDocument failed after rename (path=%q): %v", file.Path, err)
		}
	}

	return map[string]any{
		"path":    target.RelativePath,
		"applied": true,
		"summary": summary,
	}, nil
}

func (s *service) handlePrepareRename(ctx context.Context, args map[string]any) (any, error) {
	target, snapshot, err := s.resolveDocument(ctx, args)
	if err != nil {
		return nil, err
	}

	line, character, err := resolvePosition(snapshot.Content, args, true)
	if err != nil {
		return nil, err
	}

	raw, err := s.client.Request(ctx, "textDocument/prepareRename", map[string]any{
		"textDocument": map[string]any{"uri": snapshot.URI},
		"position":     map[string]any{"line": line, "character": character},
	})
	if err != nil {
		return nil, err
	}

	result, err := decodeAny(raw)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"path":      target.RelativePath,
		"position":  map[string]any{"line": line + 1, "character": character},
		"available": result != nil,
		"result":    result,
	}, nil
}

func (s *service) handleExecuteCommand(ctx context.Context, args map[string]any) (any, error) {
	command, err := requiredString(args, "command")
	if err != nil {
		return nil, err
	}

	rawArguments, ok := args["arguments"]
	if !ok {
		rawArguments = []any{}
	}
	arguments, ok := rawArguments.([]any)
	if !ok {
		return nil, fmt.Errorf("arguments must be an array")
	}

	result, err := s.client.ExecuteCommand(ctx, command, arguments)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"command":   command,
		"arguments": arguments,
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
	rootAbs, err := filepath.Abs(rootDir)
	if err != nil {
		return documentTarget{}, lsp.DocumentSnapshot{}, err
	}
	rootDir = filepath.Clean(rootAbs)

	relativePath, err := requiredString(args, "relativePath")
	if err != nil {
		return documentTarget{}, lsp.DocumentSnapshot{}, err
	}

	var absolutePath string
	if filepath.IsAbs(relativePath) {
		absolutePath = filepath.Clean(relativePath)
	} else {
		absolutePath = filepath.Clean(filepath.Join(rootDir, relativePath))
	}
	rel, err := filepath.Rel(rootDir, absolutePath)
	if err != nil {
		return documentTarget{}, lsp.DocumentSnapshot{}, fmt.Errorf("resolve relativePath against root: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return documentTarget{}, lsp.DocumentSnapshot{}, fmt.Errorf("relativePath escapes root directory")
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

func resolveRangeEnd(args map[string]any, line int, character int) (int, int, error) {
	endLine := line
	if v, ok, err := optionalInt(args, "endLine"); err != nil {
		return 0, 0, err
	} else if ok {
		endLine = v - 1
	}
	endCharacter := character
	if v, ok, err := optionalInt(args, "endCharacter"); err != nil {
		return 0, 0, err
	} else if ok {
		endCharacter = v
	}
	if endLine < line || (endLine == line && endCharacter < character) {
		return 0, 0, fmt.Errorf("invalid range: end before start")
	}
	return endLine, endCharacter, nil
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

func requiredAny(args map[string]any, key string) (any, error) {
	raw, ok := args[key]
	if !ok {
		return nil, fmt.Errorf("%s is required", key)
	}
	return raw, nil
}

func optionalString(args map[string]any, key string) string {
	raw, ok := args[key]
	if !ok {
		return ""
	}
	if value, ok := raw.(string); ok {
		return value
	}
	return ""
}

func optionalArrayArg(args map[string]any, key string) ([]any, error) {
	raw, ok := args[key]
	if !ok {
		return []any{}, nil
	}
	value, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("%s must be an array", key)
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
	case json.Number:
		n, err := v.Int64()
		if err != nil {
			return 0, false, fmt.Errorf("%s must be an integer", key)
		}
		return int(n), true, nil
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

func boolArg(args map[string]any, key string, defaultValue bool) (bool, error) {
	raw, ok := args[key]
	if !ok {
		return defaultValue, nil
	}
	switch v := raw.(type) {
	case bool:
		return v, nil
	case string:
		n, err := strconv.ParseBool(strings.TrimSpace(v))
		if err != nil {
			return false, fmt.Errorf("%s must be boolean", key)
		}
		return n, nil
	default:
		return false, fmt.Errorf("%s must be boolean", key)
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

func extractDiagnosticsFromPullReport(report any) []any {
	root, ok := report.(map[string]any)
	if !ok {
		return nil
	}
	var out []any
	if items, ok := root["items"].([]any); ok {
		out = append(out, items...)
	}
	if related, ok := root["relatedDocuments"].(map[string]any); ok {
		keys := make([]string, 0, len(related))
		for key := range related {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			doc, ok := related[key].(map[string]any)
			if !ok {
				continue
			}
			if items, ok := doc["items"].([]any); ok {
				out = append(out, items...)
			}
		}
	}
	return out
}

func extractDiagnosticsFromWorkspaceReport(report any) []any {
	root, ok := report.(map[string]any)
	if !ok {
		return nil
	}

	reports, ok := root["items"].([]any)
	if !ok {
		return nil
	}

	out := make([]any, 0)
	for _, item := range reports {
		docReport, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if diagnostics, ok := docReport["items"].([]any); ok {
			out = append(out, diagnostics...)
		}
		if related, ok := docReport["relatedDocuments"].(map[string]any); ok {
			keys := make([]string, 0, len(related))
			for key := range related {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				relatedDoc, ok := related[key].(map[string]any)
				if !ok {
					continue
				}
				if diagnostics, ok := relatedDoc["items"].([]any); ok {
					out = append(out, diagnostics...)
				}
			}
		}
	}
	return out
}

func supportsTopLevelCapability(caps map[string]json.RawMessage, key string) bool {
	raw, ok := caps[key]
	if !ok {
		return false
	}
	return rawCapabilityEnabled(raw)
}

func supportsCapabilityField(caps map[string]json.RawMessage, key string, field string) bool {
	raw, ok := caps[key]
	if !ok {
		return false
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(raw, &decoded); err != nil {
		// capability の JSON が不正な場合は false を返す
		return false
	}
	fieldRaw, ok := decoded[field]
	if !ok {
		return false
	}
	return rawCapabilityEnabled(fieldRaw)
}

func rawCapabilityEnabled(raw json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(raw))
	return trimmed != "" && trimmed != "false" && trimmed != "null"
}

func sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func emptySchema() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
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

func filePositionSchema(includeCharacterRequired bool) map[string]any {
	required := []string{"relativePath"}
	if includeCharacterRequired {
		required = append(required, "line", "character")
	}
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
		},
		"required": required,
	}
}

func filePositionWithContextSchema() map[string]any {
	schema := filePositionSchema(false)
	props := schema["properties"].(map[string]any)
	props["before"] = map[string]any{
		"type":        "integer",
		"description": "Preview lines before the definition line.",
	}
	props["after"] = map[string]any{
		"type":        "integer",
		"description": "Preview lines after the definition line.",
	}
	return schema
}

func referencesSchema() map[string]any {
	schema := filePositionWithContextSchema()
	props := schema["properties"].(map[string]any)
	props["includeDeclaration"] = map[string]any{
		"type":        "boolean",
		"description": "Include declaration locations in results.",
	}
	return schema
}

func workspaceSymbolSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "Workspace symbol query.",
			},
		},
		"required": []string{"query"},
	}
}

func resolveWorkspaceSymbolSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"symbol": map[string]any{
				"type":        "object",
				"description": "Workspace symbol object returned by workspace/symbol.",
			},
		},
		"required": []string{"symbol"},
	}
}

func completionSchema() map[string]any {
	schema := filePositionSchema(false)
	props := schema["properties"].(map[string]any)
	props["maxItems"] = map[string]any{
		"type":        "integer",
		"description": "Max completion items to return.",
	}
	return schema
}

func resolveCompletionItemSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"item": map[string]any{
				"type":        "object",
				"description": "Completion item object returned by textDocument/completion.",
			},
		},
		"required": []string{"item"},
	}
}

func diagnosticsSchema() map[string]any {
	schema := fileOnlySchema()
	props := schema["properties"].(map[string]any)
	props["waitMs"] = map[string]any{
		"type":        "integer",
		"description": "Wait time for push diagnostics fallback.",
	}
	props["usePull"] = map[string]any{
		"type":        "boolean",
		"description": "Try pull diagnostics first when supported.",
	}
	return schema
}

func workspaceDiagnosticsSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"identifier": map[string]any{
				"type":        "string",
				"description": "Optional identifier for workspace diagnostics request.",
			},
			"previousResultIds": map[string]any{
				"type":        "array",
				"description": "Optional previous result ids for incremental workspace diagnostics.",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"uri":   map[string]any{"type": "string", "description": "Document URI."},
						"value": map[string]any{"type": "string", "description": "Previous result id."},
					},
				},
			},
		},
	}
}

func codeActionSchema() map[string]any {
	schema := filePositionSchema(false)
	props := schema["properties"].(map[string]any)
	props["endLine"] = map[string]any{
		"type":        "integer",
		"description": "End line (1-based). Defaults to start line.",
	}
	props["endCharacter"] = map[string]any{
		"type":        "integer",
		"description": "End character (0-based UTF-16). Defaults to start character.",
	}
	return schema
}

func resolveCodeActionSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "object",
				"description": "Code action object returned by textDocument/codeAction.",
			},
		},
		"required": []string{"action"},
	}
}

func formattingSchema() map[string]any {
	schema := fileOnlySchema()
	props := schema["properties"].(map[string]any)
	props["tabSize"] = map[string]any{
		"type":        "integer",
		"description": "Tab size for formatter options.",
	}
	props["insertSpaces"] = map[string]any{
		"type":        "boolean",
		"description": "Formatter insertSpaces option.",
	}
	props["applyEdits"] = map[string]any{
		"type":        "boolean",
		"description": "Apply edits to file when true.",
	}
	return schema
}

func rangeFormattingSchema() map[string]any {
	schema := filePositionSchema(false)
	props := schema["properties"].(map[string]any)
	props["endLine"] = map[string]any{
		"type":        "integer",
		"description": "End line (1-based). Defaults to start line.",
	}
	props["endCharacter"] = map[string]any{
		"type":        "integer",
		"description": "End character (0-based UTF-16). Defaults to start character.",
	}
	props["tabSize"] = map[string]any{
		"type":        "integer",
		"description": "Tab size for formatter options.",
	}
	props["insertSpaces"] = map[string]any{
		"type":        "boolean",
		"description": "Formatter insertSpaces option.",
	}
	props["applyEdits"] = map[string]any{
		"type":        "boolean",
		"description": "Apply edits to file when true.",
	}
	return schema
}

func onTypeFormattingSchema() map[string]any {
	schema := filePositionSchema(false)
	props := schema["properties"].(map[string]any)
	props["ch"] = map[string]any{
		"type":        "string",
		"description": "Typed character that triggered formatting.",
	}
	props["tabSize"] = map[string]any{
		"type":        "integer",
		"description": "Tab size for formatter options.",
	}
	props["insertSpaces"] = map[string]any{
		"type":        "boolean",
		"description": "Formatter insertSpaces option.",
	}
	props["applyEdits"] = map[string]any{
		"type":        "boolean",
		"description": "Apply edits to file when true.",
	}
	schema["required"] = []string{"relativePath", "ch"}
	return schema
}

func renameSchema() map[string]any {
	schema := filePositionSchema(false)
	props := schema["properties"].(map[string]any)
	props["newName"] = map[string]any{
		"type":        "string",
		"description": "New symbol name.",
	}
	props["applyEdits"] = map[string]any{
		"type":        "boolean",
		"description": "Apply returned workspace edit to files when true.",
	}
	schema["required"] = []string{"relativePath", "newName"}
	return schema
}

func executeCommandSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{
				"type":        "string",
				"description": "Command name for workspace/executeCommand.",
			},
			"arguments": map[string]any{
				"type":        "array",
				"description": "Arguments for workspace/executeCommand.",
				"items":       map[string]any{},
			},
		},
		"required": []string{"command"},
	}
}
