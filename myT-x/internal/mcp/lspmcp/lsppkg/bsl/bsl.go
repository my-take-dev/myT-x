// Package bsl は BSL 向けの MCP 拡張ツールを提供する。
package bsl

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
	bslReferenceURL  = "https://github.com/1c-syntax/bsl-language-server"
	bslServerContext = "BSL Language Server (1c-syntax/bsl-language-server)"
)

// BuildTools は BSL 言語サーバー向けの拡張ツールを構築する。
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
			Name:        "bsl_list_extension_commands",
			Description: triadDescription("Inspect workspace commands exposed via executeCommandProvider.commands by "+bslServerContext, "none", "read"),
			InputSchema: emptySchema(),
			Handler:     svc.handleListCommands,
		},
		{
			Name:        "bsl_execute_extension_command",
			Description: triadDescription("Run one workspace command exposed by "+bslServerContext, "command, arguments?", "exec"),
			InputSchema: executeCommandSchema(),
			Handler:     svc.handleExecuteCommand,
		},
	}
}

// Matches は設定されたコマンド/引数が BSL 言語サーバーを示す場合に true を返す。
func Matches(command string, args []string) bool {
	if looksLikeBSLServer(command) {
		return true
	}

	if slices.ContainsFunc(args, looksLikeBSLServer) {
		return true
	}

	if looksLikeJava(command) && referencesBSLServer(args) {
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
		when := describeCapabilityWhen(name)
		argsHint := "command-specific payload"
		effect := "exec"
		list = append(list, map[string]any{
			"name":        name,
			"when":        when,
			"args":        argsHint,
			"effect":      effect,
			"description": describeCapabilityCommand(name),
			"available":   true,
			"source":      "server-capabilities",
		})
	}

	return map[string]any{
		"lsp":               "bsl",
		"language":          "1C Enterprise (BSL)",
		"root":              s.rootDir,
		"commands":          list,
		"availableCommands": commands,
		"count":             len(commands),
		"references":        []string{bslReferenceURL},
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

	when := describeCapabilityWhen(command)
	argsHint := "command-specific payload"
	effect := "exec"

	availableCommands, err := s.availableCommands()
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"lsp":               "bsl",
		"root":              s.rootDir,
		"command":           command,
		"commandGuide":      map[string]any{"when": when, "args": argsHint, "effect": effect, "description": describeCapabilityCommand(command)},
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

func looksLikeBSLServer(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	switch base {
	case "bsl-language-server", "bsl-language-server.exe", "bsl-language-server.cmd", "bsl-language-server.bat":
		return true
	default:
		return strings.Contains(base, "bsl-language-server")
	}
}

func looksLikeJava(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	return base == "java" || base == "java.exe"
}

func referencesBSLServer(args []string) bool {
	return slices.ContainsFunc(args, func(arg string) bool {
		normalized := strings.ToLower(strings.TrimSpace(arg))
		return strings.Contains(normalized, "bsl-language-server")
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
				"description": "BSL Language Server command name to execute (see bsl_list_extension_commands).",
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

func describeCapabilityWhen(name string) string {
	if strings.TrimSpace(name) == "" {
		return "Run this workspace command only when " + bslServerContext + " advertises it for 1C Enterprise (BSL); semantics and arguments are command-specific."
	}
	return "Run the \"" + strings.TrimSpace(name) + "\" workspace command only when " + bslServerContext + " advertises it for 1C Enterprise (BSL); semantics and arguments are command-specific."
}

func describeCapabilityCommand(name string) string {
	return triadDescription(describeCapabilityWhen(name), "command-specific payload", "exec")
}
