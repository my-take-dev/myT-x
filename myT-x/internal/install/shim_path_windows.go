//go:build windows

package install

import (
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	// registryEnvKeyPath is the registry key path for user environment variables.
	registryEnvKeyPath = `Environment`
	// pathValueName is the registry value name for the user PATH variable.
	pathValueName = "Path"
	// WM_SETTINGCHANGE notifies top-level windows that system settings changed.
	wmSettingChange = 0x001A
	// HWND_BROADCAST sends a message to all top-level windows.
	hwndBroadcast = 0xffff
	// SMTO_ABORTIFHUNG prevents blocking on unresponsive windows.
	smtoAbortIfHung = 0x0002
	// Timeout for WM_SETTINGCHANGE broadcast to avoid indefinite blocking.
	wmSettingChangeTimeoutMS = 5000
	// Maximum allowed raw registry Path value size (64KB).
	// This bound guards against unexpectedly large reads and allocation spikes.
	maxRegistryPathRawSize = 64 * 1024
)

var (
	user32DLL                 = windows.NewLazySystemDLL("user32.dll")
	procSendMessageTimeoutW   = user32DLL.NewProc("SendMessageTimeoutW")
	environmentSettingPayload = mustUTF16Ptr("Environment")
	ensurePathMu              sync.Mutex
)

func ensurePathContains(installDir string) (bool, error) {
	ensurePathMu.Lock()
	defer ensurePathMu.Unlock()

	// Read and write user PATH through a single registry handle to avoid
	// read-then-write races when concurrent processes update PATH.
	key, _, err := registry.CreateKey(
		registry.CURRENT_USER,
		registryEnvKeyPath,
		registry.QUERY_VALUE|registry.SET_VALUE,
	)
	if err != nil {
		return false, fmt.Errorf("open registry key for read/write: %w", err)
	}
	defer key.Close()

	regValue, regValueType, err := readUserPathFromRegistryKeyWithType(key)
	if err != nil {
		return false, err
	}
	if containsPathEntry(regValue, installDir) {
		return false, nil
	}

	newPath := strings.TrimRight(regValue, ";")
	if newPath != "" {
		newPath += ";"
	}
	newPath += installDir

	targetValueType := selectPathRegistryValueType(regValueType, newPath)
	slog.Debug("[DEBUG-SHIM] writing updated PATH to registry",
		"entryCount", countPathEntries(newPath),
		"valueType", pathRegistryTypeName(targetValueType))
	if err := setPathRegistryValue(key, newPath, targetValueType); err != nil {
		return false, fmt.Errorf("update user PATH in registry: %w", err)
	}
	if err := broadcastEnvironmentSettingChange(); err != nil {
		// Non-fatal: PATH update succeeded. Existing behavior ("open a new terminal")
		// still works when broadcast delivery fails.
		slog.Warn("[WARN-SHIM] failed to broadcast WM_SETTINGCHANGE",
			"error", err)
	}
	return true, nil
}

func readUserPathFromRegistry() (string, error) {
	value, _, err := readUserPathFromRegistryWithType()
	return value, err
}

func readUserPathFromRegistryWithType() (string, uint32, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, registryEnvKeyPath, registry.QUERY_VALUE)
	if err != nil {
		// If the Environment key itself does not exist, treat as empty PATH.
		if errors.Is(err, registry.ErrNotExist) {
			slog.Debug("[DEBUG-SHIM] registry Environment key does not exist, treating as empty PATH")
			return "", registry.NONE, nil
		}
		slog.Debug("[DEBUG-SHIM] failed to open registry key for reading",
			"error", err)
		return "", registry.NONE, fmt.Errorf("open registry key for reading: %w", err)
	}
	defer key.Close()

	return readUserPathFromRegistryKeyWithType(key)
}

func readUserPathFromRegistryKeyWithType(key registry.Key) (string, uint32, error) {
	rawSize, valueType, err := key.GetValue(pathValueName, nil)
	if err != nil {
		// Path value not yet created — treat as empty (first install).
		if errors.Is(err, registry.ErrNotExist) {
			slog.Debug("[DEBUG-SHIM] registry Path value does not exist, treating as empty PATH")
			return "", registry.NONE, nil
		}
		slog.Debug("[DEBUG-SHIM] failed to read Path value from registry",
			"error", err)
		return "", registry.NONE, fmt.Errorf("read user PATH from registry: %w", err)
	}

	if err := validateRegistryPathValueType(valueType); err != nil {
		return "", registry.NONE, err
	}

	rawValue, valueType, err := readRegistryPathRawValueWithRetry(
		rawSize,
		valueType,
		func(buffer []byte) (int, uint32, error) {
			return key.GetValue(pathValueName, buffer)
		},
		func() (int, uint32, error) {
			return key.GetValue(pathValueName, nil)
		},
	)
	if err != nil {
		return "", registry.NONE, err
	}

	value := decodeRegistryUTF16String(rawValue)
	slog.Debug("[DEBUG-SHIM] read user PATH from registry",
		"entryCount", countPathEntries(value),
		"valueType", pathRegistryTypeName(valueType))
	return value, valueType, nil
}

func readRegistryPathRawValueWithRetry(
	rawSize int,
	valueType uint32,
	readValue func([]byte) (int, uint32, error),
	readSizeAndType func() (int, uint32, error),
) ([]byte, uint32, error) {
	if err := validateRegistryPathRawSize(rawSize); err != nil {
		return nil, registry.NONE, err
	}
	// Read with bounded retries to tolerate size changes between length query and read.
	// This can happen when another process updates user PATH concurrently.
	const maxReadAttempts = 3
	for attempt := 1; attempt <= maxReadAttempts; attempt++ {
		buffer := make([]byte, rawSize)
		n, valueTypeRead, readErr := readValue(buffer)
		if readErr == nil {
			if n < 0 {
				return nil, registry.NONE, fmt.Errorf("read user PATH from registry: invalid read size %d", n)
			}
			if valueTypeRead != valueType {
				slog.Debug("[DEBUG-SHIM] registry Path value type changed while reading, retrying",
					"attempt", attempt, "maxAttempts", maxReadAttempts, "expectedType", valueType, "actualType", valueTypeRead)
				if attempt == maxReadAttempts {
					return nil, registry.NONE, fmt.Errorf("read user PATH from registry: inconsistent value type %d (expected %d)", valueTypeRead, valueType)
				}
				var sizeErr error
				rawSize, valueType, sizeErr = readSizeAndType()
				if sizeErr != nil {
					slog.Debug("[DEBUG-SHIM] failed to refresh Path value metadata after value type mismatch", "error", sizeErr)
					return nil, registry.NONE, fmt.Errorf("read user PATH from registry: %w", sizeErr)
				}
				if err := validateRegistryPathRawSize(rawSize); err != nil {
					return nil, registry.NONE, err
				}
				if err := validateRegistryPathValueType(valueType); err != nil {
					return nil, registry.NONE, err
				}
				continue
			}
			if n > len(buffer) {
				// Defensive path for mock/custom readers or zero-length initial buffers:
				// when n exceeds the current buffer, retry with the reported size.
				if err := validateRegistryPathValueType(valueTypeRead); err != nil {
					return nil, registry.NONE, fmt.Errorf("%w on size-growth retry", err)
				}
				slog.Debug("[DEBUG-SHIM] registry Path value exceeded current buffer size, retrying",
					"attempt", attempt, "maxAttempts", maxReadAttempts, "requestedSize", len(buffer), "actualSize", n)
				rawSize = n
				valueType = valueTypeRead
				continue
			}
			return buffer[:n], valueType, nil
		}
		if errors.Is(readErr, windows.ERROR_MORE_DATA) {
			slog.Debug("[DEBUG-SHIM] registry Path value size changed while reading, retrying",
				"attempt", attempt, "maxAttempts", maxReadAttempts)
			var sizeErr error
			// Refresh both size and valueType because registry values can change between retries.
			rawSize, valueType, sizeErr = readSizeAndType()
			if sizeErr != nil {
				slog.Debug("[DEBUG-SHIM] failed to refresh Path value size after ERROR_MORE_DATA", "error", sizeErr)
				return nil, registry.NONE, fmt.Errorf("read user PATH from registry: %w", sizeErr)
			}
			if err := validateRegistryPathRawSize(rawSize); err != nil {
				return nil, registry.NONE, err
			}
			if err := validateRegistryPathValueType(valueType); err != nil {
				return nil, registry.NONE, err
			}
			continue
		}

		slog.Debug("[DEBUG-SHIM] failed to read raw Path value from registry", "error", readErr)
		return nil, registry.NONE, fmt.Errorf("read user PATH from registry: %w", readErr)
	}

	return nil, registry.NONE, fmt.Errorf("read user PATH from registry: retry limit exceeded (%d attempts)", maxReadAttempts)
}

func validateRegistryPathRawSize(rawSize int) error {
	if rawSize < 0 {
		return fmt.Errorf("read user PATH from registry: invalid raw size %d", rawSize)
	}
	if rawSize > maxRegistryPathRawSize {
		return fmt.Errorf("read user PATH from registry: raw size %d exceeds limit %d", rawSize, maxRegistryPathRawSize)
	}
	return nil
}

func validateRegistryPathValueType(valueType uint32) error {
	if valueType != registry.SZ && valueType != registry.EXPAND_SZ {
		return fmt.Errorf("read user PATH from registry: unsupported value type %d", valueType)
	}
	return nil
}

func setPathRegistryValue(key registry.Key, pathValue string, targetType uint32) error {
	switch targetType {
	case registry.SZ:
		return key.SetStringValue(pathValueName, pathValue)
	case registry.EXPAND_SZ:
		return key.SetExpandStringValue(pathValueName, pathValue)
	default:
		return fmt.Errorf("unsupported value type %d", targetType)
	}
}

// selectPathRegistryValueType determines the registry value type (SZ or EXPAND_SZ)
// for writing the PATH value. For known types (SZ, EXPAND_SZ), the current type is
// preserved. For NONE (new entry), auto-detects based on "%" markers.
// For unknown types, the value is returned as-is; the caller is responsible
// for rejecting unsupported types (e.g. setPathRegistryValue returns an error).
func selectPathRegistryValueType(currentType uint32, pathValue string) uint32 {
	switch currentType {
	case registry.SZ, registry.EXPAND_SZ:
		return currentType
	case registry.NONE:
		if strings.Contains(pathValue, "%") {
			return registry.EXPAND_SZ
		}
		return registry.SZ
	default:
		// Unknown types are returned as-is so the caller can reject explicitly.
		return currentType
	}
}

func decodeRegistryUTF16String(rawValue []byte) string {
	if len(rawValue) == 0 {
		return ""
	}
	if len(rawValue)%2 != 0 {
		slog.Warn("[WARN-SHIM] registry Path value has odd byte length, truncating last byte",
			"byteLength", len(rawValue))
		rawValue = rawValue[:len(rawValue)-1]
	}

	utf16Data := make([]uint16, 0, len(rawValue)/2)
	for i := 0; i+1 < len(rawValue); i += 2 {
		ch := binary.LittleEndian.Uint16(rawValue[i : i+2])
		if ch == 0 {
			break
		}
		utf16Data = append(utf16Data, ch)
	}
	if len(utf16Data) > 0 && utf16Data[0] == 0xFEFF {
		utf16Data = utf16Data[1:]
	}
	return string(utf16.Decode(utf16Data))
}

func countPathEntries(pathValue string) int {
	count := 0
	for _, item := range strings.Split(pathValue, ";") {
		if strings.TrimSpace(item) != "" {
			count++
		}
	}
	return count
}

func pathRegistryTypeName(valueType uint32) string {
	switch valueType {
	case registry.NONE:
		return "NONE"
	case registry.SZ:
		return "SZ"
	case registry.EXPAND_SZ:
		return "EXPAND_SZ"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", valueType)
	}
}

func broadcastEnvironmentSettingChange() error {
	var result uintptr // Populated by SendMessageTimeoutW; not inspected by this caller.
	ret, _, callErr := procSendMessageTimeoutW.Call(
		uintptr(hwndBroadcast),
		uintptr(wmSettingChange),
		0,
		uintptr(unsafe.Pointer(environmentSettingPayload)),
		uintptr(smtoAbortIfHung),
		uintptr(wmSettingChangeTimeoutMS),
		uintptr(unsafe.Pointer(&result)),
	)
	if ret == 0 {
		if callErr == nil || errors.Is(callErr, windows.ERROR_SUCCESS) {
			return errors.New("SendMessageTimeoutW returned 0 without extended error")
		}
		return fmt.Errorf("SendMessageTimeoutW failed: %w", callErr)
	}
	return nil
}

func mustUTF16Ptr(value string) *uint16 {
	ptr, err := windows.UTF16PtrFromString(value)
	if err != nil {
		panic(err)
	}
	return ptr
}

func containsPathEntry(pathValue string, entry string) bool {
	normalizedEntry := strings.TrimSpace(entry)
	if normalizedEntry == "" {
		return false
	}
	normalizedEntry = strings.ToLower(filepath.Clean(normalizedEntry))
	for _, item := range strings.Split(pathValue, ";") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if strings.ToLower(filepath.Clean(item)) == normalizedEntry {
			return true
		}
	}
	return false
}
