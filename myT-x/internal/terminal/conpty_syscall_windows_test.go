//go:build windows

package terminal

import (
	"runtime"
	"testing"
	"unsafe"
)

func TestCreateEnvBlock(t *testing.T) {
	tests := []struct {
		name string
		env  []string
		want bool // true = non-nil result expected
	}{
		{"nil input", nil, false},
		{"empty slice", []string{}, false},
		{"single empty entry", []string{""}, false},
		{"all empty strings", []string{"", "", ""}, false},
		{"single valid entry", []string{"FOO=bar"}, true},
		{"mixed empty and valid", []string{"", "FOO=bar", ""}, true},
		{"multiple valid entries", []string{"FOO=bar", "BAZ=qux"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := createEnvBlock(tt.env)
			if (got != nil) != tt.want {
				t.Fatalf("createEnvBlock() returned nil=%v, want nil=%v", got == nil, !tt.want)
			}
		})
	}
}

func TestCreateEnvBlockContent(t *testing.T) {
	env := []string{"FOO=bar", "BAZ=qux"}
	ptr := createEnvBlock(env)
	if ptr == nil {
		t.Fatal("createEnvBlock() returned nil")
	}

	// Read the UTF-16 block from the returned pointer.
	// Environment block format: "FOO=bar\0BAZ=qux\0\0" (double null terminated).
	block := readUTF16Block(ptr)

	// Split by null terminators — last entry is empty (from double null).
	var entries []string
	current := ""
	for _, ch := range block {
		if ch == 0 {
			if current == "" {
				break // double null = end of block
			}
			entries = append(entries, current)
			current = ""
		} else {
			current += string(rune(ch))
		}
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d: %v", len(entries), entries)
	}
	if entries[0] != "FOO=bar" {
		t.Fatalf("entry[0] = %q, want %q", entries[0], "FOO=bar")
	}
	if entries[1] != "BAZ=qux" {
		t.Fatalf("entry[1] = %q, want %q", entries[1], "BAZ=qux")
	}

	// Keep ptr alive until assertions complete to avoid premature GC in unsafe test helper paths.
	runtime.KeepAlive(ptr)
}

func TestCreateEnvBlockContentUnicode(t *testing.T) {
	env := []string{"LANG=日本語", "FOO=bar"}
	ptr := createEnvBlock(env)
	if ptr == nil {
		t.Fatal("createEnvBlock() returned nil")
	}

	block := readUTF16Block(ptr)

	var entries []string
	current := ""
	for _, ch := range block {
		if ch == 0 {
			if current == "" {
				break
			}
			entries = append(entries, current)
			current = ""
			continue
		}
		current += string(rune(ch))
	}

	if len(entries) != len(env) {
		t.Fatalf("expected %d entries, got %d: %v", len(env), len(entries), entries)
	}
	if entries[0] != env[0] {
		t.Fatalf("entry[0] = %q, want %q", entries[0], env[0])
	}
	if entries[1] != env[1] {
		t.Fatalf("entry[1] = %q, want %q", entries[1], env[1])
	}

	runtime.KeepAlive(ptr)
}

func TestCoordPackUsesUnsigned16BitLayout(t *testing.T) {
	tests := []struct {
		name  string
		coord _COORD
		want  uint32
	}{
		{
			name:  "zero values",
			coord: _COORD{X: 0, Y: 0},
			want:  0x00000000,
		},
		{
			name:  "positive dimensions",
			coord: _COORD{X: 80, Y: 40},
			want:  0x00280050,
		},
		{
			name:  "negative values use unsigned 16-bit layout",
			coord: _COORD{X: -1, Y: -1},
			want:  0xffffffff,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := uint32(tt.coord.Pack())
			if got != tt.want {
				t.Fatalf("Pack() = 0x%08X, want 0x%08X", got, tt.want)
			}
		})
	}
}

// readUTF16Block reads a null-terminated UTF-16 environment block from a pointer.
// Reads until finding a double null terminator (empty string entry).
func readUTF16Block(ptr *uint16) []uint16 {
	if ptr == nil {
		return nil
	}
	var result []uint16
	p := unsafe.Pointer(ptr)
	nullCount := 0
	const maxUTF16Units = 1 << 20
	const utf16UnitSize = uintptr(unsafe.Sizeof(uint16(0)))
	for i := range maxUTF16Units {
		ch := *(*uint16)(unsafe.Pointer(uintptr(p) + uintptr(i)*utf16UnitSize))
		result = append(result, ch)
		if ch == 0 {
			nullCount++
			if nullCount >= 2 {
				break
			}
		} else {
			nullCount = 0
		}
	}
	return result
}
