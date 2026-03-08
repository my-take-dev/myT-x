// Package robotframework は Robot Framework 向けの MCP 拡張ツールを提供する。
package robotframework

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
	robotFrameworkRobotCodeReferenceURL = "https://github.com/d-biehl/robotcode"
	robotFrameworkRobocorpReferenceURL  = "https://github.com/robocorp/robotframework-lsp"
)

// BuildTools は Robot Framework 言語サーバー向けの拡張ツールを構築する。
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
			Name:        "robotframework_list_extension_commands",
			Description: triadDescription("Inspect workspace commands advertised via executeCommandProvider.commands by the connected Robot Framework language server", "none", "read"),
			InputSchema: emptySchema(),
			Handler:     svc.handleListCommands,
		},
		{
			Name:        "robotframework_execute_extension_command",
			Description: triadDescription("Run one workspace command advertised by the connected Robot Framework language server", "command, arguments?", "exec"),
			InputSchema: executeCommandSchema(),
			Handler:     svc.handleExecuteCommand,
		},
	}
}

// Matches は設定されたコマンド/引数が Robot Framework 言語サーバーを示す場合に true を返す。
func Matches(command string, args []string) bool {
	if looksLikeRobotFrameworkServer(command) {
		return true
	}

	if slices.ContainsFunc(args, looksLikeRobotFrameworkServer) {
		return true
	}

	if looksLikeRobotCode(command) && referencesLanguageServerMode(args) {
		return true
	}

	if slices.ContainsFunc(args, looksLikeRobotCode) && referencesLanguageServerMode(args) {
		return true
	}

	if looksLikePythonRuntime(command) && referencesRobotFrameworkPythonServer(args) {
		return true
	}

	if looksLikeNode(command) && referencesRobotFrameworkNodeServer(args) {
		return true
	}

	if looksLikeNpx(command) && referencesRobotFrameworkNodeServer(args) {
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
		when := describeCapabilityWhen(name, "Robot Framework")
		argsHint := describeCapabilityArgs(name, "Robot Framework")
		effect := "exec"
		list = append(list, map[string]any{
			"name":        name,
			"when":        when,
			"args":        argsHint,
			"effect":      effect,
			"description": describeCapabilityCommand(name, "Robot Framework"),
			"available":   true,
			"source":      "server-capabilities",
		})
	}

	return map[string]any{
		"lsp":               "robotframework",
		"language":          "Robot Framework",
		"root":              s.rootDir,
		"commands":          list,
		"availableCommands": commands,
		"count":             len(commands),
		"references": []string{
			robotFrameworkRobotCodeReferenceURL,
			robotFrameworkRobocorpReferenceURL,
		},
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

	when := describeCapabilityWhen(command, "Robot Framework")
	argsHint := describeCapabilityArgs(command, "Robot Framework")
	effect := "exec"

	availableCommands, err := s.availableCommands()
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"lsp":               "robotframework",
		"root":              s.rootDir,
		"command":           command,
		"commandGuide":      map[string]any{"when": when, "args": argsHint, "effect": effect, "description": describeCapabilityCommand(command, "Robot Framework")},
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

func looksLikeRobotFrameworkServer(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))

	switch base {
	case "robotframework-lsp", "robotframework-lsp.exe", "robotframework-lsp.cmd",
		"robotframework_ls", "robotframework_ls.exe", "robotframework_ls.cmd",
		"robotframework-ls", "robotframework-ls.exe", "robotframework-ls.cmd",
		"rf-language-server", "rf-language-server.exe", "rf-language-server.cmd",
		"robotcode-language-server", "robotcode-language-server.exe", "robotcode-language-server.cmd":
		return true
	default:
		return strings.Contains(base, "robotframework-lsp") ||
			strings.Contains(base, "robotframework_ls") ||
			strings.Contains(base, "robotframework-ls") ||
			strings.Contains(base, "rf-language-server") ||
			strings.Contains(base, "robotcode-language-server") ||
			strings.Contains(normalized, "robocorp/robotframework-lsp") ||
			strings.Contains(normalized, "robocorp\\robotframework-lsp")
	}
}

func looksLikeRobotCode(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	return base == "robotcode" || base == "robotcode.exe" || base == "robotcode.cmd"
}

func looksLikePythonRuntime(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	switch base {
	case "python", "python.exe",
		"python3", "python3.exe",
		"py", "py.exe":
		return true
	}
	if strings.HasPrefix(base, "python3.") {
		return true
	}
	return false
}

func looksLikeNode(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	return base == "node" || base == "node.exe"
}

func looksLikeNpx(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	return base == "npx" || base == "npx.exe" || base == "npx.cmd"
}

func referencesRobotFrameworkPythonServer(args []string) bool {
	for i, arg := range args {
		normalized := strings.ToLower(strings.TrimSpace(arg))
		if normalized == "-m" && i+1 < len(args) {
			if looksLikeRobotFrameworkModule(args[i+1]) {
				return true
			}
			continue
		}
		if strings.HasPrefix(normalized, "-m") && len(normalized) > 2 {
			if looksLikeRobotFrameworkModule(strings.TrimPrefix(normalized, "-m")) {
				return true
			}
			continue
		}
		if looksLikeRobotFrameworkModule(normalized) || looksLikeRobotFrameworkServer(normalized) {
			return true
		}
	}

	return false
}

func looksLikeRobotFrameworkModule(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "robotframework_ls", "robotframework-lsp",
		"robotcode.language_server", "robotcode_language_server":
		return true
	default:
		return strings.Contains(normalized, "robotframework_ls") ||
			strings.Contains(normalized, "robotframework-lsp") ||
			strings.Contains(normalized, "robotcode.language_server") ||
			strings.Contains(normalized, "robotcode_language_server")
	}
}

func referencesRobotFrameworkNodeServer(args []string) bool {
	return slices.ContainsFunc(args, func(arg string) bool {
		normalized := strings.ToLower(strings.TrimSpace(arg))
		return strings.Contains(normalized, "robotframework-lsp") ||
			strings.Contains(normalized, "robotframework-ls") ||
			strings.Contains(normalized, "robocorp/robotframework-lsp") ||
			strings.Contains(normalized, "robocorp\\robotframework-lsp") ||
			strings.Contains(normalized, "vscode-rf-language-server")
	})
}

func referencesLanguageServerMode(args []string) bool {
	for _, arg := range args {
		normalized := strings.ToLower(strings.TrimSpace(arg))
		if normalized == "language-server" || normalized == "language_server" || normalized == "langserver" || normalized == "lsp" {
			return true
		}
		if strings.HasPrefix(normalized, "--language-server") ||
			strings.HasPrefix(normalized, "--mode=language-server") ||
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
				"description": "Robot Framework language-server command name to execute (see robotframework_list_extension_commands).",
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
