// Package systemd は systemd 向けの MCP 拡張ツールを提供する。
package systemd

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

const systemdReferenceURL = "https://github.com/psacawa/systemd-language-server"

// BuildTools は systemd 言語サーバー向けの拡張ツールを構築する。
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
			Name:        "systemd_list_extension_commands",
			Description: triadDescription("Inspect workspace commands advertised via executeCommandProvider.commands by the connected systemd language server", "none", "read"),
			InputSchema: emptySchema(),
			Handler:     svc.handleListCommands,
		},
		{
			Name:        "systemd_execute_extension_command",
			Description: triadDescription("Run one workspace command advertised by the connected systemd language server", "command, arguments?", "exec"),
			InputSchema: executeCommandSchema(),
			Handler:     svc.handleExecuteCommand,
		},
	}
}

// Matches は設定されたコマンド/引数が systemd 言語サーバーを示す場合に true を返す。
func Matches(command string, args []string) bool {
	if looksLikeSystemdServer(command) {
		return true
	}

	if slices.ContainsFunc(args, looksLikeSystemdServer) {
		return true
	}

	if looksLikePython(command) && referencesSystemdServer(args) {
		return true
	}

	if looksLikePythonRunner(command) && referencesSystemdServer(args) {
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
	when, argsHint, effect, description := capabilityCommandGuide("systemd")
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
		"lsp":               "systemd",
		"language":          "systemd",
		"root":              s.rootDir,
		"commands":          list,
		"availableCommands": commands,
		"count":             len(commands),
		"references":        []string{systemdReferenceURL},
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

	when, argsHint, effect, description := capabilityCommandGuide("systemd")

	availableCommands, err := s.availableCommands()
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"lsp":               "systemd",
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

func looksLikeSystemdServer(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	switch base {
	case "systemd-language-server", "systemd-language-server.exe", "systemd-language-server.cmd",
		"systemd-lsp", "systemd-lsp.exe",
		"systemd_lsp", "systemd_lsp.exe":
		return true
	default:
		return strings.Contains(base, "systemd-language-server") ||
			strings.Contains(base, "systemd-lsp") ||
			strings.Contains(base, "systemd_lsp")
	}
}

func looksLikePython(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	switch base {
	case "python", "python.exe",
		"python3", "python3.exe",
		"py", "py.exe":
		return true
	default:
		return false
	}
}

func looksLikePythonRunner(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	switch base {
	case "pipx", "pipx.exe",
		"uv", "uv.exe",
		"uvx", "uvx.exe":
		return true
	default:
		return false
	}
}

func referencesSystemdServer(args []string) bool {
	return slices.ContainsFunc(args, func(arg string) bool {
		normalized := strings.ToLower(strings.TrimSpace(arg))
		normalized = strings.ReplaceAll(normalized, "\\", "/")
		return strings.Contains(normalized, "systemd-language-server") ||
			strings.Contains(normalized, "systemd-lsp") ||
			strings.Contains(normalized, "systemd_lsp") ||
			strings.Contains(normalized, "systemd_language_server") ||
			strings.Contains(normalized, "psacawa/systemd-language-server")
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
				"description": "systemd language-server command name to execute (see systemd_list_extension_commands).",
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
