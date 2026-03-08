// Package dreammaker は DreamMaker 向けの MCP 拡張ツールを提供する。
package dreammaker

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

const dreamMakerReferenceURL = "https://github.com/SpaceManiac/SpacemanDMM"

// BuildTools は DreamMaker 言語サーバー向けの拡張ツールを構築する。
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
			Name:        "dreammaker_list_extension_commands",
			Description: triadDescription("Inspect workspace commands advertised via executeCommandProvider.commands by the connected DreamMaker language server", "none", "read"),
			InputSchema: emptySchema(),
			Handler:     svc.handleListCommands,
		},
		{
			Name:        "dreammaker_execute_extension_command",
			Description: triadDescription("Run one workspace command advertised by the connected DreamMaker language server", "command, arguments?", "exec"),
			InputSchema: executeCommandSchema(),
			Handler:     svc.handleExecuteCommand,
		},
	}
}

// Matches は設定されたコマンド/引数が DreamMaker 言語サーバーを示す場合に true を返す。
func Matches(command string, args []string) bool {
	if looksLikeDreamMakerServer(command) {
		return true
	}

	if slices.ContainsFunc(args, looksLikeDreamMakerServer) {
		return true
	}

	if looksLikeCargo(command) && referencesDreamMakerServer(args) {
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
		when, argsHint, effect := capabilityCommandGuide(name, "DreamMaker")
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
		"lsp":               "dreammaker",
		"language":          "DreamMaker",
		"root":              s.rootDir,
		"commands":          list,
		"availableCommands": commands,
		"count":             len(commands),
		"references":        []string{dreamMakerReferenceURL},
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

	when, argsHint, effect := capabilityCommandGuide(command, "DreamMaker")

	return map[string]any{
		"lsp":               "dreammaker",
		"root":              s.rootDir,
		"command":           command,
		"arguments":         arguments,
		"result":            result,
		"commandGuide":      map[string]any{"when": when, "args": argsHint, "effect": effect, "description": triadDescription(when, argsHint, effect)},
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

func looksLikeDreamMakerServer(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	switch base {
	case "dm-langserver", "dm-langserver.exe",
		"dreammaker-language-server", "dreammaker-language-server.exe",
		"dreammaker-ls", "dreammaker-ls.exe":
		return true
	default:
		return strings.Contains(base, "dm-langserver") ||
			strings.Contains(base, "dreammaker-language-server") ||
			strings.Contains(base, "dreammaker-ls")
	}
}

func looksLikeCargo(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	return base == "cargo" || base == "cargo.exe"
}

func referencesDreamMakerServer(args []string) bool {
	return slices.ContainsFunc(args, func(arg string) bool {
		normalized := strings.ToLower(strings.TrimSpace(arg))
		return strings.Contains(normalized, "dm-langserver") ||
			strings.Contains(normalized, "dreammaker-language-server") ||
			strings.Contains(normalized, "spacemandmm")
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
				"description": "DreamMaker language-server command name to execute (see dreammaker_list_extension_commands).",
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
