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
	for i := 0; i < maxUTF16Units; i++ {
		ch := *(*uint16)(unsafe.Pointer(uintptr(p) + uintptr(i)*2))
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
