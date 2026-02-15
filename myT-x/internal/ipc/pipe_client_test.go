package ipc

import (
	"bufio"
	"io"
	"strings"
	"testing"
)

func TestReadDelimitedFrameWithinLimit(t *testing.T) {
	payload := `{"exit_code":0,"stdout":"ok\n"}` + "\n"
	reader := bufio.NewReaderSize(strings.NewReader(payload), maxPipeResponseBytes+1)

	raw, err := readDelimitedFrame(reader, maxPipeResponseBytes)
	if err != nil {
		t.Fatalf("readDelimitedFrame() error = %v", err)
	}
	if string(raw) != payload {
		t.Fatalf("readDelimitedFrame() = %q, want %q", string(raw), payload)
	}
}

func TestReadDelimitedFrameRejectsOversizedResponse(t *testing.T) {
	oversized := strings.Repeat("b", maxPipeResponseBytes+1) + "\n"
	reader := bufio.NewReaderSize(strings.NewReader(oversized), maxPipeResponseBytes+1)

	_, err := readDelimitedFrame(reader, maxPipeResponseBytes)
	if err == nil {
		t.Fatalf("readDelimitedFrame() expected size error")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("readDelimitedFrame() error = %q, want 'exceeds' message", err.Error())
	}
}

func TestReadDelimitedFrameReturnsEOFOnEmptyInput(t *testing.T) {
	reader := bufio.NewReaderSize(strings.NewReader(""), maxPipeResponseBytes+1)

	_, err := readDelimitedFrame(reader, maxPipeResponseBytes)
	if err == nil {
		t.Fatalf("readDelimitedFrame() expected EOF error")
	}
	if err != io.EOF {
		t.Fatalf("readDelimitedFrame() error = %v, want io.EOF", err)
	}
}

func TestReadDelimitedFrameAcceptsEOFWithPartialData(t *testing.T) {
	// Data without trailing newline should be returned on EOF.
	payload := `{"exit_code":0}`
	reader := bufio.NewReaderSize(strings.NewReader(payload), maxPipeResponseBytes+1)

	raw, err := readDelimitedFrame(reader, maxPipeResponseBytes)
	if err != nil {
		t.Fatalf("readDelimitedFrame() error = %v, want nil", err)
	}
	if string(raw) != payload {
		t.Fatalf("readDelimitedFrame() = %q, want %q", string(raw), payload)
	}
}
