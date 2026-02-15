//go:build windows

package install

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"unicode/utf16"

	"golang.org/x/sys/windows/registry"
)

func TestContainsPathEntry(t *testing.T) {
	pathValue := `C:\Windows\System32;C:\Users\tester\AppData\Local\myT-x\bin`
	tests := []struct {
		name  string
		entry string
		want  bool
	}{
		{"exact match", `C:\Users\tester\AppData\Local\myT-x\bin`, true},
		{"case-insensitive match", `c:\users\tester\appdata\local\myT-x\bin`, true},
		{"missing entry", `C:\Tools\bin`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := containsPathEntry(pathValue, tt.entry); got != tt.want {
				t.Fatalf("containsPathEntry(%q) = %v, want %v", tt.entry, got, tt.want)
			}
		})
	}
}

func TestEnsureProcessPathContains(t *testing.T) {
	original := os.Getenv("PATH")
	t.Cleanup(func() {
		_ = os.Setenv("PATH", original)
	})

	base := `C:\Windows\System32`
	if err := os.Setenv("PATH", base); err != nil {
		t.Fatalf("Setenv(PATH) error = %v", err)
	}

	target := `C:\Users\tester\AppData\Local\myT-x\bin`
	if updated := EnsureProcessPathContains(target); !updated {
		t.Fatal("EnsureProcessPathContains() should update PATH on first call")
	}
	if updated := EnsureProcessPathContains(target); updated {
		t.Fatal("EnsureProcessPathContains() should not update PATH when already present")
	}
}

func TestResolveInstallDir(t *testing.T) {
	t.Run("returns error when LOCALAPPDATA is not set", func(t *testing.T) {
		original := os.Getenv("LOCALAPPDATA")
		t.Cleanup(func() {
			_ = os.Setenv("LOCALAPPDATA", original)
		})
		if err := os.Setenv("LOCALAPPDATA", ""); err != nil {
			t.Fatalf("Setenv(LOCALAPPDATA) error = %v", err)
		}

		if _, err := ResolveInstallDir(); err == nil {
			t.Fatal("ResolveInstallDir() expected error when LOCALAPPDATA is empty")
		}
	})

	t.Run("returns expected directory", func(t *testing.T) {
		original := os.Getenv("LOCALAPPDATA")
		t.Cleanup(func() {
			_ = os.Setenv("LOCALAPPDATA", original)
		})
		localAppData := t.TempDir()
		if err := os.Setenv("LOCALAPPDATA", localAppData); err != nil {
			t.Fatalf("Setenv(LOCALAPPDATA) error = %v", err)
		}

		got, err := ResolveInstallDir()
		if err != nil {
			t.Fatalf("ResolveInstallDir() error = %v", err)
		}
		want := filepath.Join(localAppData, "myT-x", "bin")
		if got != want {
			t.Fatalf("ResolveInstallDir() = %q, want %q", got, want)
		}
	})
}

func TestReadUserPathFromRegistry(t *testing.T) {
	// This test reads the real HKCU\Environment\Path value.
	// It verifies that readUserPathFromRegistry does not return an error
	// on the current machine (the Path value may or may not exist).
	value, err := readUserPathFromRegistry()
	if err != nil {
		t.Fatalf("readUserPathFromRegistry() error = %v", err)
	}
	// On most Windows machines, user PATH is non-empty.
	// We only verify no error; the value itself is environment-dependent.
	t.Logf("readUserPathFromRegistry() = %q", value)
}

func TestInstallShimIfChangedEmptySourceHash(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "shim.exe")
	hashFile := target + ".sha256"

	writeCalled := false
	err := installShimIfChanged(hashFile, "", target, func() error {
		writeCalled = true
		return os.WriteFile(target, []byte("binary"), 0o755)
	})
	if err != nil {
		t.Fatalf("installShimIfChanged() error = %v", err)
	}
	if !writeCalled {
		t.Fatal("writeFn should be called when sourceHash is empty")
	}
	// Hash file should NOT be written when sourceHash is empty.
	if _, err := os.Stat(hashFile); !os.IsNotExist(err) {
		t.Fatalf("hash file should not exist when sourceHash is empty, stat err = %v", err)
	}
}

func TestInstallShimIfChangedWriteFnError(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "shim.exe")
	hashFile := target + ".sha256"

	writeErr := fmt.Errorf("disk full")
	err := installShimIfChanged(hashFile, "abc123", target, func() error {
		return writeErr
	})
	if err == nil {
		t.Fatal("installShimIfChanged() expected error from writeFn")
	}
	if err != writeErr {
		t.Fatalf("installShimIfChanged() error = %v, want %v", err, writeErr)
	}
	// Hash file must NOT be written when writeFn fails.
	if _, statErr := os.Stat(hashFile); !os.IsNotExist(statErr) {
		t.Fatalf("hash file should not exist when writeFn fails, stat err = %v", statErr)
	}
}

func TestInstallShimIfChangedSkipsWhenHashMatches(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "shim.exe")
	hashFile := target + ".sha256"

	// Pre-create hash file with matching hash.
	sourceHash := "deadbeef1234567890"
	if err := os.WriteFile(hashFile, []byte(sourceHash), 0o644); err != nil {
		t.Fatalf("setup: write hash file: %v", err)
	}

	writeCalled := false
	err := installShimIfChanged(hashFile, sourceHash, target, func() error {
		writeCalled = true
		return os.WriteFile(target, []byte("binary"), 0o755)
	})
	if err != nil {
		t.Fatalf("installShimIfChanged() error = %v", err)
	}
	if writeCalled {
		t.Fatal("writeFn should NOT be called when sourceHash matches hash file")
	}
}

func TestInstallShimIfChangedUpdatesHash(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "shim.exe")
	hashFile := target + ".sha256"

	if err := installShimIfChanged(hashFile, "newhash", target, func() error {
		return os.WriteFile(target, []byte("binary"), 0o755)
	}); err != nil {
		t.Fatalf("installShimIfChanged() error = %v", err)
	}

	stored, err := os.ReadFile(hashFile)
	if err != nil {
		t.Fatalf("read hash file: %v", err)
	}
	if string(stored) != "newhash" {
		t.Fatalf("hash file = %q, want %q", string(stored), "newhash")
	}
}

func TestSelectPathRegistryValueType(t *testing.T) {
	tests := []struct {
		name      string
		valueType uint32
		pathValue string
		want      uint32
	}{
		{
			name:      "preserve SZ",
			valueType: registry.SZ,
			pathValue: `C:\tools\bin`,
			want:      registry.SZ,
		},
		{
			name:      "preserve EXPAND_SZ",
			valueType: registry.EXPAND_SZ,
			pathValue: `%USERPROFILE%\go\bin`,
			want:      registry.EXPAND_SZ,
		},
		{
			name:      "new value without expansion markers defaults to SZ",
			valueType: registry.NONE,
			pathValue: `C:\Users\tester\AppData\Local\myT-x\bin`,
			want:      registry.SZ,
		},
		{
			name:      "new value with expansion markers defaults to EXPAND_SZ",
			valueType: registry.NONE,
			pathValue: `%USERPROFILE%\go\bin`,
			want:      registry.EXPAND_SZ,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := selectPathRegistryValueType(tt.valueType, tt.pathValue); got != tt.want {
				t.Fatalf("selectPathRegistryValueType(%d, %q) = %d, want %d", tt.valueType, tt.pathValue, got, tt.want)
			}
		})
	}
}

func TestDecodeRegistryUTF16String(t *testing.T) {
	tests := []struct {
		name string
		raw  []byte
		want string
	}{
		{
			name: "empty",
			raw:  nil,
			want: "",
		},
		{
			name: "expand variable remains literal",
			raw:  encodeUTF16RegistryTestValue(`%USERPROFILE%\go\bin`),
			want: `%USERPROFILE%\go\bin`,
		},
		{
			name: "odd byte length is tolerated",
			raw:  append(encodeUTF16RegistryTestValue(`C:\tools\bin`), 0xAA),
			want: `C:\tools\bin`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := decodeRegistryUTF16String(tt.raw); got != tt.want {
				t.Fatalf("decodeRegistryUTF16String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func encodeUTF16RegistryTestValue(value string) []byte {
	encoded := utf16.Encode([]rune(value + "\x00"))
	buf := make([]byte, len(encoded)*2)
	for i, ch := range encoded {
		binary.LittleEndian.PutUint16(buf[i*2:], ch)
	}
	return buf
}
