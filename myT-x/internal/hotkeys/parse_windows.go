//go:build windows

package hotkeys

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	modAlt     Modifier = 0x0001
	modControl Modifier = 0x0002
	modShift   Modifier = 0x0004
	modWin     Modifier = 0x0008
)

const (
	vkSpace  VKey = 0x20
	vkTab    VKey = 0x09
	vkReturn VKey = 0x0D
	vkEscape VKey = 0x1B
	vkDelete VKey = 0x2E
	vkLeft   VKey = 0x25
	vkUp     VKey = 0x26
	vkRight  VKey = 0x27
	vkDown   VKey = 0x28
	vkOem3   VKey = 0xC0
	vkF1     VKey = 0x70
	vkF2     VKey = 0x71
	vkF3     VKey = 0x72
	vkF4     VKey = 0x73
	vkF5     VKey = 0x74
	vkF6     VKey = 0x75
	vkF7     VKey = 0x76
	vkF8     VKey = 0x77
	vkF9     VKey = 0x78
	vkF10    VKey = 0x79
	vkF11    VKey = 0x7A
	vkF12    VKey = 0x7B
	vkF13    VKey = 0x7C
	vkF14    VKey = 0x7D
	vkF15    VKey = 0x7E
	vkF16    VKey = 0x7F
	vkF17    VKey = 0x80
	vkF18    VKey = 0x81
	vkF19    VKey = 0x82
	vkF20    VKey = 0x83
)

var windowsModifierByName = map[string]Modifier{
	"CTRL":    modControl,
	"CONTROL": modControl,
	"SHIFT":   modShift,
	"ALT":     modAlt,
	"WIN":     modWin,
	"SUPER":   modWin,
}

var windowsKeyByName = map[string]VKey{
	"SPACE":  vkSpace,
	"TAB":    vkTab,
	"ENTER":  vkReturn,
	"RETURN": vkReturn,
	"ESC":    vkEscape,
	"ESCAPE": vkEscape,
	"DELETE": vkDelete,
	"LEFT":   vkLeft,
	"RIGHT":  vkRight,
	"UP":     vkUp,
	"DOWN":   vkDown,
}

var windowsFunctionKeys = map[string]VKey{
	"F1":  vkF1,
	"F2":  vkF2,
	"F3":  vkF3,
	"F4":  vkF4,
	"F5":  vkF5,
	"F6":  vkF6,
	"F7":  vkF7,
	"F8":  vkF8,
	"F9":  vkF9,
	"F10": vkF10,
	"F11": vkF11,
	"F12": vkF12,
	"F13": vkF13,
	"F14": vkF14,
	"F15": vkF15,
	"F16": vkF16,
	"F17": vkF17,
	"F18": vkF18,
	"F19": vkF19,
	"F20": vkF20,
}

// ParseBinding parses a binding like "Ctrl+Shift+F12".
func ParseBinding(spec string) (Binding, error) {
	raw := strings.TrimSpace(spec)
	if raw == "" {
		return Binding{}, fmt.Errorf("hotkey spec is empty")
	}

	parts := strings.Split(raw, "+")
	if len(parts) < 2 {
		return Binding{}, fmt.Errorf("hotkey must include modifiers and key: %s", raw)
	}

	var modifiers Modifier
	seen := map[Modifier]struct{}{}
	var normalizedMods []string

	for _, token := range parts[:len(parts)-1] {
		name := strings.ToUpper(strings.TrimSpace(token))
		mod, ok := windowsModifierByName[name]
		if !ok {
			return Binding{}, fmt.Errorf("unknown modifier %q in hotkey %q", token, raw)
		}
		if _, exists := seen[mod]; exists {
			continue
		}
		seen[mod] = struct{}{}
		modifiers |= mod
		normalizedMods = append(normalizedMods, normalizeModifierName(mod))
	}

	keyToken := strings.TrimSpace(parts[len(parts)-1])
	key, normalizedKey, err := parseWindowsKey(keyToken)
	if err != nil {
		return Binding{}, err
	}

	if modifiers == 0 {
		return Binding{}, fmt.Errorf("at least one modifier is required: %q", raw)
	}

	normalized := strings.Join(append(normalizedMods, normalizedKey), "+")
	return Binding{
		modifiers:  modifiers,
		key:        key,
		normalized: normalized,
	}, nil
}

func parseWindowsKey(raw string) (VKey, string, error) {
	token := strings.ToUpper(strings.TrimSpace(raw))
	if token == "" {
		return 0, "", fmt.Errorf("missing hotkey key token")
	}

	if key, ok := windowsFunctionKeys[token]; ok {
		return key, token, nil
	}
	if key, ok := windowsKeyByName[token]; ok {
		return key, token, nil
	}

	if len(token) == 1 {
		ch := token[0]
		if ch >= 'A' && ch <= 'Z' {
			return VKey(ch), token, nil
		}
		if ch >= '0' && ch <= '9' {
			return VKey(ch), token, nil
		}
		if ch == '`' {
			return vkOem3, "`", nil
		}
	}

	switch token {
	case "BACKQUOTE", "GRAVE":
		return vkOem3, "`", nil
	}

	if strings.HasPrefix(token, "0X") {
		value, err := strconv.ParseUint(token[2:], 16, 16)
		if err != nil {
			return 0, "", fmt.Errorf("invalid hex key %q", raw)
		}
		if value == 0 {
			return 0, "", fmt.Errorf("key code 0x0000 is not a valid virtual key")
		}
		return VKey(value), token, nil
	}

	return 0, "", fmt.Errorf("unknown key %q in hotkey spec", raw)
}

func normalizeModifierName(mod Modifier) string {
	switch mod {
	case modControl:
		return "Ctrl"
	case modShift:
		return "Shift"
	case modAlt:
		return "Alt"
	case modWin:
		return "Win"
	default:
		return "Mod"
	}
}
