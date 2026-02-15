package ipc

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/Microsoft/go-winio"
)

const (
	defaultPipeDialTimeout = 3 * time.Second
	defaultPipeRWTimeout   = 15 * time.Second
	maxPipeResponseBytes   = 64 * 1024
)

// Send sends one request and waits for one response.
func Send(pipeName string, req TmuxRequest) (TmuxResponse, error) {
	if pipeName == "" {
		pipeName = DefaultPipeName()
	}

	dialTimeout := defaultPipeDialTimeout
	conn, err := winio.DialPipe(pipeName, &dialTimeout)
	if err != nil {
		return TmuxResponse{}, err
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(defaultPipeRWTimeout)); err != nil {
		return TmuxResponse{}, fmt.Errorf("set deadline: %w", err)
	}

	rawReq, err := encodeRequest(req)
	if err != nil {
		return TmuxResponse{}, err
	}

	if _, err := conn.Write(rawReq); err != nil {
		return TmuxResponse{}, err
	}
	if _, err := conn.Write([]byte{'\n'}); err != nil {
		return TmuxResponse{}, err
	}

	respRaw, err := readDelimitedFrame(bufio.NewReaderSize(conn, maxPipeResponseBytes+1), maxPipeResponseBytes)
	if err != nil {
		return TmuxResponse{}, err
	}

	resp, err := decodeResponse(respRaw)
	if err != nil {
		return TmuxResponse{}, fmt.Errorf("invalid response: %w", err)
	}
	return resp, nil
}

func readDelimitedFrame(reader *bufio.Reader, maxBytes int) ([]byte, error) {
	raw, err := reader.ReadSlice('\n')
	if errors.Is(err, bufio.ErrBufferFull) {
		return nil, fmt.Errorf("response exceeds %d bytes", maxBytes)
	}
	if errors.Is(err, io.EOF) {
		if len(raw) == 0 {
			return nil, io.EOF
		}
		return raw, nil
	}
	if err != nil {
		return nil, err
	}
	return raw, nil
}

// IsConnectionError returns true when the error indicates that the pipe
// server is absent or unreachable (dial/connect failures).
func IsConnectionError(err error) bool {
	if err == nil {
		return false
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return opErr.Op == "dial" || opErr.Op == "open"
	}
	return false
}
