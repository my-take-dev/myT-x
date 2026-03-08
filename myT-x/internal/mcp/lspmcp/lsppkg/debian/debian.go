// Package debian は Debian Packaging files 向けの MCP 拡張ツールを提供する。
package debian

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
	debianReferenceURL  = "https://salsa.debian.org/debian/debputy"
	debianServerContext = "debputy lsp server (debian/debputy)"
)

// BuildTools は Debian Packaging files 言語サーバー向けの拡張ツールを構築する。
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
			Name:        "debian_list_extension_commands",
			Description: triadDescription("Inspect workspace commands advertised via executeCommandProvider.commands by "+debianServerContext, "none", "read"),
			InputSchema: emptySchema(),
			Handler:     svc.handleListCommands,
		},
		{
			Name:        "debian_execute_extension_command",
			Description: triadDescription("Run one workspace command advertised by "+debianServerContext, "command, arguments?", "exec"),
			InputSchema: executeCommandSchema(),
			Handler:     svc.handleExecuteCommand,
		},
	}
}

// Matches は設定されたコマンド/引数が Debian Packaging files 言語サーバーを示す場合に true を返す。
func Matches(command string, args []string) bool {
	if looksLikeDebianServer(command) {
		return true
	}

	if slices.ContainsFunc(args, looksLikeDebianServer) {
		return true
	}

	if looksLikeDebputyCLI(command) && referencesDebianLSPSubcommand(args) {
		return true
	}

	if looksLikePython(command) && referencesDebianServer(args) {
		return true
	}

	if looksLikeUV(command) && referencesDebianServer(args) {
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
		when := describeCapabilityWhen(name, "Debian Packaging files")
		argsHint := describeCapabilityArgs(name, "Debian Packaging files")
		effect := "exec"
		list = append(list, map[string]any{
			"name":        name,
			"when":        when,
			"args":        argsHint,
			"effect":      effect,
			"description": describeCapabilityCommand(name, "Debian Packaging files"),
			"available":   true,
			"source":      "server-capabilities",
		})
	}

	return map[string]any{
		"lsp":               "debian",
		"language":          "Debian Packaging files",
		"root":              s.rootDir,
		"commands":          list,
		"availableCommands": commands,
		"count":             len(commands),
		"references":        []string{debianReferenceURL},
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

	when := describeCapabilityWhen(command, "Debian Packaging files")
	argsHint := describeCapabilityArgs(command, "Debian Packaging files")
	effect := "exec"

	availableCommands, err := s.availableCommands()
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"lsp":               "debian",
		"root":              s.rootDir,
		"command":           command,
		"commandGuide":      map[string]any{"when": when, "args": argsHint, "effect": effect, "description": describeCapabilityCommand(command, "Debian Packaging files")},
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

func looksLikeDebianServer(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	switch base {
	case "debputy-lsp", "debputy-lsp.exe",
		"debputy-language-server", "debputy-language-server.exe":
		return true
	default:
		return strings.Contains(base, "debputy-lsp") || strings.Contains(base, "debputy-language-server")
	}
}

func looksLikeDebputyCLI(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	return base == "debputy" || base == "debputy.exe"
}

func looksLikePython(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	return base == "py" || base == "py.exe" || strings.HasPrefix(base, "python")
}

func looksLikeUV(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	return base == "uv" || base == "uv.exe" || base == "uvx" || base == "uvx.exe"
}

func referencesDebianLSPSubcommand(args []string) bool {
	return slices.ContainsFunc(args, func(arg string) bool {
		normalized := strings.ToLower(strings.TrimSpace(arg))
		return normalized == "lsp" || strings.Contains(normalized, "lsp-server") || strings.Contains(normalized, "language-server")
	})
}

func referencesDebianServer(args []string) bool {
	return slices.ContainsFunc(args, func(arg string) bool {
		normalized := strings.ToLower(strings.TrimSpace(arg))
		return strings.Contains(normalized, "debputy") || strings.Contains(normalized, "debian-lsp")
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
				"description": "Debian Packaging files language-server command name to execute (see debian_list_extension_commands).",
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
		context = "Debian Packaging files"
	}
	context = context + " via " + debianServerContext
	commandName := strings.TrimSpace(name)
	if commandName == "" {
		return "Run workspace commands only when " + context + " advertises them; semantics and arguments are server-specific."
	}
	return "Run workspace command " + commandName + " only when " + context + " advertises it; semantics and arguments are server-specific."
}

func describeCapabilityArgs(name string, language string) string {
	context := strings.TrimSpace(language)
	if context == "" {
		context = "Debian Packaging files"
	}
	context = context + " via " + debianServerContext
	commandName := strings.TrimSpace(name)
	if commandName == "" {
		return "arguments array expected by " + context + " for the selected command"
	}
	return "arguments array expected by " + context + " for " + commandName
}

func describeCapabilityCommand(name string, language string) string {
	return triadDescription(describeCapabilityWhen(name, language), describeCapabilityArgs(name, language), "exec")
}
