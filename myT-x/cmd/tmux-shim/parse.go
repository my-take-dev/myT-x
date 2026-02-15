package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"myT-x/internal/ipc"
)

func parseCommand(args []string) (ipc.TmuxRequest, error) {
	name := strings.TrimSpace(args[0])
	spec, ok := commandSpecs[name]
	if !ok {
		return ipc.TmuxRequest{}, fmt.Errorf("unknown command: %s", name)
	}

	req := ipc.TmuxRequest{
		Command: name,
		Flags:   map[string]any{},
		Env:     map[string]string{},
	}

	i := 1
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			req.Args = append(req.Args, args[i+1:]...)
			return req, validateRequired(name, req)
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			req.Args = append(req.Args, args[i:]...)
			return req, validateRequired(name, req)
		}

		kind, known := spec.flags[arg]
		if !known {
			// Try expanding combined bool flags: -dPh → -d, -P, -h
			if expanded, ok := expandCombinedFlags(spec, arg); ok {
				for _, flag := range expanded {
					req.Flags[flag] = true
				}
				i++
				continue
			}
			return ipc.TmuxRequest{}, fmt.Errorf("unsupported flag %s for %s", arg, name)
		}

		switch kind {
		case flagBool:
			req.Flags[arg] = true
			i++
		case flagString:
			if i+1 >= len(args) {
				return ipc.TmuxRequest{}, fmt.Errorf("flag %s requires a value", arg)
			}
			req.Flags[arg] = args[i+1]
			i += 2
		case flagInt:
			if i+1 >= len(args) {
				return ipc.TmuxRequest{}, fmt.Errorf("flag %s requires a value", arg)
			}
			value, err := strconv.Atoi(args[i+1])
			if err != nil {
				return ipc.TmuxRequest{}, fmt.Errorf("flag %s expects integer, got %q", arg, args[i+1])
			}
			req.Flags[arg] = value
			i += 2
		case flagEnv:
			if i+1 >= len(args) {
				return ipc.TmuxRequest{}, fmt.Errorf("flag %s requires KEY=VALUE", arg)
			}
			key, value, ok := strings.Cut(args[i+1], "=")
			if !ok || strings.TrimSpace(key) == "" {
				return ipc.TmuxRequest{}, fmt.Errorf("invalid env: %s", args[i+1])
			}
			req.Env[key] = value
			i += 2
		default:
			return ipc.TmuxRequest{}, errors.New("unsupported flag parser")
		}
	}

	return req, validateRequired(name, req)
}

func validateRequired(command string, req ipc.TmuxRequest) error {
	switch command {
	case "has-session", "kill-session":
		if strings.TrimSpace(asString(req.Flags["-t"])) == "" {
			return fmt.Errorf("%s requires -t", command)
		}
	case "display-message":
		if !asBool(req.Flags["-p"]) {
			return fmt.Errorf("display-message requires -p")
		}
	}
	return nil
}

func asString(value any) string {
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}

func asBool(value any) bool {
	if b, ok := value.(bool); ok {
		return b
	}
	return false
}

// expandCombinedFlags expands combined bool flags like "-dPh" into ["-d", "-P", "-h"].
// Returns (flags, true) if all characters are known bool flags, or (nil, false) otherwise.
func expandCombinedFlags(spec commandSpec, arg string) ([]string, bool) {
	if len(arg) < 3 || arg[0] != '-' {
		return nil, false
	}
	chars := arg[1:]
	flags := make([]string, 0, len(chars))
	for _, ch := range chars {
		flag := "-" + string(ch)
		kind, known := spec.flags[flag]
		if !known || kind != flagBool {
			return nil, false
		}
		flags = append(flags, flag)
	}
	return flags, true
}
