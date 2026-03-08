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
	flags map[string]flagKind
}

var commandSpecs = map[string]commandSpec{
	"new-session": {
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
		flags: map[string]flagKind{
			"-t": flagString,
		},
	},
	"split-window": {
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
		flags: map[string]flagKind{
			"-t": flagString,
			"-T": flagString,
			"-U": flagBool,
			"-D": flagBool,
			"-L": flagBool,
			"-R": flagBool,
		},
	},
	"list-sessions": {
		flags: map[string]flagKind{
			"-F": flagString,
			"-f": flagString, // filter expression
		},
	},
	"kill-session": {
		flags: map[string]flagKind{
			"-t": flagString,
		},
	},
	"list-panes": {
		flags: map[string]flagKind{
			"-t": flagString,
			"-s": flagBool,
			"-a": flagBool, // all sessions
			"-F": flagString,
			"-f": flagString, // filter expression
		},
	},
	"display-message": {
		flags: map[string]flagKind{
			"-p": flagBool,
			"-t": flagString,
		},
	},
	"attach-session": {
		flags: map[string]flagKind{
			"-t": flagString,
		},
	},
	"kill-pane": {
		flags: map[string]flagKind{
			"-t": flagString,
		},
	},
	"rename-session": {
		flags: map[string]flagKind{
			"-t": flagString,
		},
	},
	"resize-pane": {
		// Note: -t is optional for resize-pane (defaults to current pane).
		flags: map[string]flagKind{
			"-t": flagString,
			"-x": flagInt,
			"-y": flagInt,
			"-U": flagBool, // resize up
			"-D": flagBool, // resize down
			"-L": flagBool, // resize left
			"-R": flagBool, // resize right
			"-Z": flagBool, // toggle zoom
		},
	},
	"show-environment": {
		flags: map[string]flagKind{
			"-t": flagString,
			"-g": flagBool,
		},
	},
	"set-environment": {
		flags: map[string]flagKind{
			"-t": flagString,
			"-u": flagBool,
			"-g": flagBool,
		},
	},
	"list-windows": {
		flags: map[string]flagKind{
			"-t": flagString,
			"-a": flagBool,
			"-F": flagString,
			"-f": flagString, // filter expression
		},
	},
	"rename-window": {
		flags: map[string]flagKind{
			"-t": flagString,
		},
	},
	// new-window: myT-x セマンティクス変更
	// tmux標準: 既存セッション内に新しいウィンドウを追加する。
	// myT-x:    子セッション（child session）を作成する。-n フラグで指定された名前が
	//           子セッション名として使用されるため、-n は必須である。
	"new-window": {
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
		flags: map[string]flagKind{
			"-t": flagString,
		},
	},
	"select-window": {
		flags: map[string]flagKind{
			"-t": flagString,
		},
	},
	"copy-mode": {
		flags: map[string]flagKind{
			"-t": flagString,
			"-q": flagBool, // quit copy mode
			"-u": flagBool, // page up
			"-e": flagBool, // erase on scroll
		},
	},
	"list-buffers": {
		flags: map[string]flagKind{
			"-F": flagString, // output format
		},
	},
	"set-buffer": {
		flags: map[string]flagKind{
			"-a": flagBool,   // append to buffer
			"-b": flagString, // buffer name
			"-n": flagString, // rename buffer
		},
	},
	"paste-buffer": {
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
		flags: map[string]flagKind{
			"-b": flagString,
			"-w": flagBool,
			"-t": flagString,
		},
	},
	"save-buffer": {
		flags: map[string]flagKind{
			"-a": flagBool,
			"-b": flagString,
		},
	},
	"capture-pane": {
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
		flags: map[string]flagKind{
			"-b": flagBool,   // background (no wait)
			"-t": flagString, // target pane (for format context)
			"-C": flagBool,   // run as tmux commands
			"-c": flagString, // working directory
		},
	},
	"if-shell": {
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
	"show-environment",
	"set-environment",
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
