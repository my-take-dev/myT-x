package pipebridge

import (
	"bytes"
	"context"
	"testing"
)

func TestWriteAndReadCallerPaneHandshake(t *testing.T) {
	var stream bytes.Buffer
	if err := WriteCallerPaneHandshake(&stream, "%42"); err != nil {
		t.Fatalf("WriteCallerPaneHandshake: %v", err)
	}
	if _, err := stream.WriteString("{\"jsonrpc\":\"2.0\"}\n"); err != nil {
		t.Fatalf("append payload: %v", err)
	}

	reader, paneID, err := ReadCallerPaneHandshake(&stream)
	if err != nil {
		t.Fatalf("ReadCallerPaneHandshake: %v", err)
	}
	if paneID != "%42" {
		t.Fatalf("paneID = %q, want %%42", paneID)
	}
	payload, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("ReadString: %v", err)
	}
	if payload != "{\"jsonrpc\":\"2.0\"}\n" {
		t.Fatalf("payload = %q", payload)
	}
}

func TestReadCallerPaneHandshakeWithoutHeaderPreservesPayload(t *testing.T) {
	stream := bytes.NewBufferString("{\"jsonrpc\":\"2.0\"}\n")

	reader, paneID, err := ReadCallerPaneHandshake(stream)
	if err != nil {
		t.Fatalf("ReadCallerPaneHandshake: %v", err)
	}
	if paneID != "" {
		t.Fatalf("paneID = %q, want empty", paneID)
	}
	payload, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("ReadString: %v", err)
	}
	if payload != "{\"jsonrpc\":\"2.0\"}\n" {
		t.Fatalf("payload = %q", payload)
	}
}

func TestReadCallerPaneHandshakeRejectsInvalidPaneID(t *testing.T) {
	stream := bytes.NewBufferString("MYTX_CALLER_PANE invalid\n")
	if _, _, err := ReadCallerPaneHandshake(stream); err == nil {
		t.Fatal("ReadCallerPaneHandshake error = nil, want invalid pane rejection")
	}
}

func TestContextWithCallerPaneIDIgnoresInvalidPaneID(t *testing.T) {
	ctx := ContextWithCallerPaneID(context.Background(), "invalid")
	if got := CallerPaneIDFromContext(ctx); got != "" {
		t.Fatalf("CallerPaneIDFromContext = %q, want empty", got)
	}
}
