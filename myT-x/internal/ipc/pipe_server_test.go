package ipc

import (
	"bufio"
	"io"
	"strings"
	"testing"
)

func TestReadRequestFrameWithinLimit(t *testing.T) {
	payload := `{"command":"list-sessions"}` + "\n"
	reader := bufio.NewReaderSize(strings.NewReader(payload), maxPipeRequestBytes+1)

	raw, err := readRequestFrame(reader)
	if err != nil {
		t.Fatalf("readRequestFrame() error = %v", err)
	}
	if string(raw) != payload {
		t.Fatalf("readRequestFrame() = %q, want %q", string(raw), payload)
	}
}

func TestReadRequestFrameRejectsOversizedRequest(t *testing.T) {
	oversized := strings.Repeat("a", maxPipeRequestBytes+1) + "\n"
	reader := bufio.NewReaderSize(strings.NewReader(oversized), maxPipeRequestBytes+1)

	if _, err := readRequestFrame(reader); err == nil {
		t.Fatalf("readRequestFrame() expected size error")
	}
}

func TestReadRequestFrameAcceptsEOFWithoutDelimiter(t *testing.T) {
	payload := `{"command":"has-session"}`
	reader := bufio.NewReaderSize(strings.NewReader(payload), maxPipeRequestBytes+1)

	raw, err := readRequestFrame(reader)
	if err != nil {
		t.Fatalf("readRequestFrame() error = %v", err)
	}
	if string(raw) != payload {
		t.Fatalf("readRequestFrame() = %q, want %q", string(raw), payload)
	}
}

func TestReadRequestFrameReturnsEOFOnEmptyInput(t *testing.T) {
	reader := bufio.NewReaderSize(strings.NewReader(""), maxPipeRequestBytes+1)

	_, err := readRequestFrame(reader)
	if err != io.EOF {
		t.Fatalf("readRequestFrame() error = %v, want io.EOF", err)
	}
}
