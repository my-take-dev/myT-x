// Package tads3 は Tads3 向けの MCP 拡張ツールを提供する。
package tads3

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

const tads3ReferenceURL = "https://github.com/tomas-oberg/tads3tools"

// BuildTools は Tads3 言語サーバー向けの拡張ツールを構築する。
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
			Name:        "tads3_list_extension_commands",
			Description: triadDescription("Inspect workspace commands advertised via executeCommandProvider.commands by the connected Tads3 language server", "none", "read"),
			InputSchema: emptySchema(),
			Handler:     svc.handleListCommands,
		},
		{
			Name:        "tads3_execute_extension_command",
			Description: triadDescription("Run one workspace command advertised by the connected Tads3 language server", "command, arguments?", "exec"),
			InputSchema: executeCommandSchema(),
			Handler:     svc.handleExecuteCommand,
		},
	}
}

// Matches は設定されたコマンド/引数が Tads3 言語サーバーを示す場合に true を返す。
func Matches(command string, args []string) bool {
	if looksLikeTads3Server(command) {
		return true
	}

	if slices.ContainsFunc(args, looksLikeTads3Server) {
		return true
	}

	if looksLikeTads3CLI(command) && (referencesTads3LanguageServerMode(args) || referencesTads3Server(args)) {
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
	when, argsHint, effect, description := capabilityCommandGuide("Tads3")
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
		"lsp":               "tads3",
		"language":          "Tads3",
		"root":              s.rootDir,
		"commands":          list,
		"availableCommands": commands,
		"count":             len(commands),
		"references":        []string{tads3ReferenceURL},
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

	when, argsHint, effect, description := capabilityCommandGuide("Tads3")

	availableCommands, err := s.availableCommands()
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"lsp":               "tads3",
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

func looksLikeTads3Server(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	switch base {
	case "tads3-language-server", "tads3-language-server.exe",
		"tads3-lsp", "tads3-lsp.exe",
		"tads3tools-lsp", "tads3tools-lsp.exe",
		"t3ls", "t3ls.exe":
		return true
	default:
		return strings.Contains(base, "tads3-language-server") ||
			strings.Contains(base, "tads3-lsp") ||
			strings.Contains(base, "tads3tools-lsp") ||
			strings.Contains(base, "t3ls")
	}
}

func looksLikeTads3CLI(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	switch base {
	case "tads3", "tads3.exe",
		"t3make", "t3make.exe",
		"t3run", "t3run.exe":
		return true
	default:
		return false
	}
}

func referencesTads3Server(args []string) bool {
	return slices.ContainsFunc(args, func(arg string) bool {
		normalized := strings.ToLower(strings.TrimSpace(arg))
		normalized = strings.ReplaceAll(normalized, "\\", "/")
		return strings.Contains(normalized, "tomas-oberg/tads3tools") ||
			strings.Contains(normalized, "github.com/tomas-oberg/tads3tools") ||
			strings.Contains(normalized, "tads3-language-server") ||
			strings.Contains(normalized, "tads3tools-lsp") ||
			strings.Contains(normalized, "t3ls")
	})
}

func referencesTads3LanguageServerMode(args []string) bool {
	for _, arg := range args {
		normalized := strings.ToLower(strings.TrimSpace(arg))
		switch normalized {
		case "lsp", "language-server", "langserver", "lsp-server":
			return true
		}
		if strings.HasPrefix(normalized, "--lsp") ||
			strings.HasPrefix(normalized, "-lsp") ||
			strings.Contains(normalized, "language-server") {
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
				"description": "Tads3 language-server command name to execute (see tads3_list_extension_commands).",
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
