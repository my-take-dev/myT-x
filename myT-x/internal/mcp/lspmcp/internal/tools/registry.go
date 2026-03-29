package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"path/filepath"
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

// --- ケイパビリティ検査ハンドラ ---

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

// --- ドキュメント解決・ポジション解析（全ハンドラ共通） ---

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

// --- 引数パースヘルパー（全ハンドラ共通） ---

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

// --- データ変換・ケイパビリティ確認・汎用ユーティリティ ---

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
