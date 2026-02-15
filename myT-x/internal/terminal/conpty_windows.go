//go:build windows

package terminal

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// ErrConPtyUnsupported indicates ConPTY is not available on this Windows version.
var ErrConPtyUnsupported = errors.New("ConPty is not available on this version of Windows")

// handleIO wraps a Windows handle for I/O operations.
type handleIO struct {
	handle windows.Handle
}

func (h *handleIO) Read(p []byte) (int, error) {
	var numRead uint32
	err := windows.ReadFile(h.handle, p, &numRead, nil)
	return int(numRead), err
}

func (h *handleIO) Write(p []byte) (int, error) {
	var numWritten uint32
	err := windows.WriteFile(h.handle, p, &numWritten, nil)
	return int(numWritten), err
}

func (h *handleIO) Close() error {
	return windows.CloseHandle(h.handle)
}

// ConPty represents a Windows pseudo console.
type ConPty struct {
	hpCon     _HPCON
	pi        *windows.ProcessInformation
	ptyIn     *handleIO
	ptyOut    *handleIO
	cmdIn     *handleIO
	cmdOut    *handleIO
	closeOnce sync.Once
	closeErr  error
}

// IsConPtyAvailable checks if ConPTY is supported on this Windows version.
func IsConPtyAvailable() bool {
	return isConPtyAvailable()
}

// conPtyArgs holds configuration for ConPTY creation.
type conPtyArgs struct {
	coord   _COORD
	workDir string
	env     []string
}

// ConPtyOption is a functional option for ConPTY configuration.
type ConPtyOption func(*conPtyArgs)

// ConPtyDimensions sets the console dimensions.
func ConPtyDimensions(width, height int) ConPtyOption {
	return func(args *conPtyArgs) {
		args.coord.X = int16(width)
		args.coord.Y = int16(height)
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
		coord: _COORD{X: 80, Y: 40},
	}
	for _, opt := range options {
		opt(args)
	}

	ptyIn, cmdIn, cmdOut, ptyOut, err := createPtyPipes()
	if err != nil {
		return nil, err
	}

	hpCon, err := createPseudoConsole(&args.coord, ptyIn, ptyOut)
	if err != nil {
		closeHandles(ptyIn, ptyOut, cmdIn, cmdOut)
		return nil, err
	}

	pi, err := createConPtyProcess(commandLine, args, hpCon)
	if err != nil {
		closePseudoConsole(hpCon)
		closeHandles(ptyIn, ptyOut, cmdIn, cmdOut)
		return nil, err
	}

	return &ConPty{
		hpCon:  hpCon,
		pi:     pi,
		ptyIn:  &handleIO{handle: ptyIn},
		ptyOut: &handleIO{handle: ptyOut},
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
		_ = windows.CloseHandle(ptyIn)
		_ = windows.CloseHandle(cmdIn)
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
		_ = windows.CloseHandle(h)
	}
}

type startupInfoEx struct {
	startupInfo   windows.StartupInfo
	attributeList []byte
}

func getStartupInfoExForPTY(hpCon _HPCON) (*startupInfoEx, error) {
	siEx := &startupInfoEx{}
	// STARTUPINFOEXW = STARTUPINFOW + lpAttributeList pointer
	siEx.startupInfo.Cb = uint32(unsafe.Sizeof(windows.StartupInfo{}) + unsafe.Sizeof((*byte)(nil)))
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
	if err != nil {
		return nil, fmt.Errorf("CreateProcess failed: %w", err)
	}

	return &pi, nil
}

// Read reads from the pseudo console output.
func (c *ConPty) Read(p []byte) (int, error) {
	return c.cmdOut.Read(p)
}

// Write writes to the pseudo console input.
func (c *ConPty) Write(p []byte) (int, error) {
	return c.cmdIn.Write(p)
}

// Resize changes the pseudo console dimensions.
func (c *ConPty) Resize(width, height int) error {
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
	if c.hpCon != 0 {
		closePseudoConsole(c.hpCon)
	}

	if c.pi != nil {
		// Wait briefly for the process to exit gracefully after pseudo console closure.
		// If it doesn't exit within 500ms, force-terminate.
		const gracePeriod = 500 // milliseconds — may need tuning for heavy shells (PowerShell etc.)
		ret, waitErr := windows.WaitForSingleObject(c.pi.Process, gracePeriod)
		if waitErr != nil {
			slog.Debug("[DEBUG-CONPTY] WaitForSingleObject failed",
				"pid", c.pi.ProcessId, "error", waitErr)
		}
		if ret != windows.WAIT_OBJECT_0 {
			if termErr := windows.TerminateProcess(c.pi.Process, 0); termErr != nil {
				slog.Debug("[DEBUG-CONPTY] TerminateProcess failed",
					"pid", c.pi.ProcessId, "error", termErr)
			}
		}
		closeHandles(c.pi.Process, c.pi.Thread)
	}

	var firstErr error
	if c.ptyIn != nil {
		if err := c.ptyIn.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if c.ptyOut != nil {
		if err := c.ptyOut.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if c.cmdIn != nil {
		if err := c.cmdIn.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if c.cmdOut != nil {
		if err := c.cmdOut.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

// Pid returns the process ID.
func (c *ConPty) Pid() int {
	if c.pi == nil {
		return 0
	}
	return int(c.pi.ProcessId)
}
