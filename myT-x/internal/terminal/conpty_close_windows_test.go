//go:build windows

package terminal

import (
	"errors"
	"io"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestConPtyCloseIdempotent(t *testing.T) {
	// Create a ConPty with nil handles to test Close idempotency
	// without requiring actual Windows pseudo console creation.
	cpty := &ConPty{
		// All handles are nil/zero — doClose will skip handle cleanup.
	}

	err1 := cpty.Close()
	err2 := cpty.Close()

	if err1 != nil {
		t.Fatalf("first Close() error = %v, want nil", err1)
	}
	if err2 != nil {
		t.Fatalf("second Close() error = %v, want nil", err2)
	}
	if err1 != err2 {
		t.Fatalf("Close() returned different errors: first=%v, second=%v", err1, err2)
	}
}

func TestConPtyCloseClearsPipeReferences(t *testing.T) {
	cpty := &ConPty{
		cmdIn:  &handleIO{},
		cmdOut: &handleIO{},
	}

	_ = cpty.Close()

	if cpty.cmdIn != nil || cpty.cmdOut != nil {
		t.Fatalf("Close() should clear pipe references, got cmdIn=%v cmdOut=%v",
			cpty.cmdIn, cpty.cmdOut)
	}
}

func TestConPtyReadWriteAfterCloseReturnErrors(t *testing.T) {
	cpty := &ConPty{}
	_ = cpty.Close()

	if _, err := cpty.Read(make([]byte, 1)); err == nil || !strings.Contains(err.Error(), "Read called on closed pseudo console (nil handle)") {
		t.Fatalf("Read() error = %v, want closed pseudo console error", err)
	}
	if _, err := cpty.Write([]byte("x")); err == nil || !strings.Contains(err.Error(), "Write called on closed pseudo console (nil handle)") {
		t.Fatalf("Write() error = %v, want closed pseudo console error", err)
	}
	if err := cpty.Resize(120, 40); err == nil || !strings.Contains(err.Error(), "Resize called on closed pseudo console") {
		t.Fatalf("Resize() error = %v, want closed pseudo console error", err)
	}
	if got := cpty.Pid(); got != 0 {
		t.Fatalf("Pid() after close = %d, want 0", got)
	}
}

func TestHandleIOCloseSkipsInvalidHandles(t *testing.T) {
	if err := (&handleIO{handle: 0}).Close(); err != nil {
		t.Fatalf("Close() with zero handle error = %v, want nil", err)
	}
	if err := (&handleIO{handle: windows.InvalidHandle}).Close(); err != nil {
		t.Fatalf("Close() with invalid handle error = %v, want nil", err)
	}
}

func TestHandleIOReadWriteInvalidHandles(t *testing.T) {
	if _, err := (&handleIO{handle: 0}).Read(make([]byte, 1)); !errors.Is(err, io.EOF) {
		t.Fatalf("Read() with zero handle error = %v, want io.EOF", err)
	}
	if _, err := (&handleIO{handle: windows.InvalidHandle}).Write([]byte("x")); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("Write() with invalid handle error = %v, want io.ErrClosedPipe", err)
	}
}

func TestNormalizeConPtyPipeErrorForClosedHandleCodes(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{name: "invalid handle", err: windows.ERROR_INVALID_HANDLE},
		{name: "broken pipe", err: windows.ERROR_BROKEN_PIPE},
		{name: "no data", err: windows.ERROR_NO_DATA},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := normalizeConPtyPipeError("Read", tc.err)
			if err == nil {
				t.Fatal("normalizeConPtyPipeError() returned nil")
			}
			if !strings.Contains(err.Error(), "Read called on closed pseudo console") {
				t.Fatalf("normalized error = %v, want closed pseudo console message", err)
			}
			if !errors.Is(err, tc.err) {
				t.Fatalf("normalized error = %v, want wrapped %v", err, tc.err)
			}
		})
	}
}

func TestNormalizeConPtyPipeErrorPassesThroughUnknownErrors(t *testing.T) {
	original := errors.New("custom failure")
	got := normalizeConPtyPipeError("Write", original)
	if !errors.Is(got, original) {
		t.Fatalf("normalizeConPtyPipeError() = %v, want original error", got)
	}
	if got.Error() != original.Error() {
		t.Fatalf("normalizeConPtyPipeError() message = %q, want %q", got.Error(), original.Error())
	}
}

func TestFormatWaitResult(t *testing.T) {
	tests := []struct {
		name string
		ret  uint32
		want string
	}{
		{"WAIT_OBJECT_0", windows.WAIT_OBJECT_0, "WAIT_OBJECT_0(0x0)"},
		{"WAIT_ABANDONED", windows.WAIT_ABANDONED, "WAIT_ABANDONED(0x80)"},
		{"WAIT_TIMEOUT", waitTimeoutResultCode, "WAIT_TIMEOUT(0x102)"},
		{"WAIT_FAILED", windows.WAIT_FAILED, "WAIT_FAILED(0xFFFFFFFF)"},
		{"unknown value", 0x42, "0x42"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatWaitResult(tt.ret); got != tt.want {
				t.Fatalf("formatWaitResult(%#x) = %q, want %q", tt.ret, got, tt.want)
			}
		})
	}
}

func TestConPtyResizeDimensionValidation(t *testing.T) {
	cpty := &ConPty{}
	tests := []struct {
		name         string
		width        int
		height       int
		wantContains string
	}{
		{
			name:         "negative width",
			width:        -1,
			height:       24,
			wantContains: "ConPTY dimensions must be between 1 and",
		},
		{
			name:         "negative height",
			width:        80,
			height:       -1,
			wantContains: "ConPTY dimensions must be between 1 and",
		},
		{
			name:         "zero width",
			width:        0,
			height:       24,
			wantContains: "ConPTY dimensions must be between 1 and",
		},
		{
			name:         "zero height",
			width:        80,
			height:       0,
			wantContains: "ConPTY dimensions must be between 1 and",
		},
		{
			name:         "width overflow",
			width:        32768,
			height:       24,
			wantContains: "ConPTY dimensions must be between 1 and",
		},
		{
			name:         "height overflow",
			width:        80,
			height:       32768,
			wantContains: "ConPTY dimensions must be between 1 and",
		},
		{
			name:         "max int16 passes validation then checks closed state",
			width:        32767,
			height:       32767,
			wantContains: "Resize called on closed pseudo console",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cpty.Resize(tt.width, tt.height)
			if err == nil {
				t.Fatalf("Resize(%d, %d) expected error", tt.width, tt.height)
			}
			if !strings.Contains(err.Error(), tt.wantContains) {
				t.Fatalf("Resize(%d, %d) error = %v, want substring %q", tt.width, tt.height, err, tt.wantContains)
			}
		})
	}
}

func TestNormalizeConPtyPipeErrorReturnsNilForNilError(t *testing.T) {
	if got := normalizeConPtyPipeError("Read", nil); got != nil {
		t.Fatalf("normalizeConPtyPipeError(nil) = %v, want nil", got)
	}
}

func TestNormalizeReadFileError(t *testing.T) {
	t.Run("returns nil for nil error", func(t *testing.T) {
		if got := normalizeReadFileError(nil); got != nil {
			t.Fatalf("normalizeReadFileError(nil) = %v, want nil", got)
		}
	})

	t.Run("maps broken pipe to EOF", func(t *testing.T) {
		if got := normalizeReadFileError(windows.ERROR_BROKEN_PIPE); !errors.Is(got, io.EOF) {
			t.Fatalf("normalizeReadFileError(ERROR_BROKEN_PIPE) = %v, want io.EOF", got)
		}
	})

	t.Run("maps handle EOF to EOF", func(t *testing.T) {
		if got := normalizeReadFileError(windows.ERROR_HANDLE_EOF); !errors.Is(got, io.EOF) {
			t.Fatalf("normalizeReadFileError(ERROR_HANDLE_EOF) = %v, want io.EOF", got)
		}
	})

	t.Run("maps invalid handle to EOF", func(t *testing.T) {
		if got := normalizeReadFileError(windows.ERROR_INVALID_HANDLE); !errors.Is(got, io.EOF) {
			t.Fatalf("normalizeReadFileError(ERROR_INVALID_HANDLE) = %v, want io.EOF", got)
		}
	})

	t.Run("maps no data to EOF", func(t *testing.T) {
		if got := normalizeReadFileError(windows.ERROR_NO_DATA); !errors.Is(got, io.EOF) {
			t.Fatalf("normalizeReadFileError(ERROR_NO_DATA) = %v, want io.EOF", got)
		}
	})

	t.Run("passes through unrelated errors", func(t *testing.T) {
		original := errors.New("custom read failure")
		if got := normalizeReadFileError(original); !errors.Is(got, original) {
			t.Fatalf("normalizeReadFileError(custom) = %v, want wrapped original", got)
		}
	})
}

func TestNormalizeWriteFileError(t *testing.T) {
	t.Run("returns nil for nil error", func(t *testing.T) {
		if got := normalizeWriteFileError(nil); got != nil {
			t.Fatalf("normalizeWriteFileError(nil) = %v, want nil", got)
		}
	})

	t.Run("maps closed-pipe style errors to io.ErrClosedPipe", func(t *testing.T) {
		for _, tc := range []error{
			windows.ERROR_BROKEN_PIPE,
			windows.ERROR_NO_DATA,
			windows.ERROR_INVALID_HANDLE,
		} {
			if got := normalizeWriteFileError(tc); !errors.Is(got, io.ErrClosedPipe) {
				t.Fatalf("normalizeWriteFileError(%v) = %v, want io.ErrClosedPipe", tc, got)
			}
		}
	})

	t.Run("passes through unrelated errors", func(t *testing.T) {
		original := errors.New("custom write failure")
		if got := normalizeWriteFileError(original); !errors.Is(got, original) {
			t.Fatalf("normalizeWriteFileError(custom) = %v, want wrapped original", got)
		}
	})
}

func TestDoCloseTerminatesProcessOnTimeout(t *testing.T) {
	origWait := waitForSingleObjectFn
	origTerminate := terminateProcessFn
	t.Cleanup(func() {
		waitForSingleObjectFn = origWait
		terminateProcessFn = origTerminate
	})

	waitCalls := 0
	terminateCalls := 0
	var waitTimeouts []uint32
	waitForSingleObjectFn = func(_ windows.Handle, timeout uint32) (uint32, error) {
		waitCalls++
		waitTimeouts = append(waitTimeouts, timeout)
		if waitCalls == 1 {
			return waitTimeoutResultCode, nil
		}
		return windows.WAIT_OBJECT_0, nil
	}
	terminateProcessFn = func(_ windows.Handle, _ uint32) error {
		terminateCalls++
		return nil
	}

	cpty := &ConPty{
		pi: &windows.ProcessInformation{
			Process:   windows.InvalidHandle,
			Thread:    windows.InvalidHandle,
			ProcessId: 42,
		},
	}
	if err := cpty.doClose(); err != nil {
		t.Fatalf("doClose() error = %v, want nil", err)
	}
	if waitCalls != 2 {
		t.Fatalf("wait call count = %d, want 2 (grace wait + post-terminate wait)", waitCalls)
	}
	if len(waitTimeouts) != 2 || waitTimeouts[0] != gracePeriodMS || waitTimeouts[1] != terminateWaitMS {
		t.Fatalf("wait timeouts = %v, want [%d %d]", waitTimeouts, gracePeriodMS, terminateWaitMS)
	}
	if terminateCalls != 1 {
		t.Fatalf("terminate call count = %d, want 1", terminateCalls)
	}
}

func TestDoCloseReturnsErrorWhenTerminateProcessFails(t *testing.T) {
	origWait := waitForSingleObjectFn
	origTerminate := terminateProcessFn
	t.Cleanup(func() {
		waitForSingleObjectFn = origWait
		terminateProcessFn = origTerminate
	})

	waitCalls := 0
	terminateCalls := 0
	waitForSingleObjectFn = func(_ windows.Handle, _ uint32) (uint32, error) {
		waitCalls++
		return waitTimeoutResultCode, nil
	}
	terminateProcessFn = func(_ windows.Handle, _ uint32) error {
		terminateCalls++
		return errors.New("terminate failed")
	}

	cpty := &ConPty{
		pi: &windows.ProcessInformation{
			Process:   windows.InvalidHandle,
			Thread:    windows.InvalidHandle,
			ProcessId: 99,
		},
	}

	err := cpty.doClose()
	if err == nil {
		t.Fatal("doClose() expected terminate failure error")
	}
	if !strings.Contains(err.Error(), "failed to terminate pseudo console process") {
		t.Fatalf("doClose() error = %v, want terminate failure context", err)
	}
	if waitCalls != 1 {
		t.Fatalf("wait call count = %d, want 1", waitCalls)
	}
	if terminateCalls != 1 {
		t.Fatalf("terminate call count = %d, want 1", terminateCalls)
	}
}

func TestDoCloseReturnsErrorWhenWaitForSingleObjectFails(t *testing.T) {
	origWait := waitForSingleObjectFn
	origTerminate := terminateProcessFn
	t.Cleanup(func() {
		waitForSingleObjectFn = origWait
		terminateProcessFn = origTerminate
	})

	waitCalls := 0
	terminateCalls := 0
	waitForSingleObjectFn = func(_ windows.Handle, _ uint32) (uint32, error) {
		waitCalls++
		if waitCalls == 1 {
			return windows.WAIT_FAILED, errors.New("wait failed")
		}
		return windows.WAIT_OBJECT_0, nil
	}
	terminateProcessFn = func(_ windows.Handle, _ uint32) error {
		terminateCalls++
		return nil
	}

	cpty := &ConPty{
		pi: &windows.ProcessInformation{
			Process:   windows.InvalidHandle,
			Thread:    windows.InvalidHandle,
			ProcessId: 55,
		},
	}

	err := cpty.doClose()
	if err == nil {
		t.Fatal("doClose() expected wait failure error")
	}
	if !strings.Contains(err.Error(), "WaitForSingleObject failed during ConPTY close") {
		t.Fatalf("doClose() error = %v, want WaitForSingleObject failure context", err)
	}
	if waitCalls != 2 {
		t.Fatalf("wait call count = %d, want 2", waitCalls)
	}
	if terminateCalls != 1 {
		t.Fatalf("terminate call count = %d, want 1", terminateCalls)
	}
}

func TestDoCloseReturnsErrorWhenPostTerminateWaitFails(t *testing.T) {
	origWait := waitForSingleObjectFn
	origTerminate := terminateProcessFn
	t.Cleanup(func() {
		waitForSingleObjectFn = origWait
		terminateProcessFn = origTerminate
	})

	waitCalls := 0
	terminateCalls := 0
	waitForSingleObjectFn = func(_ windows.Handle, _ uint32) (uint32, error) {
		waitCalls++
		if waitCalls == 1 {
			return waitTimeoutResultCode, nil
		}
		return windows.WAIT_FAILED, errors.New("post-terminate wait failed")
	}
	terminateProcessFn = func(_ windows.Handle, _ uint32) error {
		terminateCalls++
		return nil
	}

	cpty := &ConPty{
		pi: &windows.ProcessInformation{
			Process:   windows.InvalidHandle,
			Thread:    windows.InvalidHandle,
			ProcessId: 77,
		},
	}

	err := cpty.doClose()
	if err == nil {
		t.Fatal("doClose() expected post-terminate wait failure error")
	}
	if !strings.Contains(err.Error(), "WaitForSingleObject after TerminateProcess failed during ConPTY close") {
		t.Fatalf("doClose() error = %v, want post-terminate wait failure context", err)
	}
	if waitCalls != 2 {
		t.Fatalf("wait call count = %d, want 2", waitCalls)
	}
	if terminateCalls != 1 {
		t.Fatalf("terminate call count = %d, want 1", terminateCalls)
	}
}

func TestDoCloseSkipsTerminateWhenProcessAlreadyExited(t *testing.T) {
	origWait := waitForSingleObjectFn
	origTerminate := terminateProcessFn
	t.Cleanup(func() {
		waitForSingleObjectFn = origWait
		terminateProcessFn = origTerminate
	})

	waitCalls := 0
	terminateCalls := 0
	waitForSingleObjectFn = func(_ windows.Handle, _ uint32) (uint32, error) {
		waitCalls++
		return windows.WAIT_OBJECT_0, nil
	}
	terminateProcessFn = func(_ windows.Handle, _ uint32) error {
		terminateCalls++
		return nil
	}

	cpty := &ConPty{
		pi: &windows.ProcessInformation{
			Process:   windows.InvalidHandle,
			Thread:    windows.InvalidHandle,
			ProcessId: 7,
		},
	}
	if err := cpty.doClose(); err != nil {
		t.Fatalf("doClose() error = %v, want nil", err)
	}
	if waitCalls != 1 {
		t.Fatalf("wait call count = %d, want 1", waitCalls)
	}
	if terminateCalls != 0 {
		t.Fatalf("terminate call count = %d, want 0", terminateCalls)
	}
}
