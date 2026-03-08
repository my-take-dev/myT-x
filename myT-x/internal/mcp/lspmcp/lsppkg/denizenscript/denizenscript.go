// Package denizenscript は DenizenScript 向けの MCP 拡張ツールを提供する。
package denizenscript

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
	denizenscriptReferenceURL  = "https://github.com/DenizenScript/DenizenVSCode"
	denizenscriptServerContext = "DenizenVSCode language server (DenizenScript/DenizenVSCode)"
)

// BuildTools は DenizenScript 言語サーバー向けの拡張ツールを構築する。
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
			Name:        "denizenscript_list_extension_commands",
			Description: triadDescription("Inspect workspace commands advertised via executeCommandProvider.commands by "+denizenscriptServerContext, "none", "read"),
			InputSchema: emptySchema(),
			Handler:     svc.handleListCommands,
		},
		{
			Name:        "denizenscript_execute_extension_command",
			Description: triadDescription("Run one workspace command advertised by "+denizenscriptServerContext, "command, arguments?", "exec"),
			InputSchema: executeCommandSchema(),
			Handler:     svc.handleExecuteCommand,
		},
	}
}

// Matches は設定されたコマンド/引数が DenizenScript 言語サーバーを示す場合に true を返す。
func Matches(command string, args []string) bool {
	if looksLikeDenizenScriptServer(command) {
		return true
	}

	if slices.ContainsFunc(args, looksLikeDenizenScriptServer) {
		return true
	}

	if looksLikeDotnet(command) && referencesDenizenScriptServer(args) {
		return true
	}

	if looksLikeMono(command) && referencesDenizenScriptServer(args) {
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
		when := describeCapabilityWhen(name, "DenizenScript")
		argsHint := describeCapabilityArgs(name, "DenizenScript")
		effect := "exec"
		list = append(list, map[string]any{
			"name":        name,
			"when":        when,
			"args":        argsHint,
			"effect":      effect,
			"description": describeCapabilityCommand(name, "DenizenScript"),
			"available":   true,
			"source":      "server-capabilities",
		})
	}

	return map[string]any{
		"lsp":               "denizenscript",
		"language":          "DenizenScript",
		"root":              s.rootDir,
		"commands":          list,
		"availableCommands": commands,
		"count":             len(commands),
		"references":        []string{denizenscriptReferenceURL},
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

	when := describeCapabilityWhen(command, "DenizenScript")
	argsHint := describeCapabilityArgs(command, "DenizenScript")
	effect := "exec"

	availableCommands, err := s.availableCommands()
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"lsp":               "denizenscript",
		"root":              s.rootDir,
		"command":           command,
		"commandGuide":      map[string]any{"when": when, "args": argsHint, "effect": effect, "description": describeCapabilityCommand(command, "DenizenScript")},
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

func looksLikeDenizenScriptServer(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	switch base {
	case "denizen-language-server", "denizen-language-server.exe",
		"denizenscript-language-server", "denizenscript-language-server.exe",
		"denizen-lsp", "denizen-lsp.exe",
		"denizenscript-lsp", "denizenscript-lsp.exe",
		"denizenlanguageserver", "denizenlanguageserver.exe":
		return true
	default:
		return strings.Contains(base, "denizen-language-server") ||
			strings.Contains(base, "denizenscript-language-server") ||
			strings.Contains(base, "denizen-lsp") ||
			strings.Contains(base, "denizenscript-lsp") ||
			strings.Contains(base, "denizenlanguageserver")
	}
}

func looksLikeDotnet(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	return base == "dotnet" || base == "dotnet.exe"
}

func looksLikeMono(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	return base == "mono" || base == "mono.exe"
}

func referencesDenizenScriptServer(args []string) bool {
	return slices.ContainsFunc(args, func(arg string) bool {
		normalized := strings.ToLower(strings.TrimSpace(arg))
		return strings.Contains(normalized, "denizen-language-server") ||
			strings.Contains(normalized, "denizenscript-language-server") ||
			strings.Contains(normalized, "denizen-lsp") ||
			strings.Contains(normalized, "denizenscript-lsp") ||
			strings.Contains(normalized, "denizenlanguageserver") ||
			strings.Contains(normalized, "denizenvscode")
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
				"description": "DenizenScript language-server command name to execute (see denizenscript_list_extension_commands).",
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
		context = "DenizenScript"
	}
	context = context + " via " + denizenscriptServerContext
	commandName := strings.TrimSpace(name)
	if commandName == "" {
		return "Run workspace commands only when " + context + " advertises them; semantics and arguments are server-specific."
	}
	return "Run workspace command " + commandName + " only when " + context + " advertises it; semantics and arguments are server-specific."
}

func describeCapabilityArgs(name string, language string) string {
	context := strings.TrimSpace(language)
	if context == "" {
		context = "DenizenScript"
	}
	context = context + " via " + denizenscriptServerContext
	commandName := strings.TrimSpace(name)
	if commandName == "" {
		return "arguments array expected by " + context + " for the selected command"
	}
	return "arguments array expected by " + context + " for " + commandName
}

func describeCapabilityCommand(name string, language string) string {
	return triadDescription(describeCapabilityWhen(name, language), describeCapabilityArgs(name, language), "exec")
}
