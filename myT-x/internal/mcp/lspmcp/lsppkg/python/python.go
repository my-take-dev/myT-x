// Package python は Python 向けの MCP 拡張ツールを提供する。
package python

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
	pythonTyReferenceURL           = "https://github.com/astral-sh/ruff"
	pythonPyDevReferenceURL        = "https://github.com/fabiozadrozny/PyDev.Debugger"
	pythonPyrightReferenceURL      = "https://github.com/microsoft/pyright"
	pythonPyreflyReferenceURL      = "https://github.com/facebook/pyrefly"
	pythonBasedPyrightReferenceURL = "https://github.com/DetachHead/basedpyright"
	pythonLSPServerReferenceURL    = "https://github.com/python-lsp/python-lsp-server"
	pythonJediReferenceURL         = "https://github.com/pappasam/jedi-language-server"
	pythonPylyzerReferenceURL      = "https://github.com/mtshiba/pylyzer"
	pythonZubanReferenceURL        = "https://github.com/zuban-project/zuban"
)

// BuildTools は Python 言語サーバー向けの拡張ツールを構築する。
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
			Name:        "python_list_extension_commands",
			Description: triadDescription("Inspect workspace commands advertised via executeCommandProvider.commands by the connected Python language server", "none", "read"),
			InputSchema: emptySchema(),
			Handler:     svc.handleListCommands,
		},
		{
			Name:        "python_execute_extension_command",
			Description: triadDescription("Run one workspace command advertised by the connected Python language server", "command, arguments?", "exec"),
			InputSchema: executeCommandSchema(),
			Handler:     svc.handleExecuteCommand,
		},
	}
}

// Matches は設定されたコマンド/引数が Python 言語サーバーを示す場合に true を返す。
func Matches(command string, args []string) bool {
	if looksLikePythonServer(command) {
		return true
	}

	if slices.ContainsFunc(args, looksLikePythonServer) {
		return true
	}

	if looksLikeNode(command) && referencesNodePythonServer(args) {
		return true
	}

	if looksLikePythonRuntime(command) && referencesPythonRuntimeServer(args) {
		return true
	}

	if looksLikeRuff(command) && referencesTyServer(args) {
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
		when := describeCapabilityWhen(name, "Python")
		argsHint := describeCapabilityArgs(name, "Python")
		effect := "exec"
		list = append(list, map[string]any{
			"name":        name,
			"when":        when,
			"args":        argsHint,
			"effect":      effect,
			"description": describeCapabilityCommand(name, "Python"),
			"available":   true,
			"source":      "server-capabilities",
		})
	}

	return map[string]any{
		"lsp":               "python",
		"language":          "Python",
		"root":              s.rootDir,
		"commands":          list,
		"availableCommands": commands,
		"count":             len(commands),
		"references": []string{
			pythonTyReferenceURL,
			pythonPyDevReferenceURL,
			pythonPyrightReferenceURL,
			pythonPyreflyReferenceURL,
			pythonBasedPyrightReferenceURL,
			pythonLSPServerReferenceURL,
			pythonJediReferenceURL,
			pythonPylyzerReferenceURL,
			pythonZubanReferenceURL,
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

	when := describeCapabilityWhen(command, "Python")
	argsHint := describeCapabilityArgs(command, "Python")
	effect := "exec"

	availableCommands, err := s.availableCommands()
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"lsp":               "python",
		"root":              s.rootDir,
		"command":           command,
		"commandGuide":      map[string]any{"when": when, "args": argsHint, "effect": effect, "description": describeCapabilityCommand(command, "Python")},
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

func looksLikePythonServer(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	switch base {
	case "ty", "ty.exe", "ty.cmd",
		"pydev", "pydev.exe", "pydev.cmd",
		"pydevd", "pydevd.exe", "pydevd.cmd", "pydevd.py",
		"pydev-language-server", "pydev-language-server.exe", "pydev-language-server.cmd",
		"pydev-ls", "pydev-ls.exe", "pydev-ls.cmd",
		"pyright-langserver", "pyright-langserver.exe", "pyright-langserver.cmd",
		"pyright-language-server", "pyright-language-server.exe", "pyright-language-server.cmd",
		"pyrefly", "pyrefly.exe", "pyrefly.cmd",
		"pyrefly-lsp", "pyrefly-lsp.exe", "pyrefly-lsp.cmd",
		"basedpyright", "basedpyright.exe", "basedpyright.cmd",
		"basedpyright-langserver", "basedpyright-langserver.exe", "basedpyright-langserver.cmd",
		"pylsp", "pylsp.exe", "pylsp.cmd",
		"python-lsp-server", "python-lsp-server.exe", "python-lsp-server.cmd",
		"jedi-language-server", "jedi-language-server.exe", "jedi-language-server.cmd",
		"pylyzer", "pylyzer.exe", "pylyzer.cmd",
		"zuban", "zuban.exe", "zuban.cmd":
		return true
	default:
		return strings.Contains(base, "pyright-langserver") ||
			strings.Contains(base, "basedpyright-langserver") ||
			strings.Contains(base, "python-lsp-server") ||
			strings.Contains(base, "jedi-language-server") ||
			strings.Contains(base, "pylyzer") ||
			strings.Contains(base, "pyrefly") ||
			strings.Contains(base, "zuban") ||
			strings.Contains(base, "pylsp") ||
			strings.Contains(base, "pydev")
	}
}

func looksLikeNode(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	return base == "node" || base == "node.exe"
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
	if strings.HasPrefix(base, "python") && strings.HasSuffix(base, ".exe") {
		suffix := strings.TrimSuffix(strings.TrimPrefix(base, "python"), ".exe")
		return isDigits(suffix)
	}
	return false
}

func looksLikeRuff(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	return base == "ruff" || base == "ruff.exe"
}

func referencesNodePythonServer(args []string) bool {
	return slices.ContainsFunc(args, func(arg string) bool {
		normalized := strings.ToLower(strings.TrimSpace(arg))
		return strings.Contains(normalized, "pyright/langserver") ||
			strings.Contains(normalized, "pyright\\langserver") ||
			strings.Contains(normalized, "pyright/dist/langserver") ||
			strings.Contains(normalized, "pyright\\dist\\langserver") ||
			strings.Contains(normalized, "pyright-langserver") ||
			strings.Contains(normalized, "microsoft/pyright") ||
			strings.Contains(normalized, "microsoft\\pyright") ||
			strings.Contains(normalized, "basedpyright/langserver") ||
			strings.Contains(normalized, "basedpyright\\langserver") ||
			strings.Contains(normalized, "basedpyright/dist/langserver") ||
			strings.Contains(normalized, "basedpyright\\dist\\langserver") ||
			strings.Contains(normalized, "basedpyright-langserver") ||
			strings.Contains(normalized, "detachhead/basedpyright") ||
			strings.Contains(normalized, "detachhead\\basedpyright")
	})
}

func referencesPythonRuntimeServer(args []string) bool {
	for i, arg := range args {
		normalized := strings.ToLower(strings.TrimSpace(arg))
		if normalized == "-m" && i+1 < len(args) {
			if looksLikePythonServerModule(args[i+1]) {
				return true
			}
			continue
		}
		if strings.HasPrefix(normalized, "-m") && len(normalized) > 2 {
			if looksLikePythonServerModule(strings.TrimPrefix(normalized, "-m")) {
				return true
			}
			continue
		}
		if looksLikePythonServerModule(normalized) {
			return true
		}
	}
	return false
}

func referencesTyServer(args []string) bool {
	return slices.ContainsFunc(args, func(arg string) bool {
		normalized := strings.ToLower(strings.TrimSpace(arg))
		return normalized == "ty" ||
			strings.HasSuffix(normalized, "/ty") ||
			strings.HasSuffix(normalized, "\\ty") ||
			strings.Contains(normalized, "astral-sh/ruff")
	})
}

func looksLikePythonServerModule(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "pylsp", "python_lsp_server", "python-lsp-server",
		"jedi_language_server", "jedi-language-server",
		"pyright.langserver", "basedpyright.langserver",
		"pyrefly", "pylyzer", "zuban",
		"pydev", "pydevd", "pydevd.py":
		return true
	default:
		return strings.Contains(normalized, "python-lsp-server") ||
			strings.Contains(normalized, "python_lsp_server") ||
			strings.Contains(normalized, "jedi-language-server") ||
			strings.Contains(normalized, "jedi_language_server") ||
			strings.Contains(normalized, "pyright.langserver") ||
			strings.Contains(normalized, "basedpyright.langserver") ||
			strings.Contains(normalized, "pyrefly") ||
			strings.Contains(normalized, "pylyzer") ||
			strings.Contains(normalized, "zuban") ||
			strings.Contains(normalized, "pydev")
	}
}

func isDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
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
				"description": "Python language-server command name to execute (see python_list_extension_commands).",
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
