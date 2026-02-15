//go:build windows

package terminal

import (
	"fmt"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Windows API DLL and procedure definitions
var (
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procCreatePseudoConsole          = kernel32.NewProc("CreatePseudoConsole")
	procResizePseudoConsole          = kernel32.NewProc("ResizePseudoConsole")
	procClosePseudoConsole           = kernel32.NewProc("ClosePseudoConsole")
	procInitializeProcThreadAttrList = kernel32.NewProc("InitializeProcThreadAttributeList")
	procDeleteProcThreadAttrList     = kernel32.NewProc("DeleteProcThreadAttributeList")
	procUpdateProcThreadAttribute    = kernel32.NewProc("UpdateProcThreadAttribute")
)

// Windows API constants
const (
	_S_OK                                = 0
	_PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE = 0x20016
)

// _COORD represents console coordinates (Windows COORD structure)
type _COORD struct {
	X int16
	Y int16
}

// Pack converts the coordinate to a packed uintptr for Windows API calls
func (c *_COORD) Pack() uintptr {
	return uintptr((int32(c.Y) << 16) | int32(c.X))
}

// _HPCON is a pseudo console handle
type _HPCON windows.Handle

// isConPtyAvailable checks if CreatePseudoConsole API is available
func isConPtyAvailable() bool {
	return procCreatePseudoConsole.Find() == nil
}

// createPseudoConsole wraps the Windows CreatePseudoConsole API
func createPseudoConsole(size *_COORD, hInput, hOutput windows.Handle) (_HPCON, error) {
	var hpCon _HPCON
	ret, _, lastErr := procCreatePseudoConsole.Call(
		size.Pack(),
		uintptr(hInput),
		uintptr(hOutput),
		0,
		uintptr(unsafe.Pointer(&hpCon)),
	)
	if ret != _S_OK {
		return 0, fmt.Errorf("CreatePseudoConsole failed with code: 0x%x, lastError: %v", ret, lastErr)
	}
	return hpCon, nil
}

// resizePseudoConsole wraps the Windows ResizePseudoConsole API
func resizePseudoConsole(hpCon _HPCON, size *_COORD) error {
	ret, _, lastErr := procResizePseudoConsole.Call(uintptr(hpCon), size.Pack())
	if ret != _S_OK {
		return fmt.Errorf("ResizePseudoConsole failed with code: 0x%x, lastError: %v", ret, lastErr)
	}
	return nil
}

// closePseudoConsole wraps the Windows ClosePseudoConsole API
func closePseudoConsole(hpCon _HPCON) {
	procClosePseudoConsole.Call(uintptr(hpCon))
}

// initializeProcThreadAttrList initializes a process thread attribute list.
// Returns the allocated attribute list and any error encountered.
func initializeProcThreadAttrList() ([]byte, error) {
	var size uintptr
	// First call determines required size (expected to fail with size > 0)
	_, _, firstErr := procInitializeProcThreadAttrList.Call(0, 1, 0, uintptr(unsafe.Pointer(&size)))
	if size == 0 {
		return nil, fmt.Errorf("failed to get attribute list size, lastError: %v", firstErr)
	}

	attrList := make([]byte, size)
	ret, _, lastErr := procInitializeProcThreadAttrList.Call(
		uintptr(unsafe.Pointer(&attrList[0])),
		1, 0,
		uintptr(unsafe.Pointer(&size)),
	)
	if ret == 0 {
		return nil, fmt.Errorf("InitializeProcThreadAttributeList failed, lastError: %v", lastErr)
	}
	return attrList, nil
}

// updateProcThreadAttrWithPseudoConsole sets the pseudo console attribute
func updateProcThreadAttrWithPseudoConsole(attrList []byte, hpCon _HPCON) error {
	ret, _, lastErr := procUpdateProcThreadAttribute.Call(
		uintptr(unsafe.Pointer(&attrList[0])),
		0,
		_PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE,
		uintptr(hpCon),
		unsafe.Sizeof(hpCon),
		0, 0,
	)
	if ret == 0 {
		return fmt.Errorf("UpdateProcThreadAttribute failed, lastError: %v", lastErr)
	}
	return nil
}

// deleteProcThreadAttrList frees resources allocated by initializeProcThreadAttrList.
func deleteProcThreadAttrList(attrList []byte) {
	if len(attrList) > 0 {
		procDeleteProcThreadAttrList.Call(uintptr(unsafe.Pointer(&attrList[0])))
	}
}

// createEnvBlock creates a Windows environment block from a string slice.
// Empty strings are filtered out to prevent a stray null terminator from
// being misinterpreted as the double-null block terminator.
func createEnvBlock(env []string) *uint16 {
	if len(env) == 0 {
		return nil
	}
	var block []uint16
	for _, e := range env {
		if e == "" {
			continue
		}
		block = append(block, utf16.Encode([]rune(e))...)
		block = append(block, 0)
	}
	if len(block) == 0 {
		return nil
	}
	block = append(block, 0)
	return &block[0]
}
