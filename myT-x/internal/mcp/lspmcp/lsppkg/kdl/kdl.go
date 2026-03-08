// Package kdl は KDL 向けの MCP 拡張ツールを提供する。
package kdl

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

const kdlReferenceURL = "https://github.com/kdl-org/vscode-kdl"

// BuildTools provides KDL language-server specific extension tools.
func BuildTools(client *lsp.Client, rootDir string) []mcp.Tool {
	svc := &service{
		client:  client,
		rootDir: rootDir,
	}

	return []mcp.Tool{
		{
			Name:        "kdl_list_extension_commands",
			Description: triadDescription("Inspect workspace commands advertised via executeCommandProvider.commands by the connected KDL language server", "none", "read"),
			InputSchema: emptySchema(),
			Handler:     svc.handleListCommands,
		},
		{
			Name:        "kdl_execute_extension_command",
			Description: triadDescription("Run one workspace command advertised by the connected KDL language server", "command, arguments?", "exec"),
			InputSchema: executeCommandSchema(),
			Handler:     svc.handleExecuteCommand,
		},
	}
}

// Matches returns true when the configured LSP command/args indicate KDL language server.
func Matches(command string, args []string) bool {
	if looksLikeKDLServer(command) {
		return true
	}

	if slices.ContainsFunc(args, looksLikeKDLServer) {
		return true
	}

	if looksLikeNode(command) && referencesKDLServer(args) {
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
		when, argsHint, effect := capabilityCommandGuide(name, "KDL")
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
		"lsp":               "kdl",
		"language":          "KDL",
		"root":              s.rootDir,
		"commands":          list,
		"availableCommands": commands,
		"count":             len(commands),
		"references":        []string{kdlReferenceURL},
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

	when, argsHint, effect := capabilityCommandGuide(command, "KDL")

	availableCommands, err := s.availableCommands()
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"lsp":               "kdl",
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

func looksLikeKDLServer(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	switch base {
	case "vscode-kdl", "vscode-kdl.cmd", "vscode-kdl.exe",
		"kdl-language-server", "kdl-language-server.cmd", "kdl-language-server.exe",
		"kdl-lsp", "kdl-lsp.cmd", "kdl-lsp.exe":
		return true
	default:
		return strings.Contains(base, "vscode-kdl") ||
			strings.Contains(base, "kdl-language-server") ||
			strings.Contains(base, "kdl-lsp") ||
			strings.Contains(normalized, "vscode-kdl") ||
			strings.Contains(normalized, "kdl-language-server") ||
			strings.Contains(normalized, "kdl-lsp")
	}
}

func looksLikeNode(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	return base == "node" || base == "node.exe"
}

func referencesKDLServer(args []string) bool {
	return slices.ContainsFunc(args, func(arg string) bool {
		normalized := strings.ToLower(strings.TrimSpace(arg))
		return strings.Contains(normalized, "vscode-kdl") ||
			strings.Contains(normalized, "kdl-language-server") ||
			strings.Contains(normalized, "kdl-lsp") ||
			strings.Contains(normalized, "@kdl-org/vscode-kdl")
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
				"description": "KDL language-server command name to execute (see kdl_list_extension_commands).",
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
