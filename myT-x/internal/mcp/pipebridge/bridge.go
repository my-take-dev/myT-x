package pipebridge

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
)

type copyDirection string

const (
	dirStdinToPipe  copyDirection = "stdin->pipe"
	dirPipeToStdout copyDirection = "pipe->stdout"
)

type copyResult struct {
	direction copyDirection
	err       error
}

// Bridge relays bytes bidirectionally between stdio and a pipe connection.
// The relay exits when either side closes, context is canceled, or a non-benign
// I/O error occurs.
func Bridge(ctx context.Context, in io.Reader, out io.Writer, conn io.ReadWriteCloser) error {
	if ctx == nil {
		return errors.New("context is required")
	}
	if in == nil {
		return errors.New("input reader is required")
	}
	if out == nil {
		return errors.New("output writer is required")
	}
	if conn == nil {
		return errors.New("pipe connection is required")
	}

	results := make(chan copyResult, 2)
	go relayCopy(results, dirStdinToPipe, conn, in)
	go relayCopy(results, dirPipeToStdout, out, conn)

	var (
		stdinDone  bool
		stdoutDone bool
		finalErr   error
	)

	var closeOnce sync.Once
	closeConn := func() {
		closeOnce.Do(func() {
			_ = conn.Close()
		})
	}

	for !(stdinDone && stdoutDone) {
		select {
		case <-ctx.Done():
			closeConn()
			if finalErr == nil {
				return ctx.Err()
			}
			return finalErr
		case result := <-results:
			switch result.direction {
			case dirStdinToPipe:
				stdinDone = true
				if result.err == nil {
					if closeWriter, ok := conn.(interface{ CloseWrite() error }); ok {
						if err := closeWriter.CloseWrite(); err != nil && !isBenignRelayError(err) && finalErr == nil {
							finalErr = fmt.Errorf("close pipe write: %w", err)
							closeConn()
						}
					}
				} else if !isBenignRelayError(result.err) {
					if finalErr == nil {
						finalErr = fmt.Errorf("%s: %w", result.direction, result.err)
					}
					closeConn()
				}
			case dirPipeToStdout:
				stdoutDone = true
				if result.err != nil && !isBenignRelayError(result.err) && finalErr == nil {
					finalErr = fmt.Errorf("%s: %w", result.direction, result.err)
				}
				closeConn()
			}
		}
	}

	return finalErr
}

func relayCopy(results chan<- copyResult, direction copyDirection, dst io.Writer, src io.Reader) {
	_, err := io.Copy(dst, src)
	results <- copyResult{direction: direction, err: err}
}

func isBenignRelayError(err error) bool {
	if err == nil {
		return true
	}
	if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) || errors.Is(err, os.ErrClosed) {
		return true
	}
	lower := strings.ToLower(err.Error())
	if strings.Contains(lower, "use of closed network connection") {
		return true
	}
	if strings.Contains(lower, "the pipe is being closed") {
		return true
	}
	if strings.Contains(lower, "broken pipe") {
		return true
	}
	return false
}
