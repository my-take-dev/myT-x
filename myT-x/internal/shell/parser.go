package shell

import (
	"log/slog"
	"runtime"
	"sort"
	"strings"
)

// ParsedCommand holds parsed components of a Unix-style command translated for PowerShell.
type ParsedCommand struct {
	WorkDir   string            // extracted from "cd 'path' && ..."
	ExtraEnv  map[string]string // extracted from "KEY=VALUE" prefixes
	CleanArgs []string          // PowerShell-compatible command to execute
}

var sendKeysSpecialArgs = map[string]struct{}{
	"Enter":  {},
	"C-c":    {},
	"C-d":    {},
	"C-z":    {},
	"Escape": {},
	"Space":  {},
	"Tab":    {},
	"BSpace": {},
}

// ParseUnixCommand translates bash-style command args to Windows PowerShell-compatible args.
//
// On Windows:
//  1. Extract "cd 'path'" into WorkDir.
//  2. Extract leading KEY=VALUE tokens into ExtraEnv.
//  3. Add "& " for quoted executable paths.
//  4. Replace remaining " && " with " ; ".
//
// On non-Windows, args are returned unchanged.
func ParseUnixCommand(args []string, currentWorkDir string) ParsedCommand {
	if runtime.GOOS != "windows" {
		return ParsedCommand{
			ExtraEnv:  map[string]string{},
			CleanArgs: args,
		}
	}
	if len(args) == 0 {
		return ParsedCommand{
			ExtraEnv:  map[string]string{},
			CleanArgs: nil,
		}
	}

	cmd := strings.Join(args, " ")
	result := parseCommandCore(cmd)

	if result.WorkDir == "" && currentWorkDir != "" {
		result.WorkDir = currentWorkDir
	}

	slog.Debug("[DEBUG-SHELL] parsed unix shell command",
		"original", cmd,
		"workDir", result.WorkDir,
		"extraEnv", result.ExtraEnv,
		"cleanArgs", result.CleanArgs,
	)

	return result
}

// parseCommandCore performs the parser core logic. Kept unexported for focused tests.
func parseCommandCore(cmd string) ParsedCommand {
	result := ParsedCommand{
		ExtraEnv: map[string]string{},
	}

	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return result
	}

	remaining := cmd

	// Step 1: Extract "cd 'path' && " or "cd \"path\" && " or "cd path && ".
	if strings.HasPrefix(remaining, "cd ") {
		afterCD := remaining[3:]
		if path, rest, ok := extractCDPath(afterCD); ok {
			result.WorkDir = path
			remaining = strings.TrimSpace(rest)
		}
	}

	// Step 2: Extract "KEY=VALUE" prefixes.
	remaining = extractEnvVars(remaining, result.ExtraEnv)

	// Step 3: Add "& " for quoted executable paths in PowerShell.
	remaining = addCallOperatorIfNeeded(remaining)

	if remaining != "" {
		result.CleanArgs = []string{remaining}
	}

	// Step 4: Fallback for PowerShell 5.1.
	if len(result.CleanArgs) > 0 && strings.Contains(result.CleanArgs[0], " && ") {
		result.CleanArgs[0] = strings.ReplaceAll(result.CleanArgs[0], " && ", " ; ")
	}

	return result
}

// extractCDPath extracts path and tail from the text after "cd ".
// Handles: 'path' && rest, "path" && rest, path && rest.
func extractCDPath(afterCD string) (string, string, bool) {
	afterCD = strings.TrimSpace(afterCD)
	if afterCD == "" {
		return "", "", false
	}

	var path string
	var afterPath string

	switch afterCD[0] {
	case '\'':
		end := strings.Index(afterCD[1:], "'")
		if end < 0 {
			return "", "", false
		}
		path = afterCD[1 : end+1]
		afterPath = strings.TrimSpace(afterCD[end+2:])
	case '"':
		end := strings.Index(afterCD[1:], "\"")
		if end < 0 {
			return "", "", false
		}
		path = afterCD[1 : end+1]
		afterPath = strings.TrimSpace(afterCD[end+2:])
	default:
		sep := strings.Index(afterCD, " && ")
		if sep < 0 {
			return "", "", false
		}
		path = strings.TrimSpace(afterCD[:sep])
		afterPath = strings.TrimSpace(afterCD[sep:])
	}

	if !strings.HasPrefix(afterPath, "&& ") && afterPath != "&&" && !strings.HasPrefix(afterPath, "&&") {
		return "", "", false
	}

	rest := strings.TrimSpace(strings.TrimPrefix(afterPath, "&&"))
	return path, rest, true
}

// extractEnvVars extracts leading KEY=VALUE pairs.
// Also strips a leading "env " prefix (Unix env command) when followed by KEY=VALUE.
func extractEnvVars(cmd string, envMap map[string]string) string {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return ""
	}

	// Strip leading "env " prefix (Unix env command).
	// Only strip when the next token is a KEY=VALUE assignment,
	// so we don't accidentally strip "env" used as a command name.
	if strings.HasPrefix(cmd, "env ") {
		candidate := strings.TrimSpace(cmd[4:])
		token, _ := nextToken(candidate)
		if key, _, ok := strings.Cut(token, "="); ok && isEnvVarName(key) {
			cmd = candidate
		}
	}

	for {
		token, rest := nextToken(cmd)
		if token == "" {
			break
		}

		key, value, ok := strings.Cut(token, "=")
		if !ok || key == "" || !isEnvVarName(key) {
			break
		}

		envMap[key] = value
		cmd = strings.TrimSpace(rest)
	}

	return cmd
}

// isEnvVarName checks [A-Za-z_][A-Za-z0-9_]*.
func isEnvVarName(s string) bool {
	if s == "" {
		return false
	}
	for i, c := range s {
		if i == 0 {
			if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '_') {
				return false
			}
		} else if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}
	return true
}

// nextToken returns the next space-delimited token and the remaining suffix.
func nextToken(s string) (string, string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ""
	}

	if s[0] == '\'' || s[0] == '"' || s[0] == '-' {
		return "", s
	}

	idx := strings.IndexByte(s, ' ')
	if idx < 0 {
		return s, ""
	}
	return s[:idx], s[idx+1:]
}

func addCallOperatorIfNeeded(cmd string) string {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return ""
	}
	if cmd[0] == '\'' || cmd[0] == '"' {
		return "& " + cmd
	}
	return cmd
}

// TranslateSendKeysArgs translates bash-style send-keys args to PowerShell-friendly args.
//
// Input:  ["cd 'path' && KEY=VAL 'exe' --flags", "Enter"]
// Output: ["cd 'path'; $env:KEY='VAL'; & 'exe' --flags", "Enter"]
//
// On non-Windows, args are returned unchanged.
func TranslateSendKeysArgs(args []string) []string {
	if runtime.GOOS != "windows" || len(args) == 0 {
		return args
	}

	cmdIdx := -1
	for i, arg := range args {
		if _, isSpecial := sendKeysSpecialArgs[arg]; isSpecial {
			continue
		}
		if strings.Contains(arg, "&&") || containsEnvAssignment(arg) {
			cmdIdx = i
			break
		}
	}

	if cmdIdx < 0 {
		return args
	}

	translated := translateBashToPowerShell(args[cmdIdx])
	if translated == args[cmdIdx] {
		return args
	}

	out := make([]string, len(args))
	copy(out, args)
	out[cmdIdx] = translated

	slog.Debug("[DEBUG-SHELL] translated send-keys command for PowerShell",
		"original", args[cmdIdx][:min(len(args[cmdIdx]), 200)],
		"translated", translated[:min(len(translated), 200)],
	)

	return out
}

// translateBashToPowerShell converts:
// cd 'path' && KEY=VAL 'exe' --flags
// ->
// cd 'path'; $env:KEY='VAL'; & 'exe' --flags
func translateBashToPowerShell(cmd string) string {
	parsed := parseCommandCore(cmd)

	var parts []string
	if parsed.WorkDir != "" {
		parts = append(parts, "cd '"+parsed.WorkDir+"'")
	}
	if len(parsed.ExtraEnv) > 0 {
		keys := make([]string, 0, len(parsed.ExtraEnv))
		for key := range parsed.ExtraEnv {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			parts = append(parts, "$env:"+key+"='"+parsed.ExtraEnv[key]+"'")
		}
	}
	if len(parsed.CleanArgs) > 0 {
		parts = append(parts, parsed.CleanArgs[0])
	}

	if len(parts) == 0 {
		return cmd
	}
	return strings.Join(parts, "; ")
}

// containsEnvAssignment checks if s looks like it contains KEY=VALUE assignment.
func containsEnvAssignment(s string) bool {
	for i := 0; i < len(s)-1; i++ {
		if s[i] != '=' || i == 0 {
			continue
		}
		start := i - 1
		for start > 0 && (s[start-1] == '_' ||
			(s[start-1] >= 'A' && s[start-1] <= 'Z') ||
			(s[start-1] >= 'a' && s[start-1] <= 'z') ||
			(s[start-1] >= '0' && s[start-1] <= '9')) {
			start--
		}
		if start < i && (start == 0 || s[start-1] == ' ') {
			token := s[start:i]
			if isEnvVarName(token) {
				return true
			}
		}
	}
	return false
}
