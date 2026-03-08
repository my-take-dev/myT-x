package pipebridge

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) {
	return 0, errors.New("read failed")
}

func TestBridgeBidirectionalRelay(t *testing.T) {
	bridgeConn, peerConn := net.Pipe()
	defer peerConn.Close()

	input := bytes.NewBufferString("ping\n")
	var output bytes.Buffer

	serverErr := make(chan error, 1)
	go func() {
		reader := bufio.NewReader(peerConn)
		line, err := reader.ReadString('\n')
		if err != nil {
			serverErr <- err
			return
		}
		if line != "ping\n" {
			serverErr <- errors.New("unexpected request payload")
			return
		}
		if _, err := peerConn.Write([]byte("pong\n")); err != nil {
			serverErr <- err
			return
		}
		if err := peerConn.Close(); err != nil {
			serverErr <- err
			return
		}
		serverErr <- nil
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := Bridge(ctx, input, &output, bridgeConn); err != nil {
		t.Fatalf("Bridge returned error: %v", err)
	}
	if err := <-serverErr; err != nil {
		t.Fatalf("server goroutine failed: %v", err)
	}
	if got := output.String(); got != "pong\n" {
		t.Fatalf("unexpected output: %q", got)
	}
}

func TestBridgeStdinReadError(t *testing.T) {
	bridgeConn, peerConn := net.Pipe()
	defer peerConn.Close()
	defer bridgeConn.Close()

	var output bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := Bridge(ctx, errorReader{}, &output, bridgeConn)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "stdin->pipe") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBridgeContextCancel(t *testing.T) {
	bridgeConn, peerConn := net.Pipe()
	defer peerConn.Close()

	stdinReader, _ := io.Pipe()
	var output bytes.Buffer

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := Bridge(ctx, stdinReader, &output, bridgeConn)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}
