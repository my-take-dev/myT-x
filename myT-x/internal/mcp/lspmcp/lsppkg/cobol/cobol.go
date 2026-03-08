// Package cobol は COBOL 向けの MCP 拡張ツールを提供する。
package cobol

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
	cobolReferenceURL1 = "https://github.com/RechInformatica/rech-editor-cobol"
	cobolReferenceURL2 = "https://github.com/eclipse-che4z/cobol-language-support"
	cobolReferenceURL3 = "https://marketplace.visualstudio.com/items?itemName=IBM.zopeneditor"
	cobolServerContext = "rech-editor-cobol / che4z COBOL / IBM Z Open Editor COBOL integration"
)

// BuildTools は COBOL 言語サーバー向けの拡張ツールを構築する。
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
			Name:        "cobol_list_extension_commands",
			Description: triadDescription("Inspect workspace commands advertised via executeCommandProvider.commands by "+cobolServerContext, "none", "read"),
			InputSchema: emptySchema(),
			Handler:     svc.handleListCommands,
		},
		{
			Name:        "cobol_execute_extension_command",
			Description: triadDescription("Run one workspace command advertised by "+cobolServerContext, "command, arguments?", "exec"),
			InputSchema: executeCommandSchema(),
			Handler:     svc.handleExecuteCommand,
		},
	}
}

// Matches は設定されたコマンド/引数が COBOL 言語サーバーを示す場合に true を返す。
func Matches(command string, args []string) bool {
	if looksLikeCobolServer(command) {
		return true
	}

	if slices.ContainsFunc(args, looksLikeCobolServer) {
		return true
	}

	if looksLikeJava(command) && referencesCobolServer(args) {
		return true
	}

	if looksLikeNode(command) && referencesCobolServer(args) {
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
		when := describeCapabilityWhen(name, "COBOL")
		argsHint := describeCapabilityArgs(name, "COBOL")
		effect := "exec"
		list = append(list, map[string]any{
			"name":        name,
			"when":        when,
			"args":        argsHint,
			"effect":      effect,
			"description": describeCapabilityCommand(name, "COBOL"),
			"available":   true,
			"source":      "server-capabilities",
		})
	}

	return map[string]any{
		"lsp":               "cobol",
		"language":          "COBOL",
		"root":              s.rootDir,
		"commands":          list,
		"availableCommands": commands,
		"count":             len(commands),
		"references":        []string{cobolReferenceURL1, cobolReferenceURL2, cobolReferenceURL3},
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

	when := describeCapabilityWhen(command, "COBOL")
	argsHint := describeCapabilityArgs(command, "COBOL")
	effect := "exec"

	availableCommands, err := s.availableCommands()
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"lsp":               "cobol",
		"root":              s.rootDir,
		"command":           command,
		"commandGuide":      map[string]any{"when": when, "args": argsHint, "effect": effect, "description": describeCapabilityCommand(command, "COBOL")},
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

func looksLikeCobolServer(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	switch base {
	case "rech-editor-cobol", "rech-editor-cobol.exe",
		"cobol-language-server", "cobol-language-server.exe",
		"zopeneditor-language-server", "zopeneditor-language-server.exe":
		return true
	default:
		return strings.Contains(base, "rech-editor-cobol") ||
			strings.Contains(base, "cobol-language-server") ||
			strings.Contains(base, "zopeneditor-language-server")
	}
}

func looksLikeJava(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	return base == "java" || base == "java.exe"
}

func looksLikeNode(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	return base == "node" || base == "node.exe"
}

func referencesCobolServer(args []string) bool {
	return slices.ContainsFunc(args, func(arg string) bool {
		normalized := strings.ToLower(strings.TrimSpace(arg))
		return strings.Contains(normalized, "rech-editor-cobol") ||
			strings.Contains(normalized, "cobol-language-server") ||
			strings.Contains(normalized, "zopeneditor")
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
				"description": "COBOL language-server command name to execute (see cobol_list_extension_commands).",
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
		context = "COBOL"
	}
	context = context + " via " + cobolServerContext
	commandName := strings.TrimSpace(name)
	if commandName == "" {
		return "Run workspace commands only when " + context + " advertises them; semantics and arguments are server-specific."
	}
	return "Run workspace command " + commandName + " only when " + context + " advertises it; semantics and arguments are server-specific."
}

func describeCapabilityArgs(name string, language string) string {
	context := strings.TrimSpace(language)
	if context == "" {
		context = "COBOL"
	}
	context = context + " via " + cobolServerContext
	commandName := strings.TrimSpace(name)
	if commandName == "" {
		return "arguments array expected by " + context + " for the selected command"
	}
	return "arguments array expected by " + context + " for " + commandName
}

func describeCapabilityCommand(name string, language string) string {
	return triadDescription(describeCapabilityWhen(name, language), describeCapabilityArgs(name, language), "exec")
}
