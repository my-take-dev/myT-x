// Package gopls は gopls 向けの MCP 拡張ツールを提供する。
package gopls

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

type commandSpec struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

const (
	goplsOverviewURL = "https://go.dev/gopls/"
	goplsFeaturesURL = "https://go.dev/gopls/features/"
)

// staticCommandCatalog は gopls コマンドドキュメントに基づく安定したコマンドメタデータを保持する。
var staticCommandCatalog = map[string]commandSpec{
	"gopls.add_dependency": {
		Name:        "gopls.add_dependency",
		Description: "Add a dependency to go.mod for the current module.",
	},
	"gopls.add_import": {
		Name:        "gopls.add_import",
		Description: "Add an import path to a Go file (the server applies the edit).",
	},
	"gopls.add_telemetry_counters": {
		Name:        "gopls.add_telemetry_counters",
		Description: "Update gopls telemetry counters (forwarded as fwd/* counters).",
	},
	"gopls.add_test": {
		Name:        "gopls.add_test",
		Description: "Generate a test skeleton for the selected function.",
	},
	"gopls.apply_fix": {
		Name:        "gopls.apply_fix",
		Description: "Apply a named fix or quick-fix in a selected source range.",
	},
	"gopls.assembly": {
		Name:        "gopls.assembly",
		Description: "Open a web-based assembly listing for a function symbol.",
	},
	"gopls.change_signature": {
		Name:        "gopls.change_signature",
		Description: "Experimental refactor to change function signatures.",
	},
	"gopls.check_upgrades": {
		Name:        "gopls.check_upgrades",
		Description: "Check available dependency upgrades for the module.",
	},
	"gopls.client_open_url": {
		Name:        "gopls.client_open_url",
		Description: "Ask the client/editor to open a URL in a browser.",
	},
	"gopls.diagnose_files": {
		Name:        "gopls.diagnose_files",
		Description: "Force gopls to publish diagnostics for specified files.",
	},
	"gopls.doc": {
		Name:        "gopls.doc",
		Description: "Open package documentation for the current location.",
	},
	"gopls.edit_go_directive": {
		Name:        "gopls.edit_go_directive",
		Description: "Run `go mod edit -go=<version>` for a module.",
	},
	"gopls.extract_to_new_file": {
		Name:        "gopls.extract_to_new_file",
		Description: "Move selected declarations into a new file.",
	},
	"gopls.fetch_vulncheck_result": {
		Name:        "gopls.fetch_vulncheck_result",
		Description: "Fetch the latest cached govulncheck result (deprecated; prefer gopls.vulncheck).",
	},
	"gopls.free_symbols": {
		Name:        "gopls.free_symbols",
		Description: "Open a browser view of free symbols referenced by the selection.",
	},
	"gopls.gc_details": {
		Name:        "gopls.gc_details",
		Description: "Toggle compiler optimization diagnostics for a package.",
	},
	"gopls.generate": {
		Name:        "gopls.generate",
		Description: "Run `go generate` for a directory.",
	},
	"gopls.go_get_package": {
		Name:        "gopls.go_get_package",
		Description: "Run `go get` to fetch a package.",
	},
	"gopls.list_imports": {
		Name:        "gopls.list_imports",
		Description: "List imports in the file and its enclosing package.",
	},
	"gopls.list_known_packages": {
		Name:        "gopls.list_known_packages",
		Description: "List importable packages from the current file context.",
	},
	"gopls.lsp": {
		Name:        "gopls.lsp",
		Description: "Dispatch an arbitrary LSP method through workspace/executeCommand.",
	},
	"gopls.maybe_prompt_for_telemetry": {
		Name:        "gopls.maybe_prompt_for_telemetry",
		Description: "Prompt the user to enable Go telemetry when conditions are met.",
	},
	"gopls.mem_stats": {
		Name:        "gopls.mem_stats",
		Description: "Collect runtime memory statistics (benchmark/debug oriented).",
	},
	"gopls.modify_tags": {
		Name:        "gopls.modify_tags",
		Description: "Add, remove, or transform struct tags.",
	},
	"gopls.modules": {
		Name:        "gopls.modules",
		Description: "Return module information within a directory.",
	},
	"gopls.move_type": {
		Name:        "gopls.move_type",
		Description: "Move a type declaration to a different package.",
	},
	"gopls.package_symbols": {
		Name:        "gopls.package_symbols",
		Description: "Return symbols from the package of a given file.",
	},
	"gopls.packages": {
		Name:        "gopls.packages",
		Description: "Return package metadata for files or directories.",
	},
	"gopls.regenerate_cgo": {
		Name:        "gopls.regenerate_cgo",
		Description: "Regenerate cgo definitions.",
	},
	"gopls.remove_dependency": {
		Name:        "gopls.remove_dependency",
		Description: "Remove a dependency from go.mod.",
	},
	"gopls.reset_go_mod_diagnostics": {
		Name:        "gopls.reset_go_mod_diagnostics",
		Description: "Reset resettable go.mod diagnostics.",
	},
	"gopls.run_go_work_command": {
		Name:        "gopls.run_go_work_command",
		Description: "Run `go work` subcommands and apply resulting go.work edits.",
	},
	"gopls.run_govulncheck": {
		Name:        "gopls.run_govulncheck",
		Description: "Run govulncheck asynchronously (deprecated; prefer gopls.vulncheck).",
	},
	"gopls.run_tests": {
		Name:        "gopls.run_tests",
		Description: "Run `go test` for selected tests or benchmarks.",
	},
	"gopls.scan_imports": {
		Name:        "gopls.scan_imports",
		Description: "Force a synchronous scan of the imports cache.",
	},
	"gopls.split_package": {
		Name:        "gopls.split_package",
		Description: "Open a web tool to split one package into components.",
	},
	"gopls.start_debugging": {
		Name:        "gopls.start_debugging",
		Description: "Start the gopls debug server and return debug URLs.",
	},
	"gopls.start_profile": {
		Name:        "gopls.start_profile",
		Description: "Start capturing a pprof profile for gopls.",
	},
	"gopls.stop_profile": {
		Name:        "gopls.stop_profile",
		Description: "Stop an active gopls profile capture.",
	},
	"gopls.test": {
		Name:        "gopls.test",
		Description: "Legacy command to run a selected test function (availability depends on gopls version).",
	},
	"gopls.tidy": {
		Name:        "gopls.tidy",
		Description: "Run go mod tidy via gopls command execution.",
	},
	"gopls.update_go_sum": {
		Name:        "gopls.update_go_sum",
		Description: "Update go.sum entries for the current module.",
	},
	"gopls.upgrade_dependency": {
		Name:        "gopls.upgrade_dependency",
		Description: "Upgrade a dependency version in go.mod.",
	},
	"gopls.vendor": {
		Name:        "gopls.vendor",
		Description: "Refresh vendor directory from module dependencies.",
	},
	"gopls.views": {
		Name:        "gopls.views",
		Description: "List active gopls views in the current server session.",
	},
	"gopls.vulncheck": {
		Name:        "gopls.vulncheck",
		Description: "Run govulncheck synchronously and return vulnerability results.",
	},
	"gopls.workspace_stats": {
		Name:        "gopls.workspace_stats",
		Description: "Return workspace statistics (files/modules/packages/diagnostics).",
	},
}

var goplsCommandArgsHints = map[string]string{
	"gopls.add_dependency":             "module path (+ version optional)",
	"gopls.add_import":                 "file URI, import path",
	"gopls.add_test":                   "file URI + function/range",
	"gopls.apply_fix":                  "file URI, range, fix id",
	"gopls.assembly":                   "file URI + symbol location",
	"gopls.change_signature":           "file URI + signature edits",
	"gopls.client_open_url":            "URL",
	"gopls.diagnose_files":             "file URIs",
	"gopls.doc":                        "file URI + position",
	"gopls.edit_go_directive":          "module directory, go version",
	"gopls.extract_to_new_file":        "file URI + declarations/range + target file",
	"gopls.free_symbols":               "file URI + range",
	"gopls.generate":                   "directory URI",
	"gopls.go_get_package":             "package/module path (+ version optional)",
	"gopls.list_imports":               "file URI",
	"gopls.list_known_packages":        "file URI",
	"gopls.lsp":                        "LSP method + params",
	"gopls.modify_tags":                "file URI + struct range + tag options",
	"gopls.modules":                    "directory URI",
	"gopls.move_type":                  "file URI + type + target package",
	"gopls.package_symbols":            "file URI",
	"gopls.packages":                   "file or directory URIs",
	"gopls.regenerate_cgo":             "file/package URI",
	"gopls.remove_dependency":          "module path",
	"gopls.run_go_work_command":        "go.work directory + subcommand args",
	"gopls.run_tests":                  "file URI + test/benchmark names",
	"gopls.tidy":                       "module directory URI",
	"gopls.update_go_sum":              "module directory URI",
	"gopls.upgrade_dependency":         "module path (+ version)",
	"gopls.vulncheck":                  "package/module pattern",
	"gopls.fetch_vulncheck_result":     "none",
	"gopls.add_telemetry_counters":     "none",
	"gopls.check_upgrades":             "none",
	"gopls.mem_stats":                  "none",
	"gopls.reset_go_mod_diagnostics":   "none",
	"gopls.scan_imports":               "none",
	"gopls.start_debugging":            "none",
	"gopls.start_profile":              "profile type/path (optional)",
	"gopls.stop_profile":               "none",
	"gopls.test":                       "file URI + test function",
	"gopls.vendor":                     "module directory URI",
	"gopls.views":                      "none",
	"gopls.workspace_stats":            "none",
	"gopls.run_govulncheck":            "package/module pattern (optional)",
	"gopls.split_package":              "file URI/package",
	"gopls.maybe_prompt_for_telemetry": "none",
	"gopls.gc_details":                 "package URI (optional)",
}

var goplsReadEffects = map[string]struct{}{
	"gopls.diagnose_files":         {},
	"gopls.fetch_vulncheck_result": {},
	"gopls.list_imports":           {},
	"gopls.list_known_packages":    {},
	"gopls.mem_stats":              {},
	"gopls.modules":                {},
	"gopls.package_symbols":        {},
	"gopls.packages":               {},
	"gopls.views":                  {},
	"gopls.workspace_stats":        {},
}

var goplsEditEffects = map[string]struct{}{
	"gopls.add_dependency":      {},
	"gopls.add_import":          {},
	"gopls.add_test":            {},
	"gopls.apply_fix":           {},
	"gopls.change_signature":    {},
	"gopls.edit_go_directive":   {},
	"gopls.extract_to_new_file": {},
	"gopls.go_get_package":      {},
	"gopls.modify_tags":         {},
	"gopls.move_type":           {},
	"gopls.regenerate_cgo":      {},
	"gopls.remove_dependency":   {},
	"gopls.run_go_work_command": {},
	"gopls.tidy":                {},
	"gopls.update_go_sum":       {},
	"gopls.upgrade_dependency":  {},
	"gopls.vendor":              {},
}

func init() {
	for k := range goplsReadEffects {
		if _, ok := goplsEditEffects[k]; ok {
			panic("gopls: goplsReadEffects and goplsEditEffects must not overlap: " + k)
		}
	}
}

// BuildTools は gopls 言語サーバー向けの拡張ツールを構築する。
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
			Name:        "gopls_list_extension_commands",
			Description: goplsTriadDescription("Inspect gopls workspace commands from executeCommandProvider.commands with commandGuide metadata", "none", "read"),
			InputSchema: emptySchema(),
			Handler:     svc.handleListCommands,
		},
		{
			Name:        "gopls_execute_command",
			Description: goplsTriadDescription("Run one workspace command advertised by the connected gopls language server (prefer this over lsp_execute_command for gopls because commandGuide is returned)", "command, arguments?", "exec"),
			InputSchema: executeCommandSchema(),
			Handler:     svc.handleExecuteCommand,
		},
	}
}

// Matches は設定されたコマンド/引数が gopls を示す場合に true を返す。
func Matches(command string, args []string) bool {
	if looksLikeGopls(command) {
		return true
	}

	if slices.ContainsFunc(args, looksLikeGopls) {
		return true
	}

	if looksLikeGo(command) && len(args) >= 2 && strings.EqualFold(strings.TrimSpace(args[0]), "tool") && looksLikeGopls(args[1]) {
		return true
	}

	return false
}

type service struct {
	client  *lsp.Client
	rootDir string
}

// commandEntry はコマンド一覧の1エントリを表す map を構築する。
func commandEntry(name, when, args, effect, description, source string, available bool) map[string]any {
	return map[string]any{
		"name":        name,
		"description": description,
		"when":        when,
		"args":        args,
		"effect":      effect,
		"available":   available,
		"source":      source,
	}
}

func (s *service) handleListCommands(_ context.Context, _ map[string]any) (any, error) {
	staticCommands := sortedStaticCommands()
	availableCommands, err := s.availableCommands()
	if err != nil {
		return nil, err
	}
	availableSet := make(map[string]struct{}, len(availableCommands))
	for _, name := range availableCommands {
		availableSet[name] = struct{}{}
	}

	out := make([]map[string]any, 0, len(staticCommands)+len(availableCommands))
	for _, spec := range staticCommands {
		_, available := availableSet[spec.Name]
		when, args, effect, description := goplsCommandGuide(spec.Name, spec.Description)
		out = append(out, commandEntry(spec.Name, when, args, effect, description, "static-catalog", available))
	}

	known := make(map[string]struct{}, len(staticCommands))
	for _, spec := range staticCommands {
		known[spec.Name] = struct{}{}
	}
	for _, name := range availableCommands {
		if _, exists := known[name]; exists {
			continue
		}
		when, args, effect, description := unknownCommandGuide()
		out = append(out, commandEntry(name, when, args, effect, description, "server-capabilities", true))
	}

	return map[string]any{
		"lsp":                "gopls",
		"language":           "Go",
		"root":               s.rootDir,
		"commands":           out,
		"availableCommands":  availableCommands,
		"staticCatalogCount": len(staticCommands),
		"references":         []string{goplsOverviewURL, goplsFeaturesURL},
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

	spec, known := staticCommandCatalog[command]
	var when, argsHint, effect, description string
	if known {
		when, argsHint, effect, description = goplsCommandGuide(spec.Name, spec.Description)
	} else {
		when, argsHint, effect, description = unknownCommandGuide()
	}

	availableCommands, err := s.availableCommands()
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"lsp":               "gopls",
		"root":              s.rootDir,
		"command":           command,
		"knownInCatalog":    known,
		"catalogEntry":      spec,
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

func sortedStaticCommands() []commandSpec {
	names := make([]string, 0, len(staticCommandCatalog))
	for name := range staticCommandCatalog {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]commandSpec, 0, len(names))
	for _, name := range names {
		out = append(out, staticCommandCatalog[name])
	}
	return out
}

func looksLikeGopls(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	return base == "gopls" || base == "gopls.exe"
}

func looksLikeGo(value string) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(value)))
	return base == "go" || base == "go.exe"
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

func goplsTriadDescription(when string, args string, effect string) string {
	return "when: " + strings.TrimSpace(when) + " args: " + strings.TrimSpace(args) + " effect: " + strings.TrimSpace(effect) + "."
}

func goplsCommandGuide(name string, summary string) (string, string, string, string) {
	when := strings.TrimSpace(strings.TrimSuffix(summary, "."))
	if when == "" {
		when = "Use this command when its gopls-specific behavior is needed"
	}
	args := "command-specific payload"
	if hint, ok := goplsCommandArgsHints[name]; ok {
		args = hint
	}
	effect := goplsCommandEffect(name)
	return when, args, effect, goplsTriadDescription(when, args, effect)
}

func unknownCommandGuide() (string, string, string, string) {
	when := "Use when this gopls build advertises a command outside the static catalog"
	args := "command-specific payload"
	effect := "exec"
	return when, args, effect, goplsTriadDescription(when, args, effect)
}

func goplsCommandEffect(name string) string {
	if _, ok := goplsReadEffects[name]; ok {
		return "read"
	}
	if _, ok := goplsEditEffects[name]; ok {
		return "edit"
	}
	return "exec"
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
				"description": "gopls command name to execute (see gopls_list_extension_commands).",
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
