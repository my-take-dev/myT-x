//go:build windows

package install

import (
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
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
)

var (
	user32DLL                 = windows.NewLazySystemDLL("user32.dll")
	procSendMessageTimeoutW   = user32DLL.NewProc("SendMessageTimeoutW")
	environmentSettingPayload = mustUTF16Ptr("Environment")
)

func ensurePathContains(installDir string) (bool, error) {
	currentPath := os.Getenv("PATH")
	if containsPathEntry(currentPath, installDir) {
		return false, nil
	}

	// Query registry value for user PATH because process PATH may differ.
	regValue, regValueType, err := readUserPathFromRegistryWithType()
	if err != nil {
		return false, err
	}
	if containsPathEntry(regValue, installDir) {
		return false, nil
	}

	newPath := strings.TrimSuffix(regValue, ";")
	if newPath != "" {
		newPath += ";"
	}
	newPath += installDir

	targetValueType := selectPathRegistryValueType(regValueType, newPath)
	slog.Debug("[DEBUG-SHIM] writing updated PATH to registry",
		"entryCount", countPathEntries(newPath),
		"valueType", pathRegistryTypeName(targetValueType))
	key, _, err := registry.CreateKey(registry.CURRENT_USER, registryEnvKeyPath, registry.SET_VALUE)
	if err != nil {
		return false, fmt.Errorf("open registry key for writing: %w", err)
	}
	defer key.Close()

	if err := setPathRegistryValue(key, newPath, regValueType); err != nil {
		return false, fmt.Errorf("update user PATH in registry: %w", err)
	}
	if err := broadcastEnvironmentSettingChange(); err != nil {
		// Non-fatal: PATH update succeeded. Existing behavior ("open a new terminal")
		// still works when broadcast delivery fails.
		slog.Debug("[DEBUG-SHIM] failed to broadcast WM_SETTINGCHANGE",
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

	if valueType != registry.SZ && valueType != registry.EXPAND_SZ {
		return "", registry.NONE, fmt.Errorf("read user PATH from registry: unsupported value type %d", valueType)
	}

	rawValue := make([]byte, rawSize)
	if rawSize > 0 {
		n, valueTypeRead, readErr := key.GetValue(pathValueName, rawValue)
		if readErr != nil {
			slog.Debug("[DEBUG-SHIM] failed to read raw Path value from registry", "error", readErr)
			return "", registry.NONE, fmt.Errorf("read user PATH from registry: %w", readErr)
		}
		if valueTypeRead != valueType {
			return "", registry.NONE, fmt.Errorf("read user PATH from registry: inconsistent value type %d (expected %d)", valueTypeRead, valueType)
		}
		rawValue = rawValue[:n]
	}

	value := decodeRegistryUTF16String(rawValue)
	slog.Debug("[DEBUG-SHIM] read user PATH from registry",
		"entryCount", countPathEntries(value),
		"valueType", pathRegistryTypeName(valueType))
	return value, valueType, nil
}

func setPathRegistryValue(key registry.Key, pathValue string, currentType uint32) error {
	switch selectPathRegistryValueType(currentType, pathValue) {
	case registry.SZ:
		return key.SetStringValue(pathValueName, pathValue)
	case registry.EXPAND_SZ:
		return key.SetExpandStringValue(pathValueName, pathValue)
	default:
		return fmt.Errorf("unsupported value type %d", currentType)
	}
}

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
		return currentType
	}
}

func decodeRegistryUTF16String(rawValue []byte) string {
	if len(rawValue) == 0 {
		return ""
	}
	if len(rawValue)%2 != 0 {
		slog.Debug("[DEBUG-SHIM] registry Path value has odd byte length, truncating last byte",
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
	ret, _, callErr := procSendMessageTimeoutW.Call(
		uintptr(hwndBroadcast),
		uintptr(wmSettingChange),
		0,
		uintptr(unsafe.Pointer(environmentSettingPayload)),
		uintptr(smtoAbortIfHung),
		uintptr(wmSettingChangeTimeoutMS),
		0,
	)
	if ret == 0 {
		if callErr == windows.ERROR_SUCCESS {
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
	normalizedEntry := strings.ToLower(strings.TrimSpace(filepath.Clean(entry)))
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
