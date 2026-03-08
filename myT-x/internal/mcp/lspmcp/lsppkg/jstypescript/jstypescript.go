// Package jstypescript は JavaScript-Typescript 向けの MCP 拡張ツールを提供する。
package jstypescript

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
	jsTypeScriptReferenceURL1 = "https://github.com/sourcegraph/javascript-typescript-langserver"
	jsTypeScriptReferenceURL2 = "https://github.com/biomejs/biome"
)

// BuildTools は JavaScript-Typescript 言語サーバー向けの拡張ツールを構築する。
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
			Name:        "jstypescript_list_extension_commands",
			Description: triadDescription("Inspect workspace commands advertised via executeCommandProvider.commands by the connected JavaScript-Typescript language server", "none", "read"),
			InputSchema: emptySchema(),
			Handler:     svc.handleListCommands,
		},
		{
			Name:        "jstypescript_execute_extension_command",
			Description: triadDescription("Run one workspace command advertised by the connected JavaScript-Typescript language server", "command, arguments?", "exec"),
			InputSchema: executeCommandSchema(),
			Handler:     svc.handleExecuteCommand,
		},
	}
}

// Matches は設定されたコマンド/引数が JavaScript-Typescript 言語サーバーを示す場合に true を返す。
func Matches(command string, args []string) bool {
	if looksLikeJavaScriptTypeScriptServer(command) || looksLikeBiomeLSPServer(command) {
		return true
	}

	if slices.ContainsFunc(args, looksLikeJavaScriptTypeScriptServer) || slices.ContainsFunc(args, looksLikeBiomeLSPServer) {
		return true
	}

	if looksLikeNode(command) && referencesJavaScriptTypeScriptServer(args) {
		return true
	}

	if looksLikeBiomeCLI(command) && referencesBiomeLSP(args) {
		return true
	}

	if slices.ContainsFunc(args, looksLikeBiomeCLI) && referencesBiomeLSP(args) {
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
		when, argsHint, effect := capabilityCommandGuide(name, "JavaScript-Typescript")
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
		"lsp":               "jstypescript",
		"language":          "JavaScript-Typescript",
		"root":              s.rootDir,
		"commands":          list,
		"availableCommands": commands,
		"count":             len(commands),
		"references":        []string{jsTypeScriptReferenceURL1, jsTypeScriptReferenceURL2},
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

	when, argsHint, effect := capabilityCommandGuide(command, "JavaScript-Typescript")

	availableCommands, err := s.availableCommands()
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"lsp":               "jstypescript",
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

func looksLikeJavaScriptTypeScriptServer(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	switch base {
	case "javascript-typescript-stdio", "javascript-typescript-stdio.exe",
		"javascript-typescript-langserver", "javascript-typescript-langserver.exe",
		"javascript-typescript-language-server", "javascript-typescript-language-server.exe":
		return true
	default:
		return strings.Contains(base, "javascript-typescript-langserver") ||
			strings.Contains(base, "javascript-typescript-stdio")
	}
}

func looksLikeBiomeLSPServer(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	switch base {
	case "biome_lsp", "biome_lsp.exe",
		"biome-lsp", "biome-lsp.exe":
		return true
	default:
		return strings.Contains(base, "biome_lsp") || strings.Contains(base, "biome-lsp")
	}
}

func looksLikeBiomeCLI(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	return base == "biome" || base == "biome.exe"
}

func looksLikeNode(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	return base == "node" || base == "node.exe"
}

func referencesJavaScriptTypeScriptServer(args []string) bool {
	return slices.ContainsFunc(args, func(arg string) bool {
		normalized := strings.ToLower(strings.TrimSpace(arg))
		return strings.Contains(normalized, "javascript-typescript-langserver") ||
			strings.Contains(normalized, "javascript-typescript-stdio") ||
			strings.Contains(normalized, "sourcegraph/javascript-typescript-langserver") ||
			strings.Contains(normalized, "sourcegraph\\javascript-typescript-langserver")
	})
}

func referencesBiomeLSP(args []string) bool {
	for _, arg := range args {
		normalized := strings.ToLower(strings.TrimSpace(arg))
		if strings.Contains(normalized, "biome_lsp") || strings.Contains(normalized, "biome-lsp") {
			return true
		}
		if normalized == "lsp" || normalized == "lsp-proxy" || strings.Contains(normalized, "lsp-proxy") {
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
				"description": "JavaScript-Typescript language-server command name to execute (see jstypescript_list_extension_commands).",
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
