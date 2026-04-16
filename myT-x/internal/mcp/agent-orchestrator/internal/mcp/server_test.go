package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"
	"time"

	"myT-x/internal/mcp/agent-orchestrator/domain"
)

func TestHandleToolsCallReturnsClassifiedToolErrors(t *testing.T) {
	registry := MustNewRegistry([]Tool{
		{
			Name: "fail",
			Handler: func(context.Context, map[string]any) (any, error) {
				return nil, domain.NewOrchestratorError(
					domain.ErrCodeAccessDenied,
					"access denied",
					"caller does not match agent_name",
					errors.New("boom"),
				)
			},
		},
	})

	server := NewServer(Config{Registry: registry})
	result, rpcErr := server.handleToolsCall(context.Background(), json.RawMessage(`{"name":"fail","arguments":{}}`))
	if rpcErr != nil {
		t.Fatalf("handleToolsCall rpcErr = %v, want nil", rpcErr)
	}

	payload, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result type = %T, want map[string]any", result)
	}
	content, ok := payload["content"].([]map[string]any)
	if !ok {
		rawContent, ok := payload["content"].([]any)
		if !ok || len(rawContent) != 1 {
			t.Fatalf("content type = %T, want one text payload", payload["content"])
		}
		entry, ok := rawContent[0].(map[string]any)
		if !ok {
			t.Fatalf("content[0] type = %T, want map[string]any", rawContent[0])
		}
		content = []map[string]any{entry}
	}
	if text := content[0]["text"]; text != "access denied" {
		t.Fatalf("content text = %v, want access denied", text)
	}
	if payload["isError"] != true {
		t.Fatalf("isError = %v, want true", payload["isError"])
	}
	structured, ok := payload["structuredContent"].(map[string]any)
	if !ok {
		t.Fatalf("structuredContent = %T, want map[string]any", payload["structuredContent"])
	}
	errPayload, ok := structured["error"].(map[string]any)
	if !ok {
		t.Fatalf("structuredContent.error = %T, want map[string]any", structured["error"])
	}
	if errPayload["code"] != string(domain.ErrCodeAccessDenied) {
		t.Fatalf("error code = %v, want %q", errPayload["code"], domain.ErrCodeAccessDenied)
	}
	if errPayload["reason"] != "caller does not match agent_name" {
		t.Fatalf("error reason = %v, want caller mismatch reason", errPayload["reason"])
	}
}

func TestHandleToolsCallClassifiesValidationFallbackErrors(t *testing.T) {
	registry := MustNewRegistry([]Tool{
		{
			Name: "fail",
			Handler: func(context.Context, map[string]any) (any, error) {
				return nil, errors.New("task_id is required")
			},
		},
	})

	server := NewServer(Config{Registry: registry})
	result, rpcErr := server.handleToolsCall(context.Background(), json.RawMessage(`{"name":"fail","arguments":{}}`))
	if rpcErr != nil {
		t.Fatalf("handleToolsCall rpcErr = %v, want nil", rpcErr)
	}

	payload, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result type = %T, want map[string]any", result)
	}
	structured, ok := payload["structuredContent"].(map[string]any)
	if !ok {
		t.Fatalf("structuredContent = %T, want map[string]any", payload["structuredContent"])
	}
	errPayload, ok := structured["error"].(map[string]any)
	if !ok {
		t.Fatalf("structuredContent.error = %T, want map[string]any", structured["error"])
	}
	if errPayload["code"] != string(domain.ErrCodeValidation) {
		t.Fatalf("error code = %v, want %q", errPayload["code"], domain.ErrCodeValidation)
	}
}

func TestHandleToolsCallHoistsStructuredIsError(t *testing.T) {
	registry := MustNewRegistry([]Tool{
		{
			Name: "soft-fail",
			Handler: func(context.Context, map[string]any) (any, error) {
				return map[string]any{
					"isError": true,
					"status":  "degraded",
				}, nil
			},
		},
	})

	server := NewServer(Config{Registry: registry})
	result, rpcErr := server.handleToolsCall(context.Background(), json.RawMessage(`{"name":"soft-fail","arguments":{}}`))
	if rpcErr != nil {
		t.Fatalf("handleToolsCall rpcErr = %v, want nil", rpcErr)
	}

	payload, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result type = %T, want map[string]any", result)
	}
	if payload["isError"] != true {
		t.Fatalf("isError = %v, want true", payload["isError"])
	}
}

func TestHandleShutdownSetsShutdownFlag(t *testing.T) {
	server := NewServer(Config{})
	if server.shutdown.Load() {
		t.Fatal("shutdown flag should be false before shutdown")
	}

	if _, rpcErr := server.handleShutdown(context.Background(), nil); rpcErr != nil {
		t.Fatalf("handleShutdown rpcErr = %v, want nil", rpcErr)
	}
	if !server.shutdown.Load() {
		t.Fatal("shutdown flag should be true after shutdown")
	}
}

func TestShutdownUnblocksServe(t *testing.T) {
	reader, writer := io.Pipe()
	t.Cleanup(func() {
		_ = writer.Close()
	})

	server := NewServer(Config{In: reader, Out: io.Discard})
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Serve(context.Background())
	}()

	server.Shutdown()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Serve() error = %v, want nil", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Serve did not return after shutdown")
	}
}

func TestHandleInitializeReturnsConfiguredProtocolVersion(t *testing.T) {
	server := NewServer(Config{
		Name:            "agent-orchestrator",
		Version:         "1.2.3",
		ProtocolVersion: "2025-02-14",
	})

	result, rpcErr := server.handleInitialize(context.Background(), json.RawMessage(`{"protocolVersion":"2024-11-05"}`))
	if rpcErr != nil {
		t.Fatalf("handleInitialize rpcErr = %v, want nil", rpcErr)
	}

	payload, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result type = %T, want map[string]any", result)
	}
	if payload["protocolVersion"] != "2025-02-14" {
		t.Fatalf("protocolVersion = %v, want %q", payload["protocolVersion"], "2025-02-14")
	}
}

func TestHandleInitializeRejectsInvalidParams(t *testing.T) {
	server := NewServer(Config{})

	_, rpcErr := server.handleInitialize(context.Background(), json.RawMessage(`{"protocolVersion":1}`))
	if rpcErr == nil {
		t.Fatal("handleInitialize rpcErr = nil, want invalid params error")
	}
	if rpcErr.Code != -32602 {
		t.Fatalf("rpcErr.Code = %d, want -32602", rpcErr.Code)
	}
}

func TestServeRejectsStructuredRequestID(t *testing.T) {
	input := bytes.NewBufferString("{\"jsonrpc\":\"2.0\",\"id\":{},\"method\":\"ping\"}\n")
	var output bytes.Buffer
	server := NewServer(Config{In: input, Out: &output})

	if err := server.Serve(context.Background()); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}

	var response map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(output.Bytes()), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if response["id"] != nil {
		t.Fatalf("response id = %#v, want nil", response["id"])
	}
	errObj, ok := response["error"].(map[string]any)
	if !ok {
		t.Fatalf("error payload type = %T, want map[string]any", response["error"])
	}
	if errObj["code"] != float64(-32600) {
		t.Fatalf("error code = %v, want -32600", errObj["code"])
	}
}

func TestServePreservesNullRequestIDInResponse(t *testing.T) {
	input := bytes.NewBufferString("{\"jsonrpc\":\"2.0\",\"id\":null,\"method\":\"ping\"}\n")
	var output bytes.Buffer
	server := NewServer(Config{In: input, Out: &output})

	if err := server.Serve(context.Background()); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}

	var response struct {
		ID json.RawMessage `json:"id"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(output.Bytes()), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got := string(response.ID); got != "null" {
		t.Fatalf("response id = %s, want null", got)
	}
}

func TestServePreservesRawRequestIDInResponse(t *testing.T) {
	input := bytes.NewBufferString("{\"jsonrpc\":\"2.0\",\"id\":1e3,\"method\":\"ping\"}\n")
	var output bytes.Buffer
	server := NewServer(Config{In: input, Out: &output})

	if err := server.Serve(context.Background()); err != nil {
		t.Fatalf("Serve() error = %v", err)
	}

	var response struct {
		ID json.RawMessage `json:"id"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(output.Bytes()), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got := string(response.ID); got != "1e3" {
		t.Fatalf("response id = %s, want 1e3", got)
	}
}
