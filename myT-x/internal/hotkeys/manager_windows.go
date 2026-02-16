//go:build windows

package hotkeys

import (
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

var (
	user32DLL = syscall.NewLazyDLL("user32.dll")
	kernelDLL = syscall.NewLazyDLL("kernel32.dll")

	procRegisterHotKey     = user32DLL.NewProc("RegisterHotKey")
	procUnregisterHotKey   = user32DLL.NewProc("UnregisterHotKey")
	procGetMessageW        = user32DLL.NewProc("GetMessageW")
	procTranslateMessage   = user32DLL.NewProc("TranslateMessage")
	procDispatchMessageW   = user32DLL.NewProc("DispatchMessageW")
	procPostThreadMessageW = user32DLL.NewProc("PostThreadMessageW")
	procPeekMessageW       = user32DLL.NewProc("PeekMessageW")
	procGetCurrentThreadID = kernelDLL.NewProc("GetCurrentThreadId")
)

const (
	wmHotkey   = 0x0312
	wmQuit     = 0x0012
	pmNoRemove = 0x0000

	// maxHotkeyID is the upper bound for application-defined hotkey IDs (Win32).
	maxHotkeyID int32 = 0xBFFF
)

var nextHotkeyID int32 = 0x4000

// activeHotkey holds the state of a single active hotkey registration.
// When non-nil in Manager, all fields are valid and a message loop goroutine is running.
type activeHotkey struct {
	hotkeyID int32
	threadID uint32
	doneCh   chan struct{}
	binding  string
}

// point mirrors the Win32 POINT struct.
type point struct {
	x int32
	y int32
}

// winMsg mirrors the Win32 MSG struct (tagMSG from winuser.h).
// Field order and types must not be changed -- the layout must match
// the Win32 binary layout on both 32-bit and 64-bit Windows.
type winMsg struct {
	hWnd     uintptr
	message  uint32
	wParam   uintptr
	lParam   uintptr
	time     uint32
	pt       point
	lPrivate uint32 // reserved by Windows; required for correct struct size
}

type loopReady struct {
	threadID uint32
	err      error
}

// Manager manages one global hotkey registration.
type Manager struct {
	mu     sync.Mutex
	active *activeHotkey // nil when no hotkey is registered
}

// NewManager creates a new hotkey manager.
func NewManager() *Manager {
	return &Manager{}
}

// Start registers a global hotkey and binds onTrigger to it.
func (m *Manager) Start(spec string, onTrigger func()) error {
	if onTrigger == nil {
		return errors.New("onTrigger callback is required")
	}

	// Pre-check DLL availability so that failures produce clean errors
	// instead of panics from LazyProc.Call.
	if err := user32DLL.Load(); err != nil {
		return fmt.Errorf("user32.dll is unavailable: %w", err)
	}
	if err := kernelDLL.Load(); err != nil {
		return fmt.Errorf("kernel32.dll is unavailable: %w", err)
	}

	binding, err := ParseBinding(spec)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.stopLocked(); err != nil {
		return err
	}

	hotkeyID := atomic.AddInt32(&nextHotkeyID, 1)
	if hotkeyID < 0 || hotkeyID > maxHotkeyID {
		return fmt.Errorf("hotkey ID range exhausted (ID=%d)", hotkeyID)
	}

	readyCh := make(chan loopReady, 1)
	doneCh := make(chan struct{})

	go runHotkeyLoop(hotkeyID, binding, onTrigger, readyCh, doneCh)

	ready := <-readyCh
	if ready.err != nil {
		return fmt.Errorf("register hotkey %q failed: %w", binding.Normalized(), ready.err)
	}
	if ready.threadID == 0 {
		return errors.New("hotkey loop started but returned invalid thread ID 0")
	}

	m.active = &activeHotkey{
		hotkeyID: hotkeyID,
		threadID: ready.threadID,
		doneCh:   doneCh,
		binding:  binding.Normalized(),
	}
	return nil
}

// Stop unregisters the active global hotkey.
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stopLocked()
}

// ActiveBinding returns the normalized binding string for the active hotkey.
func (m *Manager) ActiveBinding() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.active == nil {
		return ""
	}
	return m.active.binding
}

func (m *Manager) stopLocked() error {
	if m.active == nil {
		return nil
	}

	ah := m.active
	// Clear the active pointer first so that concurrent ActiveBinding() calls
	// see the manager as idle. The actual cleanup follows using the local copy.
	m.active = nil

	stopErr := postQuit(ah.threadID)
	if stopErr != nil {
		if unregErr := unregisterHotKey(ah.hotkeyID); unregErr != nil {
			slog.Warn("[hotkey] DEBUG unregisterHotKey fallback failed (cross-thread; may be expected)",
				"error", unregErr, "hotkeyID", ah.hotkeyID)
		}
	}

	timer := time.NewTimer(2 * time.Second)
	defer func() {
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
	}()

	select {
	case <-ah.doneCh:
		// Loop exited cleanly.
	case <-timer.C:
		timeoutErr := fmt.Errorf("hotkey message loop stop timed out (hotkeyID=%d)", ah.hotkeyID)
		slog.Warn("[hotkey] DEBUG message loop stop timed out, goroutine/thread may leak",
			"hotkeyID", ah.hotkeyID)
		stopErr = errors.Join(stopErr, timeoutErr)
	}

	return stopErr
}

func runHotkeyLoop(hotkeyID int32, binding Binding, onTrigger func(), readyCh chan<- loopReady, doneCh chan struct{}) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer close(doneCh)

	threadID, err := getCurrentThreadID()
	if err != nil {
		readyCh <- loopReady{err: err}
		return
	}

	// PeekMessageW forces Windows to create the thread message queue so that
	// PostThreadMessageW in Stop() can deliver WM_QUIT. The return value is
	// intentionally not checked for success: queue creation is a side-effect of
	// the call itself and returns 0 when no messages exist. However, we do log
	// errors for diagnostic purposes in restricted environments.
	var qmsg winMsg
	ret, _, peekErr := procPeekMessageW.Call(
		uintptr(unsafe.Pointer(&qmsg)),
		0,
		0,
		0,
		pmNoRemove,
	)
	if ret == 0 && peekErr != syscall.Errno(0) {
		slog.Warn("[hotkey] DEBUG PeekMessageW for queue init returned error",
			"error", peekErr, "hotkeyID", hotkeyID)
	}

	if err := registerHotKey(hotkeyID, uint32(binding.Modifiers()), uint32(binding.Key())); err != nil {
		readyCh <- loopReady{err: err}
		return
	}
	defer func() {
		if err := unregisterHotKey(hotkeyID); err != nil {
			slog.Error("[hotkey] DEBUG unregisterHotKey on loop exit failed (resource leak)",
				"error", err, "hotkeyID", hotkeyID)
		}
	}()

	readyCh <- loopReady{threadID: threadID}

	for {
		var msg winMsg
		ret, _, lastErr := procGetMessageW.Call(
			uintptr(unsafe.Pointer(&msg)),
			0,
			0,
			0,
		)
		switch int32(ret) {
		case -1:
			slog.Warn("[hotkey] DEBUG GetMessageW returned error, exiting loop", "error", lastErr, "hotkeyID", hotkeyID)
			return
		case 0:
			// WM_QUIT received -- normal shutdown path.
			slog.Info("[hotkey] DEBUG message loop received WM_QUIT, exiting normally", "hotkeyID", hotkeyID)
			return
		}

		if msg.message == wmHotkey && int32(msg.wParam) == hotkeyID {
			go onTrigger()
			continue
		}

		// TranslateMessage and DispatchMessageW return values are informational
		// (whether the message was translated / window procedure result) and are
		// not error indicators for a thread-level message loop without a window.
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

func registerHotKey(hotkeyID int32, modifiers uint32, key uint32) error {
	res, _, err := procRegisterHotKey.Call(
		0,
		uintptr(hotkeyID),
		uintptr(modifiers),
		uintptr(key),
	)
	if res != 0 {
		return nil
	}
	if err == syscall.Errno(0) {
		return errors.New("RegisterHotKey failed")
	}
	return err
}

func unregisterHotKey(hotkeyID int32) error {
	res, _, err := procUnregisterHotKey.Call(0, uintptr(hotkeyID))
	if res != 0 {
		return nil
	}
	if err == syscall.Errno(0) {
		return errors.New("UnregisterHotKey failed")
	}
	return err
}

func postQuit(threadID uint32) error {
	if threadID == 0 {
		return errors.New("cannot post WM_QUIT: threadID is 0")
	}
	res, _, err := procPostThreadMessageW.Call(
		uintptr(threadID),
		wmQuit,
		0,
		0,
	)
	if res != 0 {
		return nil
	}
	if err == syscall.Errno(0) {
		return errors.New("PostThreadMessageW failed")
	}
	return err
}

func getCurrentThreadID() (uint32, error) {
	tid, _, err := procGetCurrentThreadID.Call()
	if tid == 0 {
		return 0, fmt.Errorf("GetCurrentThreadId returned 0: %w", err)
	}
	return uint32(tid), nil
}
