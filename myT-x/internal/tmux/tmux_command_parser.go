package tmux

import (
	"log/slog"

	"myT-x/internal/ipc"
)

// tmuxFlagKind identifies the type of a tmux command flag for internal parsing.
type tmuxFlagKind int

const (
	tmuxFlagBool   tmuxFlagKind = iota // boolean flag (no value)
	tmuxFlagString                     // string flag (takes next arg as value)
)

// internalCommandFlagSpecs defines flag types for all supported tmux commands.
// Used by parseTmuxCommandLine to correctly separate flags from positional args
// when dispatching run-shell -C and if-shell commands internally.
//
// NOTE: This corresponds to cmd/tmux-shim/spec.go but is not a 1:1 mirror.
// Differences: (1) flagInt and flagEnv from spec.go are both mapped to
// tmuxFlagString here since the internal parser only needs to know whether
// a flag consumes the next token or not. (2) If a command or flag is added
// in spec.go, it should be added here as well.
var internalCommandFlagSpecs = map[string]map[string]tmuxFlagKind{
	"new-session": {
		"-d": tmuxFlagBool, "-P": tmuxFlagBool,
		"-F": tmuxFlagString, "-s": tmuxFlagString, "-n": tmuxFlagString,
		"-x": tmuxFlagString, "-y": tmuxFlagString, "-c": tmuxFlagString,
		"-e": tmuxFlagString,
	},
	"has-session":      {"-t": tmuxFlagString},
	"split-window":     {"-h": tmuxFlagBool, "-v": tmuxFlagBool, "-d": tmuxFlagBool, "-P": tmuxFlagBool, "-F": tmuxFlagString, "-t": tmuxFlagString, "-c": tmuxFlagString, "-e": tmuxFlagString, "-l": tmuxFlagString, "-p": tmuxFlagString},
	"send-keys":        {"-t": tmuxFlagString, "-l": tmuxFlagBool, "-X": tmuxFlagBool, "-M": tmuxFlagBool, "-W": tmuxFlagBool, "-N": tmuxFlagBool},
	"select-pane":      {"-t": tmuxFlagString, "-T": tmuxFlagString, "-U": tmuxFlagBool, "-D": tmuxFlagBool, "-L": tmuxFlagBool, "-R": tmuxFlagBool},
	"list-sessions":    {"-F": tmuxFlagString, "-f": tmuxFlagString},
	"kill-session":     {"-t": tmuxFlagString, "-a": tmuxFlagBool},
	"list-panes":       {"-t": tmuxFlagString, "-s": tmuxFlagBool, "-a": tmuxFlagBool, "-F": tmuxFlagString, "-f": tmuxFlagString},
	"display-message":  {"-p": tmuxFlagBool, "-t": tmuxFlagString},
	"attach-session":   {"-t": tmuxFlagString},
	"kill-pane":        {"-t": tmuxFlagString},
	"rename-session":   {"-t": tmuxFlagString},
	"resize-pane":      {"-t": tmuxFlagString, "-x": tmuxFlagString, "-y": tmuxFlagString, "-U": tmuxFlagBool, "-D": tmuxFlagBool, "-L": tmuxFlagBool, "-R": tmuxFlagBool, "-Z": tmuxFlagBool},
	"show-environment": {"-t": tmuxFlagString, "-g": tmuxFlagBool},
	"set-environment":  {"-t": tmuxFlagString, "-u": tmuxFlagBool, "-g": tmuxFlagBool},
	"list-windows":     {"-t": tmuxFlagString, "-a": tmuxFlagBool, "-F": tmuxFlagString, "-f": tmuxFlagString},
	"rename-window":    {"-t": tmuxFlagString},
	"new-window":       {"-d": tmuxFlagBool, "-P": tmuxFlagBool, "-F": tmuxFlagString, "-n": tmuxFlagString, "-t": tmuxFlagString, "-c": tmuxFlagString, "-e": tmuxFlagString},
	"kill-window":      {"-t": tmuxFlagString},
	"select-window":    {"-t": tmuxFlagString},
	"copy-mode":        {"-t": tmuxFlagString, "-q": tmuxFlagBool, "-u": tmuxFlagBool, "-e": tmuxFlagBool},
	"list-buffers":     {"-F": tmuxFlagString},
	"set-buffer":       {"-a": tmuxFlagBool, "-b": tmuxFlagString, "-n": tmuxFlagString},
	"paste-buffer":     {"-d": tmuxFlagBool, "-b": tmuxFlagString, "-t": tmuxFlagString, "-p": tmuxFlagBool, "-r": tmuxFlagBool, "-s": tmuxFlagString},
	"load-buffer":      {"-b": tmuxFlagString, "-w": tmuxFlagBool, "-t": tmuxFlagString},
	"save-buffer":      {"-a": tmuxFlagBool, "-b": tmuxFlagString},
	"capture-pane":     {"-a": tmuxFlagBool, "-b": tmuxFlagString, "-C": tmuxFlagBool, "-e": tmuxFlagBool, "-E": tmuxFlagString, "-J": tmuxFlagBool, "-M": tmuxFlagBool, "-N": tmuxFlagBool, "-p": tmuxFlagBool, "-P": tmuxFlagBool, "-q": tmuxFlagBool, "-S": tmuxFlagString, "-T": tmuxFlagBool, "-t": tmuxFlagString},
	"run-shell":        {"-b": tmuxFlagBool, "-t": tmuxFlagString, "-C": tmuxFlagBool, "-c": tmuxFlagString},
	"if-shell":         {"-b": tmuxFlagBool, "-F": tmuxFlagBool, "-t": tmuxFlagString},
}

// splitTmuxCommands splits a tmux command string on unquoted semicolons.
// Quoted sections (single or double quotes) are preserved as-is.
// This matches tmux's behavior where ';' separates commands but quoted
// semicolons are treated as literal characters.
func splitTmuxCommands(s string) []string {
	var parts []string
	var current []byte
	inSingle := false
	inDouble := false

	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case ch == '\'' && !inDouble:
			inSingle = !inSingle
			current = append(current, ch)
		case ch == '"' && !inSingle:
			inDouble = !inDouble
			current = append(current, ch)
		case ch == ';' && !inSingle && !inDouble:
			parts = append(parts, string(current))
			current = current[:0]
		default:
			current = append(current, ch)
		}
	}
	if len(current) > 0 {
		parts = append(parts, string(current))
	}
	return parts
}

// tokenizeTmuxCommand splits a single tmux command string into tokens,
// respecting single and double quotes. Quotes are stripped from tokens.
func tokenizeTmuxCommand(s string) []string {
	var tokens []string
	var current []byte
	inSingle := false
	inDouble := false
	inToken := false

	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case ch == '\'' && !inDouble:
			inSingle = !inSingle
			inToken = true
			// Don't append the quote character itself
		case ch == '"' && !inSingle:
			inDouble = !inDouble
			inToken = true
			// Don't append the quote character itself
		case (ch == ' ' || ch == '\t') && !inSingle && !inDouble:
			if inToken {
				tokens = append(tokens, string(current))
				current = current[:0]
				inToken = false
			}
		default:
			current = append(current, ch)
			inToken = true
		}
	}
	if inToken {
		tokens = append(tokens, string(current))
	}
	return tokens
}

// parseTmuxCommandLine parses a raw tmux command string into a TmuxRequest
// with properly separated Command, Flags, and Args fields.
// Unknown commands are parsed with all tokens as Args (best-effort forwarding).
func parseTmuxCommandLine(line string) ipc.TmuxRequest {
	tokens := tokenizeTmuxCommand(line)
	if len(tokens) == 0 {
		return ipc.TmuxRequest{
			Flags: map[string]any{},
			Env:   map[string]string{},
		}
	}

	command := tokens[0]
	rest := tokens[1:]

	flags := map[string]any{}
	var args []string

	spec, hasSpec := internalCommandFlagSpecs[command]
	if !hasSpec {
		// Unknown command: pass all remaining tokens as args.
		return ipc.TmuxRequest{
			Command: command,
			Args:    rest,
			Flags:   flags,
			Env:     map[string]string{},
		}
	}

	for i := 0; i < len(rest); i++ {
		token := rest[i]
		if len(token) == 0 {
			continue
		}

		kind, isFlag := spec[token]
		if !isFlag {
			// Not a known flag: treat as positional arg.
			args = append(args, token)
			continue
		}

		switch kind {
		case tmuxFlagBool:
			flags[token] = true
		case tmuxFlagString:
			if i+1 < len(rest) {
				i++
				flags[token] = rest[i]
			} else {
				slog.Debug("[DEBUG-PARSER] string flag missing value, ignoring",
					"command", command, "flag", token)
			}
		}
	}

	if args == nil {
		args = []string{}
	}

	return ipc.TmuxRequest{
		Command: command,
		Args:    args,
		Flags:   flags,
		Env:     map[string]string{},
	}
}
