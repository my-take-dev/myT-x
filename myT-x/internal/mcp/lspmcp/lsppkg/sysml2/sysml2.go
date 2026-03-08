// Package sysml2 は SysML v2 向けの MCP 拡張ツールを提供する。
package sysml2

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

const sysML2ReferenceURL = "https://github.com/Sensmetry/sysml2-tools"

// BuildTools は SysML v2 言語サーバー向けの拡張ツールを構築する。
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
			Name:        "sysml2_list_extension_commands",
			Description: triadDescription("Inspect workspace commands advertised via executeCommandProvider.commands by the connected SysML v2 language server", "none", "read"),
			InputSchema: emptySchema(),
			Handler:     svc.handleListCommands,
		},
		{
			Name:        "sysml2_execute_extension_command",
			Description: triadDescription("Run one workspace command advertised by the connected SysML v2 language server", "command, arguments?", "exec"),
			InputSchema: executeCommandSchema(),
			Handler:     svc.handleExecuteCommand,
		},
	}
}

// Matches は設定されたコマンド/引数が SysML v2 言語サーバーを示す場合に true を返す。
func Matches(command string, args []string) bool {
	if looksLikeSysML2Server(command) {
		return true
	}

	if slices.ContainsFunc(args, looksLikeSysML2Server) {
		return true
	}

	if looksLikeNode(command) && referencesSysML2Server(args) {
		return true
	}

	if looksLikeNodeRunner(command) && referencesSysML2Server(args) {
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
	when, argsHint, effect, description := capabilityCommandGuide("SysML v2")
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
		"lsp":               "sysml2",
		"language":          "SysML v2",
		"root":              s.rootDir,
		"commands":          list,
		"availableCommands": commands,
		"count":             len(commands),
		"references":        []string{sysML2ReferenceURL},
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

	when, argsHint, effect, description := capabilityCommandGuide("SysML v2")

	availableCommands, err := s.availableCommands()
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"lsp":               "sysml2",
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

func looksLikeSysML2Server(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	switch base {
	case "sysml2-language-server", "sysml2-language-server.cmd", "sysml2-language-server.exe",
		"sysml2-lsp", "sysml2-lsp.cmd", "sysml2-lsp.exe",
		"sysml-language-server", "sysml-language-server.cmd", "sysml-language-server.exe":
		return true
	default:
		return strings.Contains(base, "sysml2-language-server") ||
			strings.Contains(base, "sysml2-lsp") ||
			strings.Contains(base, "sysml-language-server")
	}
}

func looksLikeNode(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	return base == "node" || base == "node.exe" || base == "nodejs" || base == "nodejs.exe"
}

func looksLikeNodeRunner(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	switch base {
	case "npx", "npx.cmd", "npx.exe",
		"npm", "npm.cmd", "npm.exe",
		"pnpm", "pnpm.cmd", "pnpm.exe",
		"yarn", "yarn.cmd", "yarn.exe",
		"bun", "bun.exe":
		return true
	default:
		return false
	}
}

func referencesSysML2Server(args []string) bool {
	return slices.ContainsFunc(args, func(arg string) bool {
		normalized := strings.ToLower(strings.TrimSpace(arg))
		normalized = strings.ReplaceAll(normalized, "\\", "/")
		return strings.Contains(normalized, "sysml2-language-server") ||
			strings.Contains(normalized, "sysml2-lsp") ||
			strings.Contains(normalized, "sysml-language-server") ||
			strings.Contains(normalized, "sysml2-tools") ||
			strings.Contains(normalized, "sensmetry/sysml2-tools") ||
			strings.Contains(normalized, "@sensmetry/sysml2-tools")
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
				"description": "SysML v2 language-server command name to execute (see sysml2_list_extension_commands).",
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
