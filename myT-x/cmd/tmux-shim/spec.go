package main

import "fmt"

type flagKind int

const (
	flagBool flagKind = iota
	flagString
	flagInt
	flagEnv
)

type commandSpec struct {
	description string
	flags       map[string]flagKind
}

var commandSpecs = map[string]commandSpec{
	"new-session": {
		description: "Create a new session. Common flags: -s name, -c dir, -d detached.",
		flags: map[string]flagKind{
			"-d": flagBool,
			"-P": flagBool,
			"-F": flagString,
			"-s": flagString,
			"-n": flagString,
			"-x": flagInt,
			"-y": flagInt,
			"-c": flagString,
			"-e": flagEnv,
		},
	},
	"has-session": {
		description: "Check whether the target session exists.",
		flags: map[string]flagKind{
			"-t": flagString,
		},
	},
	"split-window": {
		description: "Split the target pane. Common flags: -h horizontal, -v vertical, -c dir.",
		flags: map[string]flagKind{
			"-h": flagBool,
			"-v": flagBool,
			"-d": flagBool,
			"-P": flagBool,
			"-F": flagString,
			"-t": flagString,
			"-c": flagString,
			"-e": flagEnv,
			"-l": flagString,
			"-p": flagString,
		},
	},
	"send-keys": {
		description: "Send key input or literal text to a pane.",
		flags: map[string]flagKind{
			"-t": flagString,
			"-l": flagBool,
			"-X": flagBool, // copy-mode command
			"-M": flagBool, // mouse passthrough (no-op in myT-x)
			"-W": flagBool, // typewriter mode for interactive TUIs
			"-N": flagBool, // CRLF mode: \r → \r\n for ConPTY Enter compatibility
		},
	},
	"select-pane": {
		description: "Focus a pane or move focus with -U/-D/-L/-R.",
		flags: map[string]flagKind{
			"-t": flagString,
			"-T": flagString,
			"-P": flagString,
			"-U": flagBool,
			"-D": flagBool,
			"-L": flagBool,
			"-R": flagBool,
		},
	},
	"list-sessions": {
		description: "List sessions. Use -F to format output and -f to filter.",
		flags: map[string]flagKind{
			"-F": flagString,
			"-f": flagString, // filter expression
		},
	},
	"kill-session": {
		description: "Close the target session.",
		flags: map[string]flagKind{
			"-t": flagString,
		},
	},
	"list-panes": {
		description: "List panes. Use -t target, -a all sessions, -F format, -f filter.",
		flags: map[string]flagKind{
			"-t": flagString,
			"-s": flagBool,
			"-a": flagBool, // all sessions
			"-F": flagString,
			"-f": flagString, // filter expression
		},
	},
	"display-message": {
		description: "Print a tmux format string with -p.",
		flags: map[string]flagKind{
			"-p": flagBool,
			"-t": flagString,
		},
	},
	"attach-session": {
		description: "Attach or switch to the target session.",
		flags: map[string]flagKind{
			"-t": flagString,
		},
	},
	"kill-pane": {
		description: "Close the target pane.",
		flags: map[string]flagKind{
			"-t": flagString,
		},
	},
	"rename-session": {
		description: "Rename the target session. Pass the new name as an argument.",
		flags: map[string]flagKind{
			"-t": flagString,
		},
	},
	"resize-pane": {
		description: "Resize or zoom a pane. Use -x/-y size or -U/-D/-L/-R direction.",
		// Note: -t is optional for resize-pane (defaults to current pane).
		flags: map[string]flagKind{
			"-t": flagString,
			"-x": flagString,
			"-y": flagString,
			"-U": flagBool, // resize up
			"-D": flagBool, // resize down
			"-L": flagBool, // resize left
			"-R": flagBool, // resize right
			"-Z": flagBool, // toggle zoom
		},
	},
	"select-layout": {
		description: "Select a predefined layout. Accepted for tmux compatibility as a no-op.",
		flags: map[string]flagKind{
			"-t": flagString,
			"-E": flagBool,
			"-n": flagBool,
			"-p": flagBool,
			"-o": flagBool,
		},
	},
	"show-environment": {
		description: "Show environment variables for a session or globally with -g.",
		flags: map[string]flagKind{
			"-t": flagString,
			"-g": flagBool,
		},
	},
	"set-environment": {
		description: "Set or unset environment variables. Use -u to unset and -g for global scope.",
		flags: map[string]flagKind{
			"-t": flagString,
			"-u": flagBool,
			"-g": flagBool,
		},
	},
	"set-option": {
		description: "Set a tmux option. Accepted for tmux compatibility as a no-op.",
		flags: map[string]flagKind{
			"-p": flagBool,
			"-w": flagBool,
			"-s": flagBool,
			"-g": flagBool,
			"-u": flagBool,
			"-o": flagBool,
			"-q": flagBool,
			"-a": flagBool,
			"-F": flagBool,
			"-t": flagString,
		},
	},
	"list-windows": {
		description: "List windows. Use -t target, -a all sessions, -F format, -f filter.",
		flags: map[string]flagKind{
			"-t": flagString,
			"-a": flagBool,
			"-F": flagString,
			"-f": flagString, // filter expression
		},
	},
	"rename-window": {
		description: "Rename the target window. Pass the new name as an argument.",
		flags: map[string]flagKind{
			"-t": flagString,
		},
	},
	// new-window: myT-x セマンティクス変更
	// tmux標準: 既存セッション内に新しいウィンドウを追加する。
	// myT-x:    子セッション（child session）を作成する。-n フラグで指定された名前が
	//           子セッション名として使用されるため、-n は必須である。
	"new-window": {
		description: "Create a child session from a session. Requires -t parent and -n child name.",
		flags: map[string]flagKind{
			"-d": flagBool,
			"-P": flagBool,
			"-F": flagString,
			"-n": flagString,
			"-t": flagString,
			"-c": flagString,
			"-e": flagEnv,
		},
	},
	"kill-window": {
		description: "Close the target window.",
		flags: map[string]flagKind{
			"-t": flagString,
		},
	},
	"select-window": {
		description: "Focus the target window.",
		flags: map[string]flagKind{
			"-t": flagString,
		},
	},
	"copy-mode": {
		description: "Enter or control copy mode for a pane.",
		flags: map[string]flagKind{
			"-t": flagString,
			"-q": flagBool, // quit copy mode
			"-u": flagBool, // page up
			"-e": flagBool, // erase on scroll
		},
	},
	"list-buffers": {
		description: "List paste buffers. Use -F to format output.",
		flags: map[string]flagKind{
			"-F": flagString, // output format
		},
	},
	"set-buffer": {
		description: "Create or update a paste buffer. Use -b name, -a append, -n rename.",
		flags: map[string]flagKind{
			"-a": flagBool,   // append to buffer
			"-b": flagString, // buffer name
			"-n": flagString, // rename buffer
		},
	},
	"paste-buffer": {
		description: "Paste a buffer into a pane. Use -b name and -t target pane.",
		flags: map[string]flagKind{
			"-d": flagBool,   // delete after paste
			"-b": flagString, // buffer name
			"-t": flagString, // target pane
			"-p": flagBool,   // bracket paste mode
			"-r": flagBool,   // replace newlines with CR
			"-s": flagString, // separator
		},
	},
	"load-buffer": {
		description: "Load file contents into a paste buffer.",
		flags: map[string]flagKind{
			"-b": flagString,
			"-w": flagBool,
			"-t": flagString,
		},
	},
	"save-buffer": {
		description: "Save a paste buffer to a file.",
		flags: map[string]flagKind{
			"-a": flagBool,
			"-b": flagString,
		},
	},
	"capture-pane": {
		description: "Capture pane output. Use -p to print and -S/-E to choose line range.",
		flags: map[string]flagKind{
			"-a": flagBool,
			"-b": flagString,
			"-C": flagBool,
			"-e": flagBool,
			"-E": flagString,
			"-J": flagBool,
			"-M": flagBool,
			"-N": flagBool,
			"-p": flagBool,
			"-P": flagBool,
			"-q": flagBool,
			"-S": flagString,
			"-T": flagBool,
			"-t": flagString,
		},
	},
	"run-shell": {
		description: "Run a shell command. Use -C to run tmux commands and -b for background.",
		flags: map[string]flagKind{
			"-b": flagBool,   // background (no wait)
			"-t": flagString, // target pane (for format context)
			"-C": flagBool,   // run as tmux commands
			"-c": flagString, // working directory
		},
	},
	"if-shell": {
		description: "Run commands conditionally from a shell or format test.",
		flags: map[string]flagKind{
			"-b": flagBool,   // background
			"-F": flagBool,   // format condition (not shell command)
			"-t": flagString, // target pane (for format context)
		},
	},
}

var commandOrder = []string{
	"new-session",
	"has-session",
	"split-window",
	"send-keys",
	"select-pane",
	"list-sessions",
	"kill-session",
	"list-panes",
	"display-message",
	"attach-session",
	"kill-pane",
	"rename-session",
	"resize-pane",
	"select-layout",
	"show-environment",
	"set-environment",
	"set-option",
	"list-windows",
	"rename-window",
	"new-window",
	"kill-window",
	"select-window",
	"copy-mode",
	"list-buffers",
	"set-buffer",
	"paste-buffer",
	"load-buffer",
	"save-buffer",
	"capture-pane",
	"run-shell",
	"if-shell",
}

func validateCommandSpecConsistency() error {
	seen := make(map[string]struct{}, len(commandOrder))
	for _, commandName := range commandOrder {
		if _, exists := seen[commandName]; exists {
			return fmt.Errorf("commandOrder contains duplicate command: %s", commandName)
		}
		seen[commandName] = struct{}{}
		if _, ok := commandSpecs[commandName]; !ok {
			return fmt.Errorf("commandOrder includes command missing from commandSpecs: %s", commandName)
		}
	}
	for commandName := range commandSpecs {
		if _, ok := seen[commandName]; !ok {
			return fmt.Errorf("commandSpecs includes command missing from commandOrder: %s", commandName)
		}
	}
	return nil
}

func init() {
	if err := validateCommandSpecConsistency(); err != nil {
		panic(err)
	}
}
