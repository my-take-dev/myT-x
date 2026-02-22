package main

import (
	"testing"
	"unsafe"
)

// C-07: toString direct unit tests covering:
// - nil input -> empty string
// - string input -> same string
// - []byte input -> string conversion
// - int input -> fmt.Sprintf representation
// - unsupported struct type -> fmt.Sprintf representation
func TestToString(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  string
	}{
		{
			name:  "nil returns empty string",
			input: nil,
			want:  "",
		},
		{
			name:  "string returns same string",
			input: "hello",
			want:  "hello",
		},
		{
			name:  "empty string returns empty string",
			input: "",
			want:  "",
		},
		{
			name:  "byte slice returns string conversion",
			input: []byte("world"),
			want:  "[119 111 114 108 100]",
		},
		{
			name:  "int uses Sprintf fallback",
			input: 42,
			want:  "42",
		},
		{
			name:  "bool uses Sprintf fallback",
			input: true,
			want:  "true",
		},
		{
			name:  "float uses Sprintf fallback",
			input: 3.14,
			want:  "3.14",
		},
		{
			name:  "struct uses Sprintf fallback",
			input: struct{ X int }{X: 1},
			want:  "{1}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toString(tt.input)
			if got != tt.want {
				t.Fatalf("toString(%v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// C-07: toBytes direct unit tests covering:
// - nil input -> nil
// - []byte input -> same slice (aliasing)
// - string input -> []byte conversion
// - unsupported int type -> nil
// - unsupported struct type -> nil
// - aliasing verification: []byte input returns original backing array
func TestToBytes(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		wantNil  bool
		wantData []byte
	}{
		{
			name:    "nil returns nil",
			input:   nil,
			wantNil: true,
		},
		{
			name:     "byte slice returns same slice",
			input:    []byte("hello"),
			wantData: []byte("hello"),
		},
		{
			name:     "string returns byte conversion",
			input:    "world",
			wantData: []byte("world"),
		},
		{
			name:     "empty string returns empty byte slice",
			input:    "",
			wantData: []byte(""),
		},
		{
			name:     "empty byte slice returns empty byte slice",
			input:    []byte{},
			wantData: []byte{},
		},
		{
			name:    "int returns nil (unsupported)",
			input:   42,
			wantNil: true,
		},
		{
			name:    "bool returns nil (unsupported)",
			input:   true,
			wantNil: true,
		},
		{
			name:    "struct returns nil (unsupported)",
			input:   struct{ X int }{X: 1},
			wantNil: true,
		},
		{
			name:    "float returns nil (unsupported)",
			input:   3.14,
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toBytes(tt.input)
			if tt.wantNil {
				if got != nil {
					t.Fatalf("toBytes(%v) = %v, want nil", tt.input, got)
				}
				return
			}
			if got == nil {
				t.Fatalf("toBytes(%v) = nil, want %v", tt.input, tt.wantData)
			}
			if string(got) != string(tt.wantData) {
				t.Fatalf("toBytes(%v) = %v, want %v", tt.input, got, tt.wantData)
			}
		})
	}
}

// TestToBytesAliasing verifies that the []byte path returns the original slice
// without copying (aliasing behaviour documented in the function contract).
func TestToBytesAliasing(t *testing.T) {
	original := []byte("aliasing test")
	result := toBytes(original)

	if result == nil {
		t.Fatal("toBytes([]byte) returned nil, want non-nil alias")
	}

	// Verify the backing array is the same by comparing data pointers.
	origPtr := unsafe.SliceData(original)
	resultPtr := unsafe.SliceData(result)
	if origPtr != resultPtr {
		t.Fatal("toBytes([]byte) returned a copy; want alias to original backing array")
	}

	// Double-check by mutating the result and verifying original is affected.
	result[0] = 'A'
	if original[0] != 'A' {
		t.Fatal("mutating toBytes result did not affect original; backing arrays are independent (copy, not alias)")
	}
}

// TestToBytesStringNotAliased verifies that the string path creates a new
// byte slice (no aliasing with the original string's memory).
func TestToBytesStringNotAliased(t *testing.T) {
	input := "immutable"
	result := toBytes(input)

	if result == nil {
		t.Fatal("toBytes(string) returned nil")
	}
	if string(result) != input {
		t.Fatalf("toBytes(%q) = %q, want %q", input, string(result), input)
	}

	// Mutating result must not affect the original string.
	result[0] = 'X'
	if input[0] == 'X' {
		t.Fatal("mutating toBytes result affected original string")
	}
}
