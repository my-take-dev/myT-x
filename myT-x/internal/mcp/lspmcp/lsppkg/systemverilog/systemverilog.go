// Package systemverilog は SystemVerilog 向けの MCP 拡張ツールを提供する。
package systemverilog

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"myT-x/internal/mcp/lspmcp/internal/lsp"
	"myT-x/internal/mcp/lspmcp/internal/mcp"
)

const (
	systemVerilogSVLSReferenceURL    = "https://github.com/dalance/svls"
	systemVerilogVeribleReferenceURL = "https://github.com/chipsalliance/verible"
	systemVerilogSlangReferenceURL   = "https://github.com/chipsalliance/slang"
)

// BuildTools は SystemVerilog 言語サーバー向けの拡張ツールを構築する。
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
			Name:        "systemverilog_list_extension_commands",
			Description: triadDescription("Inspect workspace commands advertised via executeCommandProvider.commands by the connected SystemVerilog language server", "none", "read"),
			InputSchema: emptySchema(),
			Handler:     svc.handleListCommands,
		},
		{
			Name:        "systemverilog_execute_extension_command",
			Description: triadDescription("Run one workspace command advertised by the connected SystemVerilog language server", "command, arguments?", "exec"),
			InputSchema: executeCommandSchema(),
			Handler:     svc.handleExecuteCommand,
		},
	}
}

// Matches は設定されたコマンド/引数が SystemVerilog 言語サーバーを示す場合に true を返す。
func Matches(command string, args []string) bool {
	if looksLikeSystemVerilogServer(command) {
		return true
	}

	if slices.ContainsFunc(args, looksLikeSystemVerilogServer) {
		return true
	}

	if looksLikeCargo(command) && referencesSVLSServer(args) {
		return true
	}

	if looksLikeJava(command) && referencesSigasiServer(args) {
		return true
	}

	if looksLikeNode(command) && referencesSystemVerilogServer(args) {
		return true
	}

	return false
}

type service struct {
	client  *lsp.Client
	rootDir string
}

func (s *service) handleListCommands(_ context.Context, _ map[string]any) (any, error) {
	commands, err := s.availableCommands()
	if err != nil {
		return nil, err
	}
	list := make([]map[string]any, 0, len(commands))
	when, argsHint, effect, description := capabilityCommandGuide("SystemVerilog")
	for _, name := range commands {
		list = append(list, map[string]any{
			"name":        name,
			"description": description,
			"when":        when,
			"args":        argsHint,
			"effect":      effect,
			"available":   true,
			"source":      "server-capabilities",
		})
	}

	return map[string]any{
		"lsp":               "systemverilog",
		"language":          "SystemVerilog",
		"root":              s.rootDir,
		"commands":          list,
		"availableCommands": commands,
		"count":             len(commands),
		"references": []string{
			systemVerilogSVLSReferenceURL,
			systemVerilogVeribleReferenceURL,
			systemVerilogSlangReferenceURL,
		},
	}, nil
}

func (s *service) handleExecuteCommand(ctx context.Context, args map[string]any) (any, error) {
	command, err := requiredString(args, "command")
	if err != nil {
		return nil, err
	}

	rawArguments, ok := args["arguments"]
	if !ok || rawArguments == nil {
		rawArguments = []any{}
	}
	arguments, ok := rawArguments.([]any)
	if !ok {
		return nil, fmt.Errorf("arguments must be an array, got %T", rawArguments)
	}

	result, err := s.client.ExecuteCommand(ctx, command, arguments)
	if err != nil {
		return nil, err
	}

	when, argsHint, effect, description := capabilityCommandGuide("SystemVerilog")

	availableCommands, err := s.availableCommands()
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"lsp":               "systemverilog",
		"root":              s.rootDir,
		"command":           command,
		"commandGuide":      map[string]any{"when": when, "args": argsHint, "effect": effect, "description": description},
		"arguments":         arguments,
		"result":            result,
		"availableCommands": availableCommands,
	}, nil
}

func (s *service) availableCommands() ([]string, error) {
	caps := s.client.Capabilities()
	raw, ok := caps["executeCommandProvider"]
	if !ok {
		return nil, nil
	}

	var provider struct {
		Commands []string `json:"commands"`
	}
	if err := json.Unmarshal(raw, &provider); err != nil {
		return nil, fmt.Errorf("parse executeCommandProvider: %w", err)
	}
	sort.Strings(provider.Commands)
	return provider.Commands, nil
}

func looksLikeSystemVerilogServer(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	switch base {
	case "svls", "svls.exe",
		"verible-verilog-ls", "verible-verilog-ls.exe",
		"slang-server", "slang-server.exe",
		"sigasi-language-server", "sigasi-language-server.exe",
		"sigasi-lsp", "sigasi-lsp.exe",
		"systemverilog-language-server", "systemverilog-language-server.exe",
		"svlangserver", "svlangserver.exe":
		return true
	default:
		return strings.Contains(base, "verible-verilog-ls") ||
			strings.Contains(base, "slang-server") ||
			strings.Contains(base, "sigasi-language-server") ||
			strings.Contains(base, "systemverilog-language-server") ||
			strings.Contains(base, "svlangserver")
	}
}

func looksLikeCargo(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	return base == "cargo" || base == "cargo.exe"
}

func looksLikeJava(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	return base == "java" || base == "java.exe"
}

func looksLikeNode(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	return base == "node" || base == "node.exe" || base == "nodejs" || base == "nodejs.exe"
}

func referencesSVLSServer(args []string) bool {
	return slices.ContainsFunc(args, func(arg string) bool {
		normalized := strings.ToLower(strings.TrimSpace(arg))
		normalized = strings.ReplaceAll(normalized, "\\", "/")
		return strings.Contains(normalized, "dalance/svls") ||
			strings.Contains(normalized, "github.com/dalance/svls") ||
			strings.Contains(normalized, "/svls")
	})
}

func referencesSigasiServer(args []string) bool {
	return slices.ContainsFunc(args, func(arg string) bool {
		normalized := strings.ToLower(strings.TrimSpace(arg))
		return strings.Contains(normalized, "sigasi") &&
			(strings.Contains(normalized, "language-server") ||
				strings.Contains(normalized, "lsp") ||
				strings.Contains(normalized, "server"))
	})
}

func referencesSystemVerilogServer(args []string) bool {
	return slices.ContainsFunc(args, func(arg string) bool {
		normalized := strings.ToLower(strings.TrimSpace(arg))
		normalized = strings.ReplaceAll(normalized, "\\", "/")
		return strings.Contains(normalized, "dalance/svls") ||
			strings.Contains(normalized, "svls") ||
			strings.Contains(normalized, "verible-verilog-ls") ||
			strings.Contains(normalized, "chipsalliance/verible") ||
			strings.Contains(normalized, "slang-server") ||
			strings.Contains(normalized, "chipsalliance/slang") ||
			strings.Contains(normalized, "sigasi") ||
			strings.Contains(normalized, "systemverilog-language-server") ||
			strings.Contains(normalized, "svlangserver")
	})
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

func emptySchema() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}

func executeCommandSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{
				"type":        "string",
				"description": "SystemVerilog language-server command name to execute (see systemverilog_list_extension_commands).",
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

func describeCapabilityCommand(_ string, language string) string {
	_, _, _, description := capabilityCommandGuide(language)
	return description
}

func capabilityCommandGuide(language string) (string, string, string, string) {
	context := strings.TrimSpace(language)
	if context == "" {
		context = "language"
	}
	when := "Run workspace commands only when the connected " + context + " language server advertises them; semantics and arguments are server-specific."
	args := "arguments array expected by the connected " + context + " language server for the selected command"
	effect := "exec"
	return when, args, effect, triadDescription(when, args, effect)
}

func triadDescription(when string, args string, effect string) string {
	return "when: " + strings.TrimSpace(when) + " args: " + strings.TrimSpace(args) + " effect: " + strings.TrimSpace(effect) + "."
}
