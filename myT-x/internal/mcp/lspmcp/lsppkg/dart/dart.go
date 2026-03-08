// Package dart は Dart 向けの MCP 拡張ツールを提供する。
package dart

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
	dartReferenceURL  = "https://github.com/dart-lang/sdk"
	dartServerContext = "Dart Analysis Server (dart-lang/sdk)"
)

// BuildTools は Dart 言語サーバー向けの拡張ツールを構築する。
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
			Name:        "dart_list_extension_commands",
			Description: triadDescription("Inspect workspace commands advertised via executeCommandProvider.commands by "+dartServerContext, "none", "read"),
			InputSchema: emptySchema(),
			Handler:     svc.handleListCommands,
		},
		{
			Name:        "dart_execute_extension_command",
			Description: triadDescription("Run one workspace command advertised by "+dartServerContext, "command, arguments?", "exec"),
			InputSchema: executeCommandSchema(),
			Handler:     svc.handleExecuteCommand,
		},
	}
}

// Matches は設定されたコマンド/引数が Dart 言語サーバーを示す場合に true を返す。
func Matches(command string, args []string) bool {
	if looksLikeDartServer(command) {
		return true
	}

	if slices.ContainsFunc(args, looksLikeDartServer) {
		return true
	}

	if looksLikeDart(command) && referencesDartServer(args) {
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
		when := describeCapabilityWhen(name, "Dart")
		argsHint := describeCapabilityArgs(name, "Dart")
		effect := "exec"
		list = append(list, map[string]any{
			"name":        name,
			"when":        when,
			"args":        argsHint,
			"effect":      effect,
			"description": describeCapabilityCommand(name, "Dart"),
			"available":   true,
			"source":      "server-capabilities",
		})
	}

	return map[string]any{
		"lsp":               "dart",
		"language":          "Dart",
		"root":              s.rootDir,
		"commands":          list,
		"availableCommands": commands,
		"count":             len(commands),
		"references":        []string{dartReferenceURL},
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

	when := describeCapabilityWhen(command, "Dart")
	argsHint := describeCapabilityArgs(command, "Dart")
	effect := "exec"

	availableCommands, err := s.availableCommands()
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"lsp":               "dart",
		"root":              s.rootDir,
		"command":           command,
		"commandGuide":      map[string]any{"when": when, "args": argsHint, "effect": effect, "description": describeCapabilityCommand(command, "Dart")},
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

func looksLikeDartServer(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	switch base {
	case "dart-language-server", "dart-language-server.exe",
		"analysis_server.dart.snapshot", "analysis_server.dart.snapshot.exe",
		"analysis-server", "analysis-server.exe":
		return true
	default:
		return strings.Contains(base, "dart-language-server") ||
			strings.Contains(base, "analysis_server.dart.snapshot") ||
			strings.Contains(base, "analysis-server")
	}
}

func looksLikeDart(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	return base == "dart" || base == "dart.exe"
}

func referencesDartServer(args []string) bool {
	return slices.ContainsFunc(args, func(arg string) bool {
		normalized := strings.ToLower(strings.TrimSpace(arg))
		return strings.Contains(normalized, "language-server") ||
			strings.Contains(normalized, "analysis_server.dart.snapshot") ||
			strings.Contains(normalized, "analysis-server") ||
			strings.Contains(normalized, "--lsp")
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
				"description": "Dart language-server command name to execute (see dart_list_extension_commands).",
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
		context = "Dart"
	}
	context = context + " via " + dartServerContext
	commandName := strings.TrimSpace(name)
	if commandName == "" {
		return "Run workspace commands only when " + context + " advertises them; semantics and arguments are server-specific."
	}
	return "Run workspace command " + commandName + " only when " + context + " advertises it; semantics and arguments are server-specific."
}

func describeCapabilityArgs(name string, language string) string {
	context := strings.TrimSpace(language)
	if context == "" {
		context = "Dart"
	}
	context = context + " via " + dartServerContext
	commandName := strings.TrimSpace(name)
	if commandName == "" {
		return "arguments array expected by " + context + " for the selected command"
	}
	return "arguments array expected by " + context + " for " + commandName
}

func describeCapabilityCommand(name string, language string) string {
	return triadDescription(describeCapabilityWhen(name, language), describeCapabilityArgs(name, language), "exec")
}
