// Package slint は Slint 向けの MCP 拡張ツールを提供する。
package slint

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

const slintReferenceURL = "https://github.com/slint-ui/slint"

// BuildTools は Slint 言語サーバー向けの拡張ツールを構築する。
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
			Name:        "slint_list_extension_commands",
			Description: triadDescription("Inspect workspace commands advertised via executeCommandProvider.commands by the connected Slint language server", "none", "read"),
			InputSchema: emptySchema(),
			Handler:     svc.handleListCommands,
		},
		{
			Name:        "slint_execute_extension_command",
			Description: triadDescription("Run one workspace command advertised by the connected Slint language server", "command, arguments?", "exec"),
			InputSchema: executeCommandSchema(),
			Handler:     svc.handleExecuteCommand,
		},
	}
}

// Matches returns true when the configured LSP command/args indicate a Slint language server.
func Matches(command string, args []string) bool {
	if looksLikeSlintServer(command) {
		return true
	}

	if slices.ContainsFunc(args, looksLikeSlintServer) {
		return true
	}

	if looksLikeCargo(command) && referencesSlintServer(args) {
		return true
	}

	return false
}

type service struct {
	client  *lsp.Client
	rootDir string
}

func (s *service) handleListCommands(_ context.Context, _ map[string]any) (any, error) {
	commands := s.availableCommands()
	list := make([]map[string]any, 0, len(commands))
	for _, name := range commands {
		when := describeCapabilityWhen(name, "Slint")
		argsHint := describeCapabilityArgs(name, "Slint")
		effect := "exec"
		list = append(list, map[string]any{
			"name":        name,
			"when":        when,
			"args":        argsHint,
			"effect":      effect,
			"description": describeCapabilityCommand(name, "Slint"),
			"available":   true,
			"source":      "server-capabilities",
		})
	}

	return map[string]any{
		"lsp":               "slint",
		"language":          "Slint",
		"root":              s.rootDir,
		"commands":          list,
		"availableCommands": commands,
		"count":             len(commands),
		"references":        []string{slintReferenceURL},
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

	when := describeCapabilityWhen(command, "Slint")
	argsHint := describeCapabilityArgs(command, "Slint")
	effect := "exec"

	return map[string]any{
		"lsp":               "slint",
		"root":              s.rootDir,
		"command":           command,
		"commandGuide":      map[string]any{"when": when, "args": argsHint, "effect": effect, "description": describeCapabilityCommand(command, "Slint")},
		"arguments":         arguments,
		"result":            result,
		"availableCommands": s.availableCommands(),
	}, nil
}

func (s *service) availableCommands() []string {
	caps := s.client.Capabilities()
	raw, ok := caps["executeCommandProvider"]
	if !ok {
		return nil
	}

	var provider struct {
		Commands []string `json:"commands"`
	}
	if err := json.Unmarshal(raw, &provider); err != nil {
		return nil
	}
	sort.Strings(provider.Commands)
	return provider.Commands
}

func looksLikeSlintServer(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	switch base {
	case "slint-lsp", "slint-lsp.exe", "slint-lsp.cmd", "slint-lsp.bat",
		"slint-language-server", "slint-language-server.exe", "slint-language-server.cmd",
		"slint_language_server", "slint_language_server.exe", "slint_language_server.cmd":
		return true
	default:
		return strings.Contains(base, "slint-lsp") ||
			strings.Contains(base, "slint-language-server") ||
			strings.Contains(base, "slint_language_server")
	}
}

func looksLikeCargo(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	return base == "cargo" || base == "cargo.exe"
}

func referencesSlintServer(args []string) bool {
	return slices.ContainsFunc(args, func(arg string) bool {
		normalized := strings.ToLower(strings.TrimSpace(arg))
		return looksLikeSlintServer(normalized) ||
			strings.Contains(normalized, "slint-lsp") ||
			strings.Contains(normalized, "slint-language-server") ||
			strings.Contains(normalized, "slint_language_server") ||
			strings.Contains(normalized, "slint-ui/slint") ||
			strings.Contains(normalized, "slint-ui\\slint")
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
				"description": "Slint language-server command name to execute (see slint_list_extension_commands).",
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
