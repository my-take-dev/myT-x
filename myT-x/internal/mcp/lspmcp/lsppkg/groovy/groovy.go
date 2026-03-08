// Package groovy は Groovy 向けの MCP 拡張ツールを提供する。
package groovy

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
	groovyReferenceURL1 = "https://github.com/palantir/groovy-language-server"
	groovyReferenceURL2 = "https://github.com/Prominic/groovy-language-server"
	groovyReferenceURL3 = "https://github.com/nvuillam/vscode-groovy-lint"
)

// BuildTools は Groovy 言語サーバー向けの拡張ツールを構築する。
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
			Name:        "groovy_list_extension_commands",
			Description: triadDescription("Inspect workspace commands advertised via executeCommandProvider.commands by the connected Groovy language server", "none", "read"),
			InputSchema: emptySchema(),
			Handler:     svc.handleListCommands,
		},
		{
			Name:        "groovy_execute_extension_command",
			Description: triadDescription("Run one workspace command advertised by the connected Groovy language server", "command, arguments?", "exec"),
			InputSchema: executeCommandSchema(),
			Handler:     svc.handleExecuteCommand,
		},
	}
}

// Matches returns true when the configured LSP command/args indicate Groovy language servers.
func Matches(command string, args []string) bool {
	if looksLikeGroovyServer(command) {
		return true
	}

	if slices.ContainsFunc(args, looksLikeGroovyServer) {
		return true
	}

	if looksLikeJava(command) && referencesGroovyServer(args) {
		return true
	}

	if looksLikeNode(command) && referencesGroovyServer(args) {
		return true
	}

	if looksLikeNodeRunner(command) && referencesGroovyServer(args) {
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
		when, argsHint, effect := capabilityCommandGuide(name, "Groovy")
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
		"lsp":               "groovy",
		"language":          "Groovy",
		"root":              s.rootDir,
		"commands":          list,
		"availableCommands": commands,
		"count":             len(commands),
		"references":        []string{groovyReferenceURL1, groovyReferenceURL2, groovyReferenceURL3},
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

	when, argsHint, effect := capabilityCommandGuide(command, "Groovy")

	return map[string]any{
		"lsp":               "groovy",
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

func looksLikeGroovyServer(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	switch base {
	case "groovy-language-server", "groovy-language-server.exe",
		"groovy-language-server-all.jar",
		"groovy-ls", "groovy-ls.cmd", "groovy-ls.exe",
		"groovy-lint-language-server", "groovy-lint-language-server.cmd", "groovy-lint-language-server.exe",
		"npm-groovy-lint-language-server", "npm-groovy-lint-language-server.cmd", "npm-groovy-lint-language-server.exe":
		return true
	default:
		return strings.Contains(base, "groovy-language-server") ||
			strings.Contains(base, "groovy-ls") ||
			strings.Contains(base, "groovy-lint-language-server") ||
			strings.Contains(base, "npm-groovy-lint")
	}
}

func looksLikeJava(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	return base == "java" || base == "java.exe"
}

func looksLikeNode(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	return base == "node" || base == "node.exe" || base == "nodejs" || base == "nodejs.exe"
}

func looksLikeNodeRunner(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	switch base {
	case "npx", "npx.cmd", "npx.exe",
		"pnpm", "pnpm.cmd", "pnpm.exe",
		"yarn", "yarn.cmd", "yarn.exe",
		"bun", "bun.exe":
		return true
	default:
		return false
	}
}

func referencesGroovyServer(args []string) bool {
	return slices.ContainsFunc(args, func(arg string) bool {
		normalized := strings.ToLower(strings.TrimSpace(arg))
		normalized = strings.ReplaceAll(normalized, "\\", "/")
		return strings.Contains(normalized, "groovy-language-server") ||
			strings.Contains(normalized, "groovy-ls") ||
			strings.Contains(normalized, "groovy-lint-language-server") ||
			strings.Contains(normalized, "npm-groovy-lint") ||
			strings.Contains(normalized, "vscode-groovy-lint")
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
				"description": "Groovy language-server command name to execute (see groovy_list_extension_commands).",
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
