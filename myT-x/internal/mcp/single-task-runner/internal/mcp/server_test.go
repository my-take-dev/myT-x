package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestHandleToolsCallSanitizesToolErrors(t *testing.T) {
	registry := MustNewRegistry([]Tool{
		{
			Name: "fail",
			Handler: func(context.Context, map[string]any) (any, error) {
				return nil, errors.New("boom")
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
	if text := content[0]["text"]; text != "tool execution failed" {
		t.Fatalf("content text = %v, want sanitized tool error", text)
	}
	if payload["isError"] != true {
		t.Fatalf("isError = %v, want true", payload["isError"])
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

func TestHandleInitializeReturnsConfiguredServerInfo(t *testing.T) {
	server := NewServer(Config{
		Name:    "custom-runner",
		Version: "1.2.3",
	})

	result, rpcErr := server.handleInitialize(context.Background(), json.RawMessage(`{"protocolVersion":"2024-11-05"}`))
	if rpcErr != nil {
		t.Fatalf("handleInitialize rpcErr = %v, want nil", rpcErr)
	}

	payload, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result type = %T, want map[string]any", result)
	}
	if payload["protocolVersion"] != "2024-11-05" {
		t.Fatalf("protocolVersion = %v, want %q", payload["protocolVersion"], "2024-11-05")
	}
	serverInfo, ok := payload["serverInfo"].(map[string]any)
	if !ok {
		t.Fatalf("serverInfo type = %T, want map[string]any", payload["serverInfo"])
	}
	if serverInfo["name"] != "custom-runner" || serverInfo["version"] != "1.2.3" {
		t.Fatalf("serverInfo = %#v, want configured name/version", serverInfo)
	}
}

func TestHandleToolsListReturnsRegisteredTools(t *testing.T) {
	registry := MustNewRegistry([]Tool{
		{Name: "beta", Description: "second"},
		{Name: "alpha", Description: "first"},
	})
	server := NewServer(Config{Registry: registry})

	result, rpcErr := server.handleToolsList(context.Background(), nil)
	if rpcErr != nil {
		t.Fatalf("handleToolsList rpcErr = %v, want nil", rpcErr)
	}

	payload, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result type = %T, want map[string]any", result)
	}
	tools, ok := payload["tools"].([]map[string]any)
	if !ok {
		rawTools, ok := payload["tools"].([]any)
		if !ok {
			t.Fatalf("tools type = %T, want tool list", payload["tools"])
		}
		tools = make([]map[string]any, 0, len(rawTools))
		for _, item := range rawTools {
			tool, ok := item.(map[string]any)
			if !ok {
				t.Fatalf("tool type = %T, want map[string]any", item)
			}
			tools = append(tools, tool)
		}
	}
	if len(tools) != 2 {
		t.Fatalf("tool count = %d, want 2", len(tools))
	}
	if tools[0]["name"] != "alpha" || tools[1]["name"] != "beta" {
		t.Fatalf("tool order = %#v, want alpha then beta", tools)
	}
}

func TestHandleRequestReturnsMethodNotFoundForUnknownMethod(t *testing.T) {
	server := NewServer(Config{})

	_, rpcErr := server.handleRequest(context.Background(), "unknown/method", nil)
	if rpcErr == nil {
		t.Fatal("handleRequest rpcErr = nil, want method not found error")
	}
	if rpcErr.Code != -32601 {
		t.Fatalf("rpcErr.Code = %d, want -32601", rpcErr.Code)
	}
}

func TestHandleRequestRejectsRequestsAfterShutdown(t *testing.T) {
	server := NewServer(Config{})
	server.shutdown.Store(true)

	_, rpcErr := server.handleRequest(context.Background(), "tools/list", nil)
	if rpcErr == nil {
		t.Fatal("handleRequest rpcErr = nil, want shutdown rejection")
	}
	if rpcErr.Code != -32000 {
		t.Fatalf("rpcErr.Code = %d, want -32000", rpcErr.Code)
	}
	if rpcErr.Message != "Server is shutting down" {
		t.Fatalf("rpcErr.Message = %q, want shutdown rejection message", rpcErr.Message)
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
