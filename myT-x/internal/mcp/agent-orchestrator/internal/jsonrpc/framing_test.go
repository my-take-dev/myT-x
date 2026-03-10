package jsonrpc

import (
	"bufio"
	"errors"
	"strconv"
	"strings"
	"testing"
)

func TestReadMessageWithFramingRejectsOversizedContentLength(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("Content-Length: " + strconv.FormatInt(MaxFrameBytes+1, 10) + "\r\n\r\n"))

	_, _, err := ReadMessageWithFraming(reader)
	if !errors.Is(err, ErrFrameTooLarge) {
		t.Fatalf("expected ErrFrameTooLarge, got %v", err)
	}
}

func TestReadMessageWithFramingRejectsOversizedLineJSON(t *testing.T) {
	payload := "{" + strings.Repeat("a", int(MaxFrameBytes)) + "\n"
	reader := bufio.NewReader(strings.NewReader(payload))

	_, _, err := ReadMessageWithFraming(reader)
	if !errors.Is(err, ErrFrameTooLarge) {
		t.Fatalf("expected ErrFrameTooLarge, got %v", err)
	}
}
