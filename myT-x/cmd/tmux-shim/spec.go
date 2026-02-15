package main

type flagKind int

const (
	flagBool flagKind = iota
	flagString
	flagInt
	flagEnv
)

type commandSpec struct {
	name  string
	flags map[string]flagKind
}

var commandSpecs = map[string]commandSpec{
	"new-session": {
		name: "new-session",
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
		name: "has-session",
		flags: map[string]flagKind{
			"-t": flagString,
		},
	},
	"split-window": {
		name: "split-window",
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
		name: "send-keys",
		flags: map[string]flagKind{
			"-t": flagString,
		},
	},
	"select-pane": {
		name: "select-pane",
		flags: map[string]flagKind{
			"-t": flagString,
			"-U": flagBool,
			"-D": flagBool,
			"-L": flagBool,
			"-R": flagBool,
		},
	},
	"list-sessions": {
		name: "list-sessions",
		flags: map[string]flagKind{
			"-F": flagString,
		},
	},
	"kill-session": {
		name: "kill-session",
		flags: map[string]flagKind{
			"-t": flagString,
		},
	},
	"list-panes": {
		name: "list-panes",
		flags: map[string]flagKind{
			"-t": flagString,
			"-s": flagBool,
			"-F": flagString,
		},
	},
	"display-message": {
		name: "display-message",
		flags: map[string]flagKind{
			"-p": flagBool,
			"-t": flagString,
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
}
