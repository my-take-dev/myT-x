// Package protobuf は Protocol Buffers 向けの MCP 拡張ツールを提供する。
package protobuf

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
	protolsReferenceURL                = "https://github.com/coder3101/protols"
	protobufLanguageServerReferenceURL = "https://github.com/lasorda/protobuf-language-server"
	bufLanguageServerReferenceURL      = "https://github.com/bufbuild/buf"
)

// BuildTools は Protocol Buffers 言語サーバー向けの拡張ツールを構築する。
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
			Name:        "protobuf_list_extension_commands",
			Description: triadDescription("Inspect workspace commands advertised via executeCommandProvider.commands by the connected Protocol Buffers language server", "none", "read"),
			InputSchema: emptySchema(),
			Handler:     svc.handleListCommands,
		},
		{
			Name:        "protobuf_execute_extension_command",
			Description: triadDescription("Run one workspace command advertised by the connected Protocol Buffers language server", "command, arguments?", "exec"),
			InputSchema: executeCommandSchema(),
			Handler:     svc.handleExecuteCommand,
		},
	}
}

// Matches は設定されたコマンド/引数が Protocol Buffers 言語サーバーを示す場合に true を返す。
func Matches(command string, args []string) bool {
	if looksLikeProtobufServer(command) {
		return true
	}

	if slices.ContainsFunc(args, looksLikeProtobufServer) {
		return true
	}

	if looksLikeGo(command) && referencesProtobufGoServer(args) {
		return true
	}

	if looksLikeCargo(command) && referencesProtobufRustServer(args) {
		return true
	}

	if looksLikeBuf(command) && referencesBufLanguageServer(args) {
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
	for _, name := range commands {
		when := describeCapabilityWhen(name, "Protocol Buffers")
		argsHint := describeCapabilityArgs(name, "Protocol Buffers")
		effect := "exec"
		list = append(list, map[string]any{
			"name":        name,
			"when":        when,
			"args":        argsHint,
			"effect":      effect,
			"description": describeCapabilityCommand(name, "Protocol Buffers"),
			"available":   true,
			"source":      "server-capabilities",
		})
	}

	return map[string]any{
		"lsp":               "protobuf",
		"language":          "Protocol Buffers",
		"root":              s.rootDir,
		"commands":          list,
		"availableCommands": commands,
		"count":             len(commands),
		"references": []string{
			protolsReferenceURL,
			protobufLanguageServerReferenceURL,
			bufLanguageServerReferenceURL,
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

	when := describeCapabilityWhen(command, "Protocol Buffers")
	argsHint := describeCapabilityArgs(command, "Protocol Buffers")
	effect := "exec"

	availableCommands, err := s.availableCommands()
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"lsp":               "protobuf",
		"root":              s.rootDir,
		"command":           command,
		"commandGuide":      map[string]any{"when": when, "args": argsHint, "effect": effect, "description": describeCapabilityCommand(command, "Protocol Buffers")},
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

func looksLikeProtobufServer(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	switch base {
	case "protols", "protols.exe",
		"protobuf-language-server", "protobuf-language-server.exe",
		"bufls", "bufls.exe",
		"buf-language-server", "buf-language-server.exe":
		return true
	default:
		return strings.Contains(base, "protols") ||
			strings.Contains(base, "protobuf-language-server") ||
			strings.Contains(base, "bufls") ||
			strings.Contains(base, "buf-language-server") ||
			strings.Contains(normalized, "coder3101/protols") ||
			strings.Contains(normalized, "coder3101\\protols") ||
			strings.Contains(normalized, "lasorda/protobuf-language-server") ||
			strings.Contains(normalized, "lasorda\\protobuf-language-server")
	}
}

func looksLikeGo(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	return base == "go" || base == "go.exe" || base == "go.cmd"
}

func looksLikeCargo(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	return base == "cargo" || base == "cargo.exe" || base == "cargo.cmd"
}

func looksLikeBuf(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	return base == "buf" || base == "buf.exe" || base == "buf.cmd"
}

func referencesProtobufGoServer(args []string) bool {
	return slices.ContainsFunc(args, func(arg string) bool {
		normalized := strings.ToLower(strings.TrimSpace(arg))
		return strings.Contains(normalized, "protobuf-language-server") ||
			strings.Contains(normalized, "lasorda/protobuf-language-server") ||
			strings.Contains(normalized, "lasorda\\protobuf-language-server")
	})
}

func referencesProtobufRustServer(args []string) bool {
	return slices.ContainsFunc(args, func(arg string) bool {
		normalized := strings.ToLower(strings.TrimSpace(arg))
		return strings.Contains(normalized, "protols") ||
			strings.Contains(normalized, "coder3101/protols") ||
			strings.Contains(normalized, "coder3101\\protols")
	})
}

func referencesBufLanguageServer(args []string) bool {
	for _, arg := range args {
		normalized := strings.ToLower(strings.TrimSpace(arg))
		if strings.Contains(normalized, "bufls") ||
			strings.Contains(normalized, "buf-language-server") ||
			(strings.Contains(normalized, "bufbuild/buf") && strings.Contains(normalized, "lsp")) ||
			(strings.Contains(normalized, "bufbuild\\buf") && strings.Contains(normalized, "lsp")) {
			return true
		}
	}

	for i := range args {
		normalized := strings.ToLower(strings.TrimSpace(args[i]))
		if normalized == "lsp" {
			return true
		}
		if normalized == "beta" && i+1 < len(args) {
			next := strings.ToLower(strings.TrimSpace(args[i+1]))
			if next == "lsp" {
				return true
			}
		}
	}

	return false
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
				"description": "Protocol Buffers language-server command name to execute (see protobuf_list_extension_commands).",
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

func triadDescription(when string, args string, effect string) string {
	return "when: " + strings.TrimSpace(when) + " args: " + strings.TrimSpace(args) + " effect: " + strings.TrimSpace(effect) + "."
}

func describeCapabilityWhen(name string, language string) string {
	context := strings.TrimSpace(language)
	if context == "" {
		context = "language"
	}
	commandName := strings.TrimSpace(name)
	if commandName == "" {
		return "Run workspace commands only when the connected " + context + " language server advertises them; semantics and arguments are server-specific."
	}
	return "Run workspace command " + commandName + " only when the connected " + context + " language server advertises it; semantics and arguments are server-specific."
}

func describeCapabilityArgs(name string, language string) string {
	context := strings.TrimSpace(language)
	if context == "" {
		context = "language"
	}
	commandName := strings.TrimSpace(name)
	if commandName == "" {
		return "arguments array expected by the connected " + context + " language server for the selected command"
	}
	return "arguments array expected by the connected " + context + " language server for " + commandName
}

func describeCapabilityCommand(name string, language string) string {
	return triadDescription(describeCapabilityWhen(name, language), describeCapabilityArgs(name, language), "exec")
}
