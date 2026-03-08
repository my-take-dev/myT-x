// Package liquid は Liquid 向けの MCP 拡張ツールを提供する。
package liquid

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

const liquidThemeCheckReferenceURL = "https://github.com/Shopify/theme-check"

// BuildTools は Liquid 言語サーバー向けの拡張ツールを構築する。
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
			Name:        "liquid_list_extension_commands",
			Description: triadDescription("Inspect workspace commands advertised via executeCommandProvider.commands by the connected Liquid language server", "none", "read"),
			InputSchema: emptySchema(),
			Handler:     svc.handleListCommands,
		},
		{
			Name:        "liquid_execute_extension_command",
			Description: triadDescription("Run one workspace command advertised by the connected Liquid language server", "command, arguments?", "exec"),
			InputSchema: executeCommandSchema(),
			Handler:     svc.handleExecuteCommand,
		},
	}
}

// Matches は設定されたコマンド/引数が Liquid 言語サーバーを示す場合に true を返す。
func Matches(command string, args []string) bool {
	if looksLikeThemeCheckServer(command) {
		return true
	}

	if slices.ContainsFunc(args, looksLikeThemeCheckServer) {
		return true
	}

	if looksLikeThemeCheckCLI(command) && referencesThemeCheckSubcommand(args) {
		return true
	}

	if looksLikeRuby(command) && referencesThemeCheckServer(args) {
		return true
	}

	if looksLikeBundle(command) && referencesThemeCheckServer(args) {
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
		when, argsHint, effect := capabilityCommandGuide(name, "Liquid")
		list = append(list, map[string]any{
			"name":        name,
			"when":        when,
			"args":        argsHint,
			"effect":      effect,
			"description": triadDescription(when, argsHint, effect),
			"available":   true,
			"source":      "server-capabilities",
		})
	}

	return map[string]any{
		"lsp":               "liquid",
		"language":          "Liquid",
		"root":              s.rootDir,
		"commands":          list,
		"availableCommands": commands,
		"count":             len(commands),
		"references":        []string{liquidThemeCheckReferenceURL},
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

	when, argsHint, effect := capabilityCommandGuide(command, "Liquid")

	availableCommands, err := s.availableCommands()
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"lsp":               "liquid",
		"root":              s.rootDir,
		"command":           command,
		"arguments":         arguments,
		"result":            result,
		"commandGuide":      map[string]any{"when": when, "args": argsHint, "effect": effect, "description": triadDescription(when, argsHint, effect)},
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

func looksLikeThemeCheckServer(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	switch base {
	case "theme-check-language-server", "theme-check-language-server.exe", "theme-check-language-server.cmd",
		"theme_check_language_server", "theme_check_language_server.exe",
		"theme-check-lsp", "theme-check-lsp.exe",
		"theme_check_lsp", "theme_check_lsp.exe":
		return true
	default:
		return strings.Contains(base, "theme-check-language-server") ||
			strings.Contains(base, "theme_check_language_server") ||
			strings.Contains(base, "theme-check-lsp") ||
			strings.Contains(base, "theme_check_lsp") ||
			strings.Contains(normalized, "theme-check-language-server") ||
			strings.Contains(normalized, "theme_check_language_server") ||
			strings.Contains(normalized, "theme-check-lsp") ||
			strings.Contains(normalized, "theme_check_lsp")
	}
}

func looksLikeThemeCheckCLI(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	switch base {
	case "theme-check", "theme-check.exe", "theme-check.cmd",
		"theme_check", "theme_check.exe":
		return true
	default:
		return false
	}
}

func looksLikeRuby(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	switch base {
	case "ruby", "ruby.exe", "ruby3", "ruby3.exe":
		return true
	default:
		return strings.HasPrefix(base, "ruby")
	}
}

func looksLikeBundle(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	return base == "bundle" || base == "bundle.exe" || base == "bundle.cmd"
}

func referencesThemeCheckSubcommand(args []string) bool {
	for _, arg := range args {
		normalized := strings.ToLower(strings.TrimSpace(arg))
		if normalized == "language-server" || normalized == "language_server" || normalized == "lsp" {
			return true
		}
	}
	return false
}

func referencesThemeCheckServer(args []string) bool {
	hasThemeCheck := false
	hasLSPSubcommand := false

	for _, arg := range args {
		normalized := strings.ToLower(strings.TrimSpace(arg))
		if strings.Contains(normalized, "theme-check-language-server") ||
			strings.Contains(normalized, "theme_check_language_server") ||
			strings.Contains(normalized, "theme-check-lsp") ||
			strings.Contains(normalized, "theme_check_lsp") {
			return true
		}

		if strings.Contains(normalized, "theme-check") || strings.Contains(normalized, "theme_check") {
			hasThemeCheck = true
		}

		if normalized == "language-server" || normalized == "language_server" || normalized == "lsp" {
			hasLSPSubcommand = true
		}
	}

	return hasThemeCheck && hasLSPSubcommand
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
				"description": "Liquid language-server command name to execute (see liquid_list_extension_commands).",
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

func describeCapabilityCommand(name string, language string) string {
	when, args, effect := capabilityCommandGuide(name, language)
	return triadDescription(when, args, effect)
}

func capabilityCommandGuide(name string, language string) (string, string, string) {
	context := strings.TrimSpace(language)
	if context == "" {
		context = "language"
	}
	commandName := strings.TrimSpace(name)
	when := "Run workspace commands only when the connected " + context + " language server advertises them; semantics and arguments are server-specific."
	if commandName != "" {
		when = "Run workspace command " + commandName + " only when the connected " + context + " language server advertises it; semantics and arguments are server-specific."
	}
	args := "arguments array expected by the connected " + context + " language server for the selected command"
	if commandName != "" {
		args = "arguments array expected by the connected " + context + " language server for " + commandName
	}
	effect := "exec"
	return when, args, effect
}

func triadDescription(when string, args string, effect string) string {
	return "when: " + strings.TrimSpace(when) + " args: " + strings.TrimSpace(args) + " effect: " + strings.TrimSpace(effect) + "."
}
