// Package ruby は Ruby 向けの MCP 拡張ツールを提供する。
package ruby

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
	rubySolargraphReferenceURL      = "https://github.com/castwide/solargraph"
	rubyLanguageServerReferenceURL  = "https://github.com/mtsmfm/language_server-ruby"
	rubySorbetReferenceURL          = "https://github.com/sorbet/sorbet"
	rubyOrbacleReferenceURL         = "https://github.com/radarek/orbacle"
	rubyRubyLanguageServerReference = "https://github.com/kwerle/ruby_language_server"
	rubyLSPReferenceURL             = "https://github.com/Shopify/ruby-lsp"
)

// BuildTools は Ruby 言語サーバー向けの拡張ツールを構築する。
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
			Name:        "ruby_list_extension_commands",
			Description: triadDescription("Inspect workspace commands advertised via executeCommandProvider.commands by the connected Ruby language server", "none", "read"),
			InputSchema: emptySchema(),
			Handler:     svc.handleListCommands,
		},
		{
			Name:        "ruby_execute_extension_command",
			Description: triadDescription("Run one workspace command advertised by the connected Ruby language server", "command, arguments?", "exec"),
			InputSchema: executeCommandSchema(),
			Handler:     svc.handleExecuteCommand,
		},
	}
}

// Matches は設定されたコマンド/引数が Ruby 言語サーバーを示す場合に true を返す。
func Matches(command string, args []string) bool {
	if looksLikeRubyServer(command) {
		return true
	}

	if looksLikeSorbetCommand(command) && referencesSorbetLSPMode(args) {
		return true
	}

	if slices.ContainsFunc(args, looksLikeRubyServer) {
		return true
	}

	if slices.ContainsFunc(args, looksLikeSorbetCommand) && referencesSorbetLSPMode(args) {
		return true
	}

	if looksLikeRubyRuntime(command) && referencesRubyServer(args) {
		return true
	}

	if looksLikeBundle(command) && referencesRubyServer(args) {
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
		when := describeCapabilityWhen(name, "Ruby")
		argsHint := describeCapabilityArgs(name, "Ruby")
		effect := "exec"
		list = append(list, map[string]any{
			"name":        name,
			"when":        when,
			"args":        argsHint,
			"effect":      effect,
			"description": describeCapabilityCommand(name, "Ruby"),
			"available":   true,
			"source":      "server-capabilities",
		})
	}

	return map[string]any{
		"lsp":               "ruby",
		"language":          "Ruby",
		"root":              s.rootDir,
		"commands":          list,
		"availableCommands": commands,
		"count":             len(commands),
		"references": []string{
			rubySolargraphReferenceURL,
			rubyLanguageServerReferenceURL,
			rubySorbetReferenceURL,
			rubyOrbacleReferenceURL,
			rubyRubyLanguageServerReference,
			rubyLSPReferenceURL,
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

	when := describeCapabilityWhen(command, "Ruby")
	argsHint := describeCapabilityArgs(command, "Ruby")
	effect := "exec"

	availableCommands, err := s.availableCommands()
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"lsp":               "ruby",
		"root":              s.rootDir,
		"command":           command,
		"commandGuide":      map[string]any{"when": when, "args": argsHint, "effect": effect, "description": describeCapabilityCommand(command, "Ruby")},
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

func looksLikeRubyServer(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))

	switch base {
	case "solargraph", "solargraph.exe", "solargraph.cmd", "solargraph.bat",
		"language_server-ruby", "language_server-ruby.exe", "language_server-ruby.cmd", "language_server-ruby.bat",
		"language-server-ruby", "language-server-ruby.exe", "language-server-ruby.cmd", "language-server-ruby.bat",
		"orbacle", "orbacle.exe", "orbacle.cmd", "orbacle.bat",
		"ruby_language_server", "ruby_language_server.exe", "ruby_language_server.cmd", "ruby_language_server.bat",
		"ruby-language-server", "ruby-language-server.exe", "ruby-language-server.cmd", "ruby-language-server.bat",
		"ruby-lsp", "ruby-lsp.exe", "ruby-lsp.cmd", "ruby-lsp.bat",
		"shopify-ruby-lsp", "shopify-ruby-lsp.exe", "shopify-ruby-lsp.cmd":
		return true
	default:
		return strings.Contains(base, "solargraph") ||
			strings.Contains(base, "language_server-ruby") ||
			strings.Contains(base, "language-server-ruby") ||
			strings.Contains(base, "orbacle") ||
			strings.Contains(base, "ruby_language_server") ||
			strings.Contains(base, "ruby-language-server") ||
			strings.Contains(base, "ruby-lsp") ||
			strings.Contains(normalized, "castwide/solargraph") ||
			strings.Contains(normalized, "castwide\\solargraph") ||
			strings.Contains(normalized, "mtsmfm/language_server-ruby") ||
			strings.Contains(normalized, "mtsmfm\\language_server-ruby") ||
			strings.Contains(normalized, "radarek/orbacle") ||
			strings.Contains(normalized, "radarek\\orbacle") ||
			strings.Contains(normalized, "kwerle/ruby_language_server") ||
			strings.Contains(normalized, "kwerle\\ruby_language_server") ||
			strings.Contains(normalized, "shopify/ruby-lsp") ||
			strings.Contains(normalized, "shopify\\ruby-lsp")
	}
}

func looksLikeSorbetCommand(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	switch base {
	case "sorbet", "sorbet.exe", "sorbet.cmd",
		"srb", "srb.exe", "srb.cmd":
		return true
	default:
		return false
	}
}

func looksLikeRubyRuntime(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	return base == "ruby" || base == "ruby.exe"
}

func looksLikeBundle(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	switch base {
	case "bundle", "bundle.exe", "bundle.cmd", "bundle.bat",
		"bundler", "bundler.exe", "bundler.cmd", "bundler.bat":
		return true
	default:
		return false
	}
}

func referencesRubyServer(args []string) bool {
	if slices.ContainsFunc(args, looksLikeRubyServer) {
		return true
	}

	if referencesRubyServerViaRubySwitch(args) {
		return true
	}

	if slices.ContainsFunc(args, looksLikeSorbetCommand) && referencesSorbetLSPMode(args) {
		return true
	}

	return slices.ContainsFunc(args, func(arg string) bool {
		normalized := strings.ToLower(strings.TrimSpace(arg))
		return strings.Contains(normalized, "solargraph") ||
			strings.Contains(normalized, "language_server-ruby") ||
			strings.Contains(normalized, "language-server-ruby") ||
			strings.Contains(normalized, "orbacle") ||
			strings.Contains(normalized, "ruby_language_server") ||
			strings.Contains(normalized, "ruby-language-server") ||
			strings.Contains(normalized, "ruby-lsp") ||
			strings.Contains(normalized, "castwide/solargraph") ||
			strings.Contains(normalized, "castwide\\solargraph") ||
			strings.Contains(normalized, "mtsmfm/language_server-ruby") ||
			strings.Contains(normalized, "mtsmfm\\language_server-ruby") ||
			strings.Contains(normalized, "radarek/orbacle") ||
			strings.Contains(normalized, "radarek\\orbacle") ||
			strings.Contains(normalized, "kwerle/ruby_language_server") ||
			strings.Contains(normalized, "kwerle\\ruby_language_server") ||
			strings.Contains(normalized, "shopify/ruby-lsp") ||
			strings.Contains(normalized, "shopify\\ruby-lsp")
	})
}

func referencesRubyServerViaRubySwitch(args []string) bool {
	for i, arg := range args {
		normalized := strings.ToLower(strings.TrimSpace(arg))
		if normalized == "-s" && i+1 < len(args) {
			if looksLikeRubyServer(args[i+1]) {
				return true
			}
			if looksLikeSorbetCommand(args[i+1]) && referencesSorbetLSPMode(args) {
				return true
			}
		}
		if strings.HasPrefix(normalized, "-s") && len(normalized) > 2 {
			candidate := strings.TrimPrefix(normalized, "-s")
			if looksLikeRubyServer(candidate) {
				return true
			}
			if looksLikeSorbetCommand(candidate) && referencesSorbetLSPMode(args) {
				return true
			}
		}
	}
	return false
}

func referencesSorbetLSPMode(args []string) bool {
	for _, arg := range args {
		normalized := strings.ToLower(strings.TrimSpace(arg))
		if normalized == "--lsp" || normalized == "--enable-lsp" || normalized == "lsp" {
			return true
		}
		if strings.HasPrefix(normalized, "--lsp=") || strings.HasPrefix(normalized, "--lsp-") {
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
				"description": "Ruby language-server command name to execute (see ruby_list_extension_commands).",
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
