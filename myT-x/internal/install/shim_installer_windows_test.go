//go:build windows

package install

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"

	"golang.org/x/sys/windows"
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
		{"empty entry is never a match", "", false},
		{"whitespace entry is never a match", "   ", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := containsPathEntry(pathValue, tt.entry); got != tt.want {
				t.Fatalf("containsPathEntry(%q) = %v, want %v", tt.entry, got, tt.want)
			}
		})
	}

	t.Run("empty entry does not match dot segment", func(t *testing.T) {
		if got := containsPathEntry(`C:\Windows\System32;.;C:\Tools`, ""); got {
			t.Fatal("containsPathEntry() should return false for empty entry")
		}
	})
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

	t.Run("returns false when target is empty", func(t *testing.T) {
		before := os.Getenv("PATH")
		if updated := EnsureProcessPathContains("   "); updated {
			t.Fatal("EnsureProcessPathContains() should return false for empty target")
		}
		if got := os.Getenv("PATH"); got != before {
			t.Fatalf("PATH changed after empty target: got %q, want %q", got, before)
		}
	})

	t.Run("returns false when PATH update fails", func(t *testing.T) {
		before := os.Getenv("PATH")
		invalidTarget := "C:\\invalid\x00entry"
		if updated := EnsureProcessPathContains(invalidTarget); updated {
			t.Fatal("EnsureProcessPathContains() should return false when os.Setenv fails")
		}
		if got := os.Getenv("PATH"); got != before {
			t.Fatalf("PATH changed after failed update: got %q, want %q", got, before)
		}
	})
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
	if os.Getenv("MYTX_RUN_REGISTRY_INTEGRATION_TEST") != "1" {
		t.Skip("set MYTX_RUN_REGISTRY_INTEGRATION_TEST=1 to run registry integration test")
	}

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
	if _, err := os.Stat(hashFile); !errors.Is(err, os.ErrNotExist) {
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
	if _, statErr := os.Stat(hashFile); !errors.Is(statErr, os.ErrNotExist) {
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

func TestWriteFileAtomically(t *testing.T) {
	target := filepath.Join(t.TempDir(), "tmux.exe")
	if err := os.WriteFile(target, []byte("old-binary"), 0o644); err != nil {
		t.Fatalf("setup write target error = %v", err)
	}

	if err := writeFileAtomically(target, []byte("new-binary"), 0o755); err != nil {
		t.Fatalf("writeFileAtomically() error = %v", err)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target error = %v", err)
	}
	if string(data) != "new-binary" {
		t.Fatalf("target content = %q, want %q", string(data), "new-binary")
	}
}

func TestMatchesHashFileTrimsExpectedHash(t *testing.T) {
	hashFile := filepath.Join(t.TempDir(), "tmux.exe.sha256")
	if err := os.WriteFile(hashFile, []byte("abc123\n"), 0o644); err != nil {
		t.Fatalf("setup write hash file error = %v", err)
	}
	if !matchesHashFile(hashFile, " abc123 \r\n") {
		t.Fatal("matchesHashFile() expected true with trimmed expected hash")
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
		{
			name:      "unknown type is preserved for caller-side validation",
			valueType: 9999,
			pathValue: `C:\tools\bin`,
			want:      9999,
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

func TestReadRegistryPathRawValueWithRetry(t *testing.T) {
	t.Run("returns error when raw size is negative", func(t *testing.T) {
		_, _, err := readRegistryPathRawValueWithRetry(
			-1,
			registry.SZ,
			func([]byte) (int, uint32, error) {
				t.Fatal("readValue should not be called for negative raw size")
				return 0, registry.NONE, nil
			},
			func() (int, uint32, error) {
				t.Fatal("readSizeAndType should not be called for negative raw size")
				return 0, registry.NONE, nil
			},
		)
		if err == nil {
			t.Fatal("readRegistryPathRawValueWithRetry() expected error for negative raw size")
		}
		if !strings.Contains(err.Error(), "invalid raw size") {
			t.Fatalf("error = %v, want invalid raw size", err)
		}
	})

	t.Run("returns error when raw size exceeds limit", func(t *testing.T) {
		_, _, err := readRegistryPathRawValueWithRetry(
			maxRegistryPathRawSize+1,
			registry.SZ,
			func([]byte) (int, uint32, error) {
				t.Fatal("readValue should not be called for oversized raw size")
				return 0, registry.NONE, nil
			},
			func() (int, uint32, error) {
				t.Fatal("readSizeAndType should not be called for oversized raw size")
				return 0, registry.NONE, nil
			},
		)
		if err == nil {
			t.Fatal("readRegistryPathRawValueWithRetry() expected error for oversized raw size")
		}
		if !strings.Contains(err.Error(), "exceeds limit") {
			t.Fatalf("error = %v, want exceeds limit", err)
		}
	})

	t.Run("returns error when read callback returns negative n after success", func(t *testing.T) {
		_, _, err := readRegistryPathRawValueWithRetry(
			4,
			registry.SZ,
			func(buffer []byte) (int, uint32, error) {
				// Simulate a successful read that reports negative bytes read.
				return -1, registry.SZ, nil
			},
			func() (int, uint32, error) {
				t.Fatal("readSizeAndType should not be called when read returns negative n")
				return 0, registry.NONE, nil
			},
		)
		if err == nil {
			t.Fatal("readRegistryPathRawValueWithRetry() expected error for negative read size")
		}
		if !strings.Contains(err.Error(), "invalid read size") {
			t.Fatalf("error = %v, want invalid read size", err)
		}
	})

	t.Run("returns empty payload when registry value is empty", func(t *testing.T) {
		raw, valueType, err := readRegistryPathRawValueWithRetry(
			0,
			registry.SZ,
			func(buffer []byte) (int, uint32, error) {
				if len(buffer) != 0 {
					t.Fatalf("buffer length = %d, want 0 for empty value", len(buffer))
				}
				return 0, registry.SZ, nil
			},
			func() (int, uint32, error) {
				t.Fatal("readSizeAndType should not be called for stable empty value")
				return 0, registry.NONE, nil
			},
		)
		if err != nil {
			t.Fatalf("readRegistryPathRawValueWithRetry() error = %v", err)
		}
		if valueType != registry.SZ {
			t.Fatalf("valueType = %d, want %d", valueType, registry.SZ)
		}
		if len(raw) != 0 {
			t.Fatalf("raw length = %d, want 0", len(raw))
		}
	})

	t.Run("handles size growth after initial zero-length query", func(t *testing.T) {
		callCount := 0
		raw, valueType, err := readRegistryPathRawValueWithRetry(
			0,
			registry.SZ,
			func(buffer []byte) (int, uint32, error) {
				callCount++
				if callCount == 1 {
					if len(buffer) != 0 {
						t.Fatalf("first call buffer length = %d, want 0 (zero-length initial query)", len(buffer))
					}
					return 4, registry.SZ, nil
				}
				copy(buffer, []byte{1, 2, 3, 4})
				return 4, registry.SZ, nil
			},
			func() (int, uint32, error) {
				t.Fatal("readSizeAndType should not be called in size-growth retry path")
				return 0, registry.NONE, nil
			},
		)
		if err != nil {
			t.Fatalf("readRegistryPathRawValueWithRetry() error = %v", err)
		}
		if valueType != registry.SZ {
			t.Fatalf("valueType = %d, want %d", valueType, registry.SZ)
		}
		if !bytes.Equal(raw, []byte{1, 2, 3, 4}) {
			t.Fatalf("raw value = %v, want [1 2 3 4]", raw)
		}
		if callCount != 2 {
			t.Fatalf("readValue call count = %d, want 2", callCount)
		}
	})

	t.Run("returns error on unsupported value type during size-growth retry", func(t *testing.T) {
		_, _, err := readRegistryPathRawValueWithRetry(
			0,
			registry.BINARY,
			func([]byte) (int, uint32, error) {
				return 4, registry.BINARY, nil
			},
			func() (int, uint32, error) {
				t.Fatal("readSizeAndType should not be called in size-growth retry path")
				return 0, registry.NONE, nil
			},
		)
		if err == nil {
			t.Fatal("readRegistryPathRawValueWithRetry() expected unsupported value type error")
		}
		if !strings.Contains(err.Error(), "unsupported value type") {
			t.Fatalf("error = %v, want unsupported value type", err)
		}
	})

	t.Run("retries on ERROR_MORE_DATA and refreshes value type", func(t *testing.T) {
		readCalls := 0
		refreshCalls := 0
		raw, valueType, err := readRegistryPathRawValueWithRetry(
			1,
			registry.SZ,
			func(buffer []byte) (int, uint32, error) {
				readCalls++
				if readCalls == 1 {
					return 0, registry.NONE, windows.ERROR_MORE_DATA
				}
				copy(buffer, []byte{0x41, 0x00})
				return 2, registry.EXPAND_SZ, nil
			},
			func() (int, uint32, error) {
				refreshCalls++
				return 2, registry.EXPAND_SZ, nil
			},
		)
		if err != nil {
			t.Fatalf("readRegistryPathRawValueWithRetry() error = %v", err)
		}
		if valueType != registry.EXPAND_SZ {
			t.Fatalf("valueType = %d, want %d", valueType, registry.EXPAND_SZ)
		}
		if !bytes.Equal(raw, []byte{0x41, 0x00}) {
			t.Fatalf("raw value = %v, want [65 0]", raw)
		}
		if refreshCalls != 1 {
			t.Fatalf("refresh call count = %d, want 1", refreshCalls)
		}
	})

	t.Run("returns error on persistent inconsistent value type after successful read", func(t *testing.T) {
		refreshCalls := 0
		_, _, err := readRegistryPathRawValueWithRetry(
			2,
			registry.SZ,
			func(buffer []byte) (int, uint32, error) {
				copy(buffer, []byte{0x41, 0x00})
				return 2, registry.EXPAND_SZ, nil
			},
			func() (int, uint32, error) {
				refreshCalls++
				return 2, registry.SZ, nil
			},
		)
		if err == nil {
			t.Fatal("readRegistryPathRawValueWithRetry() expected inconsistent value type error")
		}
		if !strings.Contains(err.Error(), "inconsistent value type") {
			t.Fatalf("error = %v, want inconsistent value type", err)
		}
		if refreshCalls != 2 {
			t.Fatalf("refresh call count = %d, want 2", refreshCalls)
		}
	})

	t.Run("retries when value type mismatch is transient", func(t *testing.T) {
		readCalls := 0
		raw, valueType, err := readRegistryPathRawValueWithRetry(
			2,
			registry.SZ,
			func(buffer []byte) (int, uint32, error) {
				readCalls++
				copy(buffer, []byte{0x41, 0x00})
				if readCalls == 1 {
					return 2, registry.EXPAND_SZ, nil
				}
				return 2, registry.EXPAND_SZ, nil
			},
			func() (int, uint32, error) {
				return 2, registry.EXPAND_SZ, nil
			},
		)
		if err != nil {
			t.Fatalf("readRegistryPathRawValueWithRetry() error = %v", err)
		}
		if valueType != registry.EXPAND_SZ {
			t.Fatalf("valueType = %d, want %d", valueType, registry.EXPAND_SZ)
		}
		if !bytes.Equal(raw, []byte{0x41, 0x00}) {
			t.Fatalf("raw value = %v, want [65 0]", raw)
		}
		if readCalls != 2 {
			t.Fatalf("read call count = %d, want 2", readCalls)
		}
	})

	t.Run("returns error on unsupported value type after refresh", func(t *testing.T) {
		_, _, err := readRegistryPathRawValueWithRetry(
			1,
			registry.SZ,
			func([]byte) (int, uint32, error) {
				return 0, registry.NONE, windows.ERROR_MORE_DATA
			},
			func() (int, uint32, error) {
				return 8, 9999, nil
			},
		)
		if err == nil {
			t.Fatal("readRegistryPathRawValueWithRetry() expected error")
		}
		if !strings.Contains(err.Error(), "unsupported value type") {
			t.Fatalf("error = %v, want unsupported value type", err)
		}
	})

	t.Run("returns error when size refresh fails after ERROR_MORE_DATA", func(t *testing.T) {
		_, _, err := readRegistryPathRawValueWithRetry(
			1,
			registry.SZ,
			func([]byte) (int, uint32, error) {
				return 0, registry.NONE, windows.ERROR_MORE_DATA
			},
			func() (int, uint32, error) {
				return 0, registry.NONE, windows.ERROR_ACCESS_DENIED
			},
		)
		if err == nil {
			t.Fatal("readRegistryPathRawValueWithRetry() expected size refresh error")
		}
		if !errors.Is(err, windows.ERROR_ACCESS_DENIED) {
			t.Fatalf("error = %v, want wrapped ERROR_ACCESS_DENIED", err)
		}
	})

	t.Run("returns error when refreshed raw size is negative", func(t *testing.T) {
		_, _, err := readRegistryPathRawValueWithRetry(
			1,
			registry.SZ,
			func([]byte) (int, uint32, error) {
				return 0, registry.NONE, windows.ERROR_MORE_DATA
			},
			func() (int, uint32, error) {
				return -2, registry.SZ, nil
			},
		)
		if err == nil {
			t.Fatal("readRegistryPathRawValueWithRetry() expected invalid raw size error")
		}
		if !strings.Contains(err.Error(), "invalid raw size") {
			t.Fatalf("error = %v, want invalid raw size", err)
		}
	})

	t.Run("returns immediate error on non-ERROR_MORE_DATA read failure", func(t *testing.T) {
		_, _, err := readRegistryPathRawValueWithRetry(
			4,
			registry.SZ,
			func([]byte) (int, uint32, error) {
				return 0, registry.NONE, windows.ERROR_ACCESS_DENIED
			},
			func() (int, uint32, error) {
				t.Fatal("readSizeAndType should not be called on non-ERROR_MORE_DATA failure")
				return 0, registry.NONE, nil
			},
		)
		if err == nil {
			t.Fatal("readRegistryPathRawValueWithRetry() expected immediate read error")
		}
		if !errors.Is(err, windows.ERROR_ACCESS_DENIED) {
			t.Fatalf("error = %v, want wrapped ERROR_ACCESS_DENIED", err)
		}
	})

	t.Run("returns retry limit exceeded when value keeps changing", func(t *testing.T) {
		refreshCalls := 0
		_, _, err := readRegistryPathRawValueWithRetry(
			1,
			registry.SZ,
			func([]byte) (int, uint32, error) {
				return 0, registry.NONE, windows.ERROR_MORE_DATA
			},
			func() (int, uint32, error) {
				refreshCalls++
				return 1, registry.SZ, nil
			},
		)
		if err == nil {
			t.Fatal("readRegistryPathRawValueWithRetry() expected retry-limit error")
		}
		if !strings.Contains(err.Error(), "retry limit exceeded") {
			t.Fatalf("error = %v, want retry limit exceeded", err)
		}
		if refreshCalls != 3 {
			t.Fatalf("refresh call count = %d, want 3", refreshCalls)
		}
	})

	t.Run("returns retry limit when n always exceeds buffer", func(t *testing.T) {
		readCalls := 0
		_, _, err := readRegistryPathRawValueWithRetry(
			0,
			registry.SZ,
			func(buffer []byte) (int, uint32, error) {
				readCalls++
				return len(buffer) + 4, registry.SZ, nil
			},
			func() (int, uint32, error) {
				t.Fatal("readSizeAndType should not be called in n>len path")
				return 0, registry.NONE, nil
			},
		)
		if err == nil {
			t.Fatal("readRegistryPathRawValueWithRetry() expected retry-limit error")
		}
		if !strings.Contains(err.Error(), "retry limit exceeded") {
			t.Fatalf("error = %v, want retry limit exceeded", err)
		}
		if readCalls != 3 {
			t.Fatalf("read call count = %d, want 3", readCalls)
		}
	})
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
		{
			name: "single byte is truncated to empty",
			raw:  []byte{0xAA},
			want: "",
		},
		{
			name: "CJK characters",
			raw:  encodeUTF16RegistryTestValue(`C:\ユーザー\bin`),
			want: `C:\ユーザー\bin`,
		},
		{
			name: "UTF-16 BOM is removed",
			raw:  append([]byte{0xFF, 0xFE}, encodeUTF16RegistryTestValue(`C:\tools\bom`)...),
			want: `C:\tools\bom`,
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
