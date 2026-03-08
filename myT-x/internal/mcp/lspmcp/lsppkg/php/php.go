// Package php は PHP 向けの MCP 拡張ツールを提供する。
package php

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
	phpReferenceURL1 = "https://github.com/HvyIndustries/crane"
	phpReferenceURL2 = "https://github.com/bmewburn/vscode-intelephense"
	phpReferenceURL3 = "https://github.com/felixfbecker/php-language-server"
	phpReferenceURL4 = "https://github.com/serenata-php/serenata"
	phpReferenceURL5 = "https://github.com/phan/phan"
	phpReferenceURL6 = "https://github.com/phpactor/phpactor"
)

// BuildTools は PHP 言語サーバー向けの拡張ツールを構築する。
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
			Name:        "php_list_extension_commands",
			Description: triadDescription("Inspect workspace commands advertised via executeCommandProvider.commands by the connected PHP language server", "none", "read"),
			InputSchema: emptySchema(),
			Handler:     svc.handleListCommands,
		},
		{
			Name:        "php_execute_extension_command",
			Description: triadDescription("Run one workspace command advertised by the connected PHP language server", "command, arguments?", "exec"),
			InputSchema: executeCommandSchema(),
			Handler:     svc.handleExecuteCommand,
		},
	}
}

// Matches は設定されたコマンド/引数が PHP 言語サーバーを示す場合に true を返す。
func Matches(command string, args []string) bool {
	if looksLikePHPServer(command) {
		return true
	}

	if slices.ContainsFunc(args, looksLikePHPServer) {
		return true
	}

	if looksLikePHPRuntime(command) && referencesPHPServer(args) {
		return true
	}

	if looksLikeNode(command) && referencesPHPNodeServer(args) {
		return true
	}

	if looksLikeNpx(command) && referencesPHPNodeServer(args) {
		return true
	}

	if looksLikePHPActor(command) && referencesLanguageServerMode(args) {
		return true
	}

	if slices.ContainsFunc(args, looksLikePHPActor) && referencesLanguageServerMode(args) {
		return true
	}

	if looksLikePhanCLI(command) && referencesPhanLanguageServerMode(args) {
		return true
	}

	if slices.ContainsFunc(args, looksLikePhanCLI) && referencesPhanLanguageServerMode(args) {
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
		when := describeCapabilityWhen(name, "PHP")
		argsHint := describeCapabilityArgs(name, "PHP")
		effect := "exec"
		list = append(list, map[string]any{
			"name":        name,
			"when":        when,
			"args":        argsHint,
			"effect":      effect,
			"description": describeCapabilityCommand(name, "PHP"),
			"available":   true,
			"source":      "server-capabilities",
		})
	}

	return map[string]any{
		"lsp":               "php",
		"language":          "PHP",
		"root":              s.rootDir,
		"commands":          list,
		"availableCommands": commands,
		"count":             len(commands),
		"references": []string{
			phpReferenceURL1,
			phpReferenceURL2,
			phpReferenceURL3,
			phpReferenceURL4,
			phpReferenceURL5,
			phpReferenceURL6,
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

	when := describeCapabilityWhen(command, "PHP")
	argsHint := describeCapabilityArgs(command, "PHP")
	effect := "exec"

	availableCommands, err := s.availableCommands()
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"lsp":               "php",
		"root":              s.rootDir,
		"command":           command,
		"commandGuide":      map[string]any{"when": when, "args": argsHint, "effect": effect, "description": describeCapabilityCommand(command, "PHP")},
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

func looksLikePHPServer(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	switch base {
	case "crane", "crane.exe", "crane.cmd", "crane.bat",
		"intelephense", "intelephense.exe", "intelephense.cmd", "intelephense.bat",
		"php-language-server", "php-language-server.php", "php-language-server.exe", "php-language-server.cmd",
		"serenata", "serenata.exe", "serenata.phar",
		"phan-language-server", "phan-language-server.php", "phan-language-server.exe",
		"phpactor-language-server", "phpactor-language-server.php", "phpactor-language-server.exe":
		return true
	default:
		return strings.Contains(base, "crane") ||
			strings.Contains(base, "intelephense") ||
			strings.Contains(base, "php-language-server") ||
			strings.Contains(base, "serenata") ||
			strings.Contains(base, "phan-language-server") ||
			strings.Contains(base, "phpactor-language-server") ||
			strings.Contains(normalized, "hvyindustries/crane") ||
			strings.Contains(normalized, "hvyindustries\\crane") ||
			strings.Contains(normalized, "vscode-intelephense")
	}
}

func looksLikePHPActor(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	return base == "phpactor" || base == "phpactor.exe" || base == "phpactor.phar"
}

func looksLikePhanCLI(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	return base == "phan" || base == "phan.exe" || base == "phan.phar"
}

func looksLikePHPRuntime(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	return base == "php" || base == "php.exe"
}

func looksLikeNode(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	return base == "node" || base == "node.exe"
}

func looksLikeNpx(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	return base == "npx" || base == "npx.cmd" || base == "npx.exe"
}

func referencesPHPServer(args []string) bool {
	if referencesLanguageServerMode(args) && slices.ContainsFunc(args, looksLikePHPActor) {
		return true
	}
	if referencesPhanLanguageServerMode(args) {
		return true
	}

	return slices.ContainsFunc(args, func(arg string) bool {
		normalized := strings.ToLower(strings.TrimSpace(arg))
		return looksLikePHPServer(normalized) ||
			strings.Contains(normalized, "intelephense") ||
			strings.Contains(normalized, "vscode-intelephense") ||
			strings.Contains(normalized, "php-language-server") ||
			strings.Contains(normalized, "felixfbecker/php-language-server") ||
			strings.Contains(normalized, "felixfbecker\\php-language-server") ||
			strings.Contains(normalized, "serenata") ||
			strings.Contains(normalized, "serenata-php") ||
			strings.Contains(normalized, "hvyindustries/crane") ||
			strings.Contains(normalized, "hvyindustries\\crane")
	})
}

func referencesPHPNodeServer(args []string) bool {
	return slices.ContainsFunc(args, func(arg string) bool {
		normalized := strings.ToLower(strings.TrimSpace(arg))
		return strings.Contains(normalized, "intelephense") ||
			strings.Contains(normalized, "vscode-intelephense") ||
			strings.Contains(normalized, "hvyindustries/crane") ||
			strings.Contains(normalized, "hvyindustries\\crane") ||
			strings.Contains(normalized, "php-language-server") ||
			strings.Contains(normalized, "serenata")
	})
}

func referencesLanguageServerMode(args []string) bool {
	for _, arg := range args {
		normalized := strings.ToLower(strings.TrimSpace(arg))
		if normalized == "language-server" || normalized == "language_server" {
			return true
		}
		if strings.HasPrefix(normalized, "--language-server") ||
			strings.HasPrefix(normalized, "language-server=") ||
			strings.Contains(normalized, "language-server") {
			return true
		}
	}
	return false
}

func referencesPhanLanguageServerMode(args []string) bool {
	for _, arg := range args {
		normalized := strings.ToLower(strings.TrimSpace(arg))
		if strings.HasPrefix(normalized, "--language-server") {
			return true
		}
		if strings.Contains(normalized, "phan/language_server") || strings.Contains(normalized, "phan\\language_server") {
			return true
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
				"description": "PHP language-server command name to execute (see php_list_extension_commands).",
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
