// Package latex は LaTeX 向けの MCP 拡張ツールを提供する。
package latex

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

const latexReferenceURL = "https://github.com/latex-lsp/texlab"

// BuildTools は LaTeX 言語サーバー向けの拡張ツールを構築する。
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
			Name:        "latex_list_extension_commands",
			Description: triadDescription("Inspect workspace commands advertised via executeCommandProvider.commands by the connected LaTeX language server", "none", "read"),
			InputSchema: emptySchema(),
			Handler:     svc.handleListCommands,
		},
		{
			Name:        "latex_execute_extension_command",
			Description: triadDescription("Run one workspace command advertised by the connected LaTeX language server", "command, arguments?", "exec"),
			InputSchema: executeCommandSchema(),
			Handler:     svc.handleExecuteCommand,
		},
	}
}

// Matches は設定されたコマンド/引数が texlab を示す場合に true を返す。
func Matches(command string, args []string) bool {
	if looksLikeTexlabServer(command) {
		return true
	}

	if slices.ContainsFunc(args, looksLikeTexlabServer) {
		return true
	}

	if looksLikeCargo(command) && referencesTexlabServer(args) {
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
		when, argsHint, effect := capabilityCommandGuide(name, "LaTeX")
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
		"lsp":               "latex",
		"language":          "LaTeX",
		"root":              s.rootDir,
		"commands":          list,
		"availableCommands": commands,
		"count":             len(commands),
		"references":        []string{latexReferenceURL},
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

	when, argsHint, effect := capabilityCommandGuide(command, "LaTeX")

	availableCommands, err := s.availableCommands()
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"lsp":               "latex",
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

func looksLikeTexlabServer(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	switch base {
	case "texlab", "texlab.exe":
		return true
	default:
		return strings.Contains(base, "texlab")
	}
}

func looksLikeCargo(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	return base == "cargo" || base == "cargo.exe"
}

func referencesTexlabServer(args []string) bool {
	for i, arg := range args {
		normalized := strings.ToLower(strings.TrimSpace(arg))
		if strings.Contains(normalized, "texlab") {
			return true
		}
		if (normalized == "--bin" || normalized == "-p" || normalized == "--package") && i+1 < len(args) {
			next := strings.ToLower(strings.TrimSpace(args[i+1]))
			if strings.Contains(next, "texlab") {
				return true
			}
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
				"description": "LaTeX language-server command name to execute (see latex_list_extension_commands).",
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
