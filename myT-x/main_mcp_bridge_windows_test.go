//go:build windows

package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

// stubConn implements net.Conn for testing purposes.
type stubConn struct {
	readBuf *bytes.Buffer
	writeTo *bytes.Buffer
	closed  bool
}

func newStubConn() *stubConn {
	return &stubConn{readBuf: &bytes.Buffer{}, writeTo: &bytes.Buffer{}}
}

func (c *stubConn) Read(b []byte) (int, error) {
	if c.closed {
		return 0, net.ErrClosed
	}
	return c.readBuf.Read(b)
}
func (c *stubConn) Write(b []byte) (int, error) {
	if c.closed {
		return 0, net.ErrClosed
	}
	return c.writeTo.Write(b)
}
func (c *stubConn) Close() error                     { c.closed = true; return nil }
func (c *stubConn) LocalAddr() net.Addr              { return nil }
func (c *stubConn) RemoteAddr() net.Addr             { return nil }
func (c *stubConn) SetDeadline(time.Time) error      { return nil }
func (c *stubConn) SetReadDeadline(time.Time) error  { return nil }
func (c *stubConn) SetWriteDeadline(time.Time) error { return nil }

func TestBridgeMCPStdio_RetriesUntilSuccess(t *testing.T) {
	t.Parallel()

	attempts := 0
	conn := newStubConn()
	// Simulate pipe not ready twice, then succeed.
	// Write EOF to readBuf so Bridge returns immediately after connect.
	dialFn := func(path string, timeout *time.Duration) (net.Conn, error) {
		attempts++
		if attempts <= 2 {
			return nil, errors.New("pipe not found")
		}
		return conn, nil
	}

	err := bridgeMCPStdio(context.Background(), `\\.\pipe\test-retry`, 5*time.Second, &bytes.Buffer{}, io.Discard, dialFn)
	// Bridge will get io.EOF from empty readBuf and return nil.
	if err != nil {
		t.Fatalf("bridgeMCPStdio error = %v, want nil", err)
	}
	if attempts != 3 {
		t.Fatalf("dial attempts = %d, want 3", attempts)
	}
}

func TestBridgeMCPStdio_ExhaustsRetryBudget(t *testing.T) {
	t.Parallel()

	attempts := 0
	dialFn := func(path string, timeout *time.Duration) (net.Conn, error) {
		attempts++
		return nil, errors.New("pipe not found")
	}

	err := bridgeMCPStdio(context.Background(), `\\.\pipe\test-exhaust`, 1*time.Second, &bytes.Buffer{}, io.Discard, dialFn)
	if err == nil {
		t.Fatal("bridgeMCPStdio should fail when all retries exhausted")
	}
	if !strings.Contains(err.Error(), "failed after") {
		t.Fatalf("error = %v, want 'failed after' message", err)
	}
	if !strings.Contains(err.Error(), "pipe not found") {
		t.Fatalf("error = %v, want wrapped last error", err)
	}
	if attempts < 2 {
		t.Fatalf("dial attempts = %d, want at least 2 retries within 1s budget", attempts)
	}
}

func TestBridgeMCPStdio_ContextCancelledDuringBackoff(t *testing.T) {
	t.Parallel()

	dialFn := func(path string, timeout *time.Duration) (net.Conn, error) {
		return nil, errors.New("pipe not found")
	}

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel after a short delay so the first backoff sleep is interrupted.
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := bridgeMCPStdio(ctx, `\\.\pipe\test-cancel`, 10*time.Second, &bytes.Buffer{}, io.Discard, dialFn)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestBridgeMCPStdio_ImmediateSuccess(t *testing.T) {
	t.Parallel()

	attempts := 0
	conn := newStubConn()
	dialFn := func(path string, timeout *time.Duration) (net.Conn, error) {
		attempts++
		return conn, nil
	}

	start := time.Now()
	err := bridgeMCPStdio(context.Background(), `\\.\pipe\test-immediate`, 5*time.Second, &bytes.Buffer{}, io.Discard, dialFn)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("bridgeMCPStdio error = %v, want nil", err)
	}
	if attempts != 1 {
		t.Fatalf("dial attempts = %d, want 1", attempts)
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("elapsed = %v, want fast return without backoff delay", elapsed)
	}
}

func TestBridgeMCPStdio_InvalidPipePath(t *testing.T) {
	t.Parallel()

	dialFn := func(path string, timeout *time.Duration) (net.Conn, error) {
		t.Fatal("dialFn should not be called for invalid pipe path")
		return nil, nil
	}
	err := bridgeMCPStdio(context.Background(), `not-a-pipe-path`, 3*time.Second, &bytes.Buffer{}, io.Discard, dialFn)
	if err == nil {
		t.Fatal("bridgeMCPStdio should fail for invalid pipe path")
	}
	if !strings.Contains(err.Error(), "invalid pipe path") {
		t.Fatalf("error = %v, want invalid pipe path error", err)
	}
}
