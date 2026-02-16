//go:build windows

package terminal

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"runtime"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// ErrConPtyUnsupported indicates ConPTY is not available on this Windows version.
var ErrConPtyUnsupported = errors.New("ConPty is not available on this version of Windows")

var (
	waitForSingleObjectFn = windows.WaitForSingleObject
	terminateProcessFn    = windows.TerminateProcess
)

// handleIO wraps a Windows pipe handle used by ConPTY I/O.
// Methods copy the raw handle under lock, then perform blocking syscalls
// unlocked so Close can invalidate the handle without deadlocking readers/writers.
type handleIO struct {
	mu     sync.Mutex
	handle windows.Handle
}

func (h *handleIO) Read(p []byte) (int, error) {
	// Read copies the handle under lock and then performs blocking I/O without
	// holding the mutex. Close may race and invalidate the handle; callers
	// normalize closed-handle errors to keep this race safe and non-panicking.
	h.mu.Lock()
	handle := h.handle
	h.mu.Unlock()
	if handle == 0 || handle == windows.InvalidHandle {
		return 0, io.EOF
	}

	var numRead uint32
	err := windows.ReadFile(handle, p, &numRead, nil)
	return int(numRead), normalizeReadFileError(err)
}

func (h *handleIO) Write(p []byte) (int, error) {
	// Write mirrors Read: acquire handle snapshot under lock, then perform
	// blocking syscall unlocked so Close cannot deadlock waiting on I/O.
	h.mu.Lock()
	handle := h.handle
	h.mu.Unlock()
	if handle == 0 || handle == windows.InvalidHandle {
		return 0, io.ErrClosedPipe
	}

	var numWritten uint32
	err := windows.WriteFile(handle, p, &numWritten, nil)
	return int(numWritten), normalizeWriteFileError(err)
}

func (h *handleIO) Close() error {
	h.mu.Lock()
	handle := h.handle
	if handle == 0 || handle == windows.InvalidHandle {
		h.mu.Unlock()
		return nil
	}
	h.handle = windows.InvalidHandle
	h.mu.Unlock()

	err := windows.CloseHandle(handle)
	if err != nil {
		slog.Debug("[DEBUG-CONPTY] handleIO.Close failed", "error", err)
	}
	return err
}

// ConPty represents a Windows pseudo console.
type ConPty struct {
	stateMu   sync.RWMutex
	hpCon     _HPCON
	pi        *windows.ProcessInformation
	cmdIn     *handleIO
	cmdOut    *handleIO
	closeOnce sync.Once
	closeErr  error // Written once by closeOnce and reused by subsequent Close calls.
}

// IsConPtyAvailable checks if ConPTY is supported on this Windows version.
func IsConPtyAvailable() bool {
	return isConPtyAvailable()
}

// conPtyArgs holds configuration for ConPTY creation.
type conPtyArgs struct {
	coord               _COORD
	hasCustomDimensions bool
	width               int
	height              int
	workDir             string
	env                 []string
}

// ConPtyOption is a functional option for ConPTY configuration.
type ConPtyOption func(*conPtyArgs)

const (
	defaultConPtyWidth  = 80
	defaultConPtyHeight = 40
	// gracePeriodMS balances fast close behavior and normal shell exit latency.
	gracePeriodMS = 500
	// terminateWaitMS is a short post-terminate wait to observe process exit state.
	terminateWaitMS       = 100
	waitTimeoutResultCode = uint32(windows.WAIT_TIMEOUT)
)

// ConPtyDimensions sets the console dimensions.
func ConPtyDimensions(width, height int) ConPtyOption {
	return func(args *conPtyArgs) {
		args.hasCustomDimensions = true
		args.width = width
		args.height = height
	}
}

// ConPtyWorkDir sets the working directory.
func ConPtyWorkDir(workDir string) ConPtyOption {
	return func(args *conPtyArgs) {
		args.workDir = workDir
	}
}

// ConPtyEnv sets environment variables.
func ConPtyEnv(env []string) ConPtyOption {
	return func(args *conPtyArgs) {
		args.env = env
	}
}

// startConPty creates a new ConPTY and starts a process.
func startConPty(commandLine string, options ...ConPtyOption) (*ConPty, error) {
	if !IsConPtyAvailable() {
		return nil, ErrConPtyUnsupported
	}

	args := &conPtyArgs{
		coord: _COORD{X: defaultConPtyWidth, Y: defaultConPtyHeight},
	}
	for _, opt := range options {
		opt(args)
	}
	dimWidth := int(args.coord.X)
	dimHeight := int(args.coord.Y)
	if args.hasCustomDimensions {
		dimWidth = args.width
		dimHeight = args.height
	}
	if err := validateConPtyDimensions(dimWidth, dimHeight); err != nil {
		return nil, err
	}
	args.coord = _COORD{X: int16(dimWidth), Y: int16(dimHeight)}

	ptyIn, cmdIn, cmdOut, ptyOut, err := createPtyPipes()
	if err != nil {
		return nil, err
	}

	hpCon, err := createPseudoConsole(&args.coord, ptyIn, ptyOut)
	if err != nil {
		closeHandles(ptyIn, ptyOut, cmdIn, cmdOut)
		return nil, err
	}
	// On Windows 10 1809+ CreatePseudoConsole takes ownership of ptyIn/ptyOut.
	// Close local duplicates immediately to avoid delaying broken-pipe detection.
	closeHandles(ptyIn, ptyOut)

	pi, err := createConPtyProcess(commandLine, args, hpCon)
	if err != nil {
		closePseudoConsole(hpCon)
		closeHandles(cmdIn, cmdOut)
		return nil, err
	}

	return &ConPty{
		hpCon:  hpCon,
		pi:     pi,
		cmdIn:  &handleIO{handle: cmdIn},
		cmdOut: &handleIO{handle: cmdOut},
	}, nil
}

// createPtyPipes creates the input and output pipes for PTY communication.
func createPtyPipes() (ptyIn windows.Handle, cmdIn windows.Handle, cmdOut windows.Handle, ptyOut windows.Handle, err error) {
	if err = windows.CreatePipe(&ptyIn, &cmdIn, nil, 0); err != nil {
		return 0, 0, 0, 0, fmt.Errorf("failed to create input pipe: %w", err)
	}
	if err = windows.CreatePipe(&cmdOut, &ptyOut, nil, 0); err != nil {
		closeHandles(ptyIn, cmdIn)
		return 0, 0, 0, 0, fmt.Errorf("failed to create output pipe: %w", err)
	}
	return
}

// closeHandles closes all provided Windows handles.
func closeHandles(handles ...windows.Handle) {
	for _, h := range handles {
		if h == 0 || h == windows.InvalidHandle {
			continue
		}
		if err := windows.CloseHandle(h); err != nil {
			slog.Debug("[DEBUG-CONPTY] CloseHandle failed", "handle", h, "error", err)
		}
	}
}

type startupInfoEx struct {
	startupInfo   windows.StartupInfo
	attributeList []byte
}

func getStartupInfoExForPTY(hpCon _HPCON) (*startupInfoEx, error) {
	siEx := &startupInfoEx{}
	// STARTUPINFOEXW = STARTUPINFOW + lpAttributeList pointer.
	// This evaluates to 112 bytes on Win64 and 72 bytes on Win32.
	siEx.startupInfo.Cb = uint32(unsafe.Sizeof(windows.StartupInfo{}) + unsafe.Sizeof(uintptr(0)))
	// Keep STARTF_USESTDHANDLES for compatibility with this startup configuration.
	// Removing it causes interactive ConPTY input tests to fail in this codebase.
	siEx.startupInfo.Flags |= windows.STARTF_USESTDHANDLES

	attrList, err := initializeProcThreadAttrList()
	if err != nil {
		return nil, err
	}
	siEx.attributeList = attrList

	if err := updateProcThreadAttrWithPseudoConsole(siEx.attributeList, hpCon); err != nil {
		deleteProcThreadAttrList(siEx.attributeList)
		return nil, err
	}
	return siEx, nil
}

// createConPtyProcess creates a process attached to the pseudo console.
func createConPtyProcess(commandLine string, args *conPtyArgs, hpCon _HPCON) (*windows.ProcessInformation, error) {
	cmdLinePtr, err := windows.UTF16PtrFromString(commandLine)
	if err != nil {
		return nil, err
	}

	var workDirPtr *uint16
	if args.workDir != "" {
		workDirPtr, err = windows.UTF16PtrFromString(args.workDir)
		if err != nil {
			return nil, err
		}
	}

	siEx, err := getStartupInfoExForPTY(hpCon)
	if err != nil {
		return nil, fmt.Errorf("failed to build startup info for ConPTY: %w", err)
	}
	defer deleteProcThreadAttrList(siEx.attributeList)

	var pi windows.ProcessInformation
	envBlock := createEnvBlock(args.env)
	var flags uint32 = windows.EXTENDED_STARTUPINFO_PRESENT
	if envBlock != nil {
		flags |= windows.CREATE_UNICODE_ENVIRONMENT
	}

	err = windows.CreateProcess(
		nil,
		cmdLinePtr,
		nil,
		nil,
		false,
		flags,
		envBlock,
		workDirPtr,
		&siEx.startupInfo,
		&pi,
	)
	// Keep envBlock alive until CreateProcess returns to avoid premature GC while
	// Windows may still read the UTF-16 environment block.
	runtime.KeepAlive(envBlock)
	if err != nil {
		return nil, fmt.Errorf("CreateProcess failed: %w", err)
	}

	return &pi, nil
}

// Read reads from the pseudo console output.
// Read may race with Close: if Close runs between RUnlock and ReadFile,
// the OS returns ERROR_INVALID_HANDLE/ERROR_BROKEN_PIPE, which
// normalizeConPtyPipeError converts to a clear "closed" message.
// Holding the lock during blocking I/O would risk deadlock with Close.
func (c *ConPty) Read(p []byte) (int, error) {
	c.stateMu.RLock()
	cmdOut := c.cmdOut
	c.stateMu.RUnlock()
	if cmdOut == nil {
		return 0, errors.New("Read called on closed pseudo console (nil handle)")
	}
	n, err := cmdOut.Read(p)
	return n, normalizeConPtyPipeError("Read", err)
}

// Write writes to the pseudo console input.
// Write may race with Close: if Close runs between RUnlock and WriteFile,
// the OS returns ERROR_INVALID_HANDLE/ERROR_BROKEN_PIPE, which
// normalizeConPtyPipeError converts to a clear "closed" message.
// Holding the lock during blocking I/O would risk deadlock with Close.
func (c *ConPty) Write(p []byte) (int, error) {
	c.stateMu.RLock()
	cmdIn := c.cmdIn
	c.stateMu.RUnlock()
	if cmdIn == nil {
		return 0, errors.New("Write called on closed pseudo console (nil handle)")
	}
	n, err := cmdIn.Write(p)
	return n, normalizeConPtyPipeError("Write", err)
}

// Resize changes the pseudo console dimensions.
func (c *ConPty) Resize(width, height int) error {
	if err := validateConPtyDimensions(width, height); err != nil {
		return err
	}
	// Resize is non-blocking, so keeping the read lock during syscall is
	// acceptable and avoids races with concurrent Close() state transitions.
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	if c.hpCon == 0 {
		return errors.New("Resize called on closed pseudo console")
	}
	coord := &_COORD{X: int16(width), Y: int16(height)}
	return resizePseudoConsole(c.hpCon, coord)
}

// Close terminates the process and releases resources.
// Closes the pseudo console first, then waits briefly for the process to
// exit gracefully before forcing termination with TerminateProcess.
// Safe to call multiple times; only the first call performs cleanup.
func (c *ConPty) Close() error {
	c.closeOnce.Do(func() {
		c.closeErr = c.doClose()
	})
	return c.closeErr
}

// doClose performs the actual resource cleanup.
// Order: pseudo console -> process wait/terminate -> process handles -> pipes.
func (c *ConPty) doClose() error {
	c.stateMu.Lock()
	hpCon := c.hpCon
	pi := c.pi
	cmdIn := c.cmdIn
	cmdOut := c.cmdOut
	c.hpCon = 0
	c.pi = nil
	c.cmdIn = nil
	c.cmdOut = nil
	c.stateMu.Unlock()

	if hpCon != 0 {
		closePseudoConsole(hpCon)
	}

	var firstErr error
	if pi != nil {
		// Wait briefly for the process to exit gracefully after pseudo console closure.
		// If it doesn't exit within gracePeriodMS, force-terminate.
		ret, waitErr := waitForSingleObjectFn(pi.Process, gracePeriodMS)
		waitRet := formatWaitResult(ret)
		if waitErr != nil {
			slog.Warn("[WARN-CONPTY] WaitForSingleObject failed",
				"pid", pi.ProcessId, "wait_ret", waitRet, "error", waitErr)
			if firstErr == nil {
				firstErr = fmt.Errorf("WaitForSingleObject failed during ConPTY close: %w", waitErr)
			}
		}
		// For WAIT_TIMEOUT and WAIT_FAILED we cannot trust that the child exited;
		// force termination to avoid leaking a zombie process.
		if ret != windows.WAIT_OBJECT_0 {
			if termErr := terminateProcessFn(pi.Process, 0); termErr != nil {
				slog.Warn("[WARN-CONPTY] TerminateProcess failed (zombie process risk)",
					"pid", pi.ProcessId, "wait_ret", waitRet, "error", termErr)
				if firstErr == nil {
					firstErr = fmt.Errorf("failed to terminate pseudo console process: %w", termErr)
				}
			} else {
				postTerminateRet, postTerminateWaitErr := waitForSingleObjectFn(pi.Process, terminateWaitMS)
				if postTerminateWaitErr != nil {
					slog.Warn("[WARN-CONPTY] WaitForSingleObject after TerminateProcess failed",
						"pid", pi.ProcessId,
						"wait_ret", formatWaitResult(postTerminateRet),
						"error", postTerminateWaitErr)
					if firstErr == nil {
						firstErr = fmt.Errorf("WaitForSingleObject after TerminateProcess failed during ConPTY close: %w", postTerminateWaitErr)
					}
				} else if postTerminateRet != windows.WAIT_OBJECT_0 {
					slog.Warn("[WARN-CONPTY] process did not report exited state after TerminateProcess",
						"pid", pi.ProcessId,
						"wait_ret", formatWaitResult(postTerminateRet))
				}
			}
		}
		closeHandles(pi.Process, pi.Thread)
	}

	for _, closer := range []*handleIO{cmdIn, cmdOut} {
		if closer != nil {
			if err := closer.Close(); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}

	return firstErr
}

func formatWaitResult(ret uint32) string {
	switch ret {
	case windows.WAIT_OBJECT_0:
		return "WAIT_OBJECT_0(0x0)"
	case windows.WAIT_ABANDONED:
		return "WAIT_ABANDONED(0x80)"
	case waitTimeoutResultCode:
		return "WAIT_TIMEOUT(0x102)"
	case windows.WAIT_FAILED:
		return "WAIT_FAILED(0xFFFFFFFF)"
	default:
		return fmt.Sprintf("0x%X", ret)
	}
}

func validateConPtyDimensions(width, height int) error {
	const maxConPtyDimension = 32767
	if width <= 0 || width > maxConPtyDimension || height <= 0 || height > maxConPtyDimension {
		return fmt.Errorf("ConPTY dimensions must be between 1 and %d: width=%d, height=%d", maxConPtyDimension, width, height)
	}
	return nil
}

func normalizeConPtyPipeError(operation string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, windows.ERROR_INVALID_HANDLE) ||
		errors.Is(err, windows.ERROR_BROKEN_PIPE) ||
		errors.Is(err, io.ErrClosedPipe) ||
		errors.Is(err, windows.ERROR_NO_DATA) {
		return fmt.Errorf("%s called on closed pseudo console: %w", operation, err)
	}
	return err
}

func normalizeWriteFileError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, windows.ERROR_BROKEN_PIPE) ||
		errors.Is(err, windows.ERROR_NO_DATA) ||
		errors.Is(err, windows.ERROR_INVALID_HANDLE) {
		return io.ErrClosedPipe
	}
	return err
}

func normalizeReadFileError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, windows.ERROR_BROKEN_PIPE) ||
		errors.Is(err, windows.ERROR_HANDLE_EOF) ||
		errors.Is(err, windows.ERROR_INVALID_HANDLE) ||
		errors.Is(err, windows.ERROR_NO_DATA) {
		return io.EOF
	}
	return err
}

// Pid returns the process ID.
func (c *ConPty) Pid() int {
	c.stateMu.RLock()
	pi := c.pi
	c.stateMu.RUnlock()
	if pi == nil {
		return 0
	}
	return int(pi.ProcessId)
}
