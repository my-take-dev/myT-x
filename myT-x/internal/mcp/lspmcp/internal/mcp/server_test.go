package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"myT-x/internal/mcp/lspmcp/internal/jsonrpc"
)

// sendRequestAndReadResponse は行区切り JSON でリクエストを送信し、レスポンスを読み取る。
// clientToServer: テストが書き込み、サーバーが読み取る
// serverToClient: サーバーが書き込み、テストが読み取る
func sendRequestAndReadResponse(t *testing.T, clientToServer io.Writer, serverToClient io.Reader, req map[string]any) map[string]any {
	t.Helper()
	payload, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal request: %v", err)
	}
	if _, err := clientToServer.Write(append(payload, '\n')); err != nil {
		t.Fatalf("Write request: %v", err)
	}

	reader := bufio.NewReader(serverToClient)
	// 行区切り JSON（サーバーは最初のメッセージ形式に合わせる）
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("Read response: %v", err)
	}
	line = strings.TrimSpace(line)
	if line == "" {
		// Content-Length 形式の可能性
		payload, err := jsonrpc.ReadMessage(reader)
		if err != nil {
			t.Fatalf("ReadMessage: %v", err)
		}
		line = string(payload)
	}

	var resp map[string]any
	if err := json.Unmarshal([]byte(line), &resp); err != nil {
		t.Fatalf("Unmarshal response: %v", err)
	}
	return resp
}

func TestServerToolsCallUnknownTool(t *testing.T) {
	clientToServerR, clientToServerW := io.Pipe()
	serverToClientR, serverToClientW := io.Pipe()
	reg := NewRegistry(nil)
	srv := NewServer(Config{
		In:       clientToServerR,
		Out:      serverToClientW,
		Registry: reg,
	})

	ctx := t.Context()
	var wg sync.WaitGroup
	wg.Go(func() {
		if err := srv.Serve(ctx); err != nil {
			t.Logf("Serve error: %v", err)
		}
		if err := serverToClientW.Close(); err != nil {
			t.Logf("Close serverToClientW: %v", err)
		}
	})
	defer func() {
		if err := clientToServerW.Close(); err != nil {
			t.Logf("Close clientToServerW: %v", err)
		}
		wg.Wait()
	}()

	// initialize が先に必要
	initReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params":  map[string]any{},
	}
	initResp := sendRequestAndReadResponse(t, clientToServerW, serverToClientR, initReq)
	if initResp["result"] == nil {
		t.Fatalf("initialize failed: %v", initResp["error"])
	}

	// 存在しないツールで tools/call
	callReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "nonexistent_tool",
			"arguments": map[string]any{},
		},
	}
	callResp := sendRequestAndReadResponse(t, clientToServerW, serverToClientR, callReq)
	if callResp["error"] == nil {
		t.Fatalf("tools/call with unknown tool should return error, got: %v", callResp)
	}
	errObj, ok := callResp["error"].(map[string]any)
	if !ok {
		t.Fatalf("error field should be object")
	}
	if !strings.Contains(errObj["message"].(string), "Unknown tool") {
		t.Errorf("unexpected error message: %v", errObj["message"])
	}
}

func TestServerToolsCallInvalidParams(t *testing.T) {
	clientToServerR, clientToServerW := io.Pipe()
	serverToClientR, serverToClientW := io.Pipe()
	reg := NewRegistry(nil)
	srv := NewServer(Config{
		In:       clientToServerR,
		Out:      serverToClientW,
		Registry: reg,
	})

	ctx := t.Context()
	go func() {
		if err := srv.Serve(ctx); err != nil {
			t.Logf("Serve error: %v", err)
		}
		if err := serverToClientW.Close(); err != nil {
			t.Logf("Close serverToClientW: %v", err)
		}
	}()
	defer func() {
		if err := clientToServerW.Close(); err != nil {
			t.Logf("Close clientToServerW: %v", err)
		}
	}()

	initReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params":  map[string]any{},
	}
	sendRequestAndReadResponse(t, clientToServerW, serverToClientR, initReq)

	callReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params":  "invalid",
	}
	callResp := sendRequestAndReadResponse(t, clientToServerW, serverToClientR, callReq)
	if callResp["error"] == nil {
		t.Fatalf("tools/call with invalid params should return error")
	}
}

func TestServerUnknownMethod(t *testing.T) {
	clientToServerR, clientToServerW := io.Pipe()
	serverToClientR, serverToClientW := io.Pipe()
	srv := NewServer(Config{
		In:       clientToServerR,
		Out:      serverToClientW,
		Registry: NewRegistry(nil),
	})

	ctx := t.Context()
	go func() {
		if err := srv.Serve(ctx); err != nil {
			t.Logf("Serve error: %v", err)
		}
		if err := serverToClientW.Close(); err != nil {
			t.Logf("Close serverToClientW: %v", err)
		}
	}()
	defer func() {
		if err := clientToServerW.Close(); err != nil {
			t.Logf("Close clientToServerW: %v", err)
		}
	}()

	initReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params":  map[string]any{},
	}
	sendRequestAndReadResponse(t, clientToServerW, serverToClientR, initReq)

	unknownReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "unknown/method",
		"params":  map[string]any{},
	}
	resp := sendRequestAndReadResponse(t, clientToServerW, serverToClientR, unknownReq)
	if resp["error"] == nil {
		t.Fatalf("unknown method should return error")
	}
	errObj := resp["error"].(map[string]any)
	if errObj["code"].(float64) != -32601 {
		t.Errorf("expected -32601 Method not found, got %v", errObj["code"])
	}
}

func TestServerInitializeSuccess(t *testing.T) {
	clientToServerR, clientToServerW := io.Pipe()
	serverToClientR, serverToClientW := io.Pipe()
	srv := NewServer(Config{
		In:       clientToServerR,
		Out:      serverToClientW,
		Registry: NewRegistry(nil),
	})

	ctx := t.Context()
	go func() {
		if err := srv.Serve(ctx); err != nil {
			t.Logf("Serve error: %v", err)
		}
		if err := serverToClientW.Close(); err != nil {
			t.Logf("Close serverToClientW: %v", err)
		}
	}()
	defer func() {
		if err := clientToServerW.Close(); err != nil {
			t.Logf("Close clientToServerW: %v", err)
		}
	}()

	initReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params":  map[string]any{},
	}
	resp := sendRequestAndReadResponse(t, clientToServerW, serverToClientR, initReq)
	if resp["error"] != nil {
		t.Fatalf("initialize should succeed, got error: %v", resp["error"])
	}
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("result should be object, got %T", resp["result"])
	}
	if result["capabilities"] == nil {
		t.Error("result should have capabilities")
	}
	if result["serverInfo"] == nil {
		t.Error("result should have serverInfo")
	}
}

func TestServerToolsListSuccess(t *testing.T) {
	reg := NewRegistry([]Tool{
		{Name: "test_tool", Description: "test", Handler: func(ctx context.Context, args map[string]any) (any, error) { return "ok", nil }},
	})
	clientToServerR, clientToServerW := io.Pipe()
	serverToClientR, serverToClientW := io.Pipe()
	srv := NewServer(Config{In: clientToServerR, Out: serverToClientW, Registry: reg})

	ctx := t.Context()
	go func() {
		if err := srv.Serve(ctx); err != nil {
			t.Logf("Serve error: %v", err)
		}
		_ = serverToClientW.Close()
	}()
	defer clientToServerW.Close()

	sendRequestAndReadResponse(t, clientToServerW, serverToClientR, map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{},
	})

	resp := sendRequestAndReadResponse(t, clientToServerW, serverToClientR, map[string]any{
		"jsonrpc": "2.0", "id": 2, "method": "tools/list", "params": map[string]any{},
	})
	if resp["error"] != nil {
		t.Fatalf("tools/list should succeed: %v", resp["error"])
	}
	result := resp["result"].(map[string]any)
	tools, ok := result["tools"].([]any)
	if !ok || len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %v", result["tools"])
	}
	tool := tools[0].(map[string]any)
	if tool["name"] != "test_tool" {
		t.Errorf("unexpected tool name: %v", tool["name"])
	}
}

func TestServerToolsCallSuccess(t *testing.T) {
	reg := NewRegistry([]Tool{
		{
			Name:        "echo_tool",
			Description: "echo",
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return map[string]any{"echoed": args["value"]}, nil
			},
		},
	})
	clientToServerR, clientToServerW := io.Pipe()
	serverToClientR, serverToClientW := io.Pipe()
	srv := NewServer(Config{In: clientToServerR, Out: serverToClientW, Registry: reg})

	ctx := t.Context()
	go func() {
		if err := srv.Serve(ctx); err != nil {
			t.Logf("Serve error: %v", err)
		}
		_ = serverToClientW.Close()
	}()
	defer clientToServerW.Close()

	sendRequestAndReadResponse(t, clientToServerW, serverToClientR, map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{},
	})

	resp := sendRequestAndReadResponse(t, clientToServerW, serverToClientR, map[string]any{
		"jsonrpc": "2.0", "id": 2, "method": "tools/call",
		"params": map[string]any{"name": "echo_tool", "arguments": map[string]any{"value": "hello"}},
	})
	if resp["error"] != nil {
		t.Fatalf("tools/call should succeed: %v", resp["error"])
	}
	result := resp["result"].(map[string]any)
	if result["isError"] == true {
		t.Fatalf("handler should succeed, got isError: true")
	}
	if content, ok := result["content"].([]any); ok && len(content) > 0 {
		// content にテキストが含まれる
		_ = content
	}
}

func TestServerPingSuccess(t *testing.T) {
	clientToServerR, clientToServerW := io.Pipe()
	serverToClientR, serverToClientW := io.Pipe()
	srv := NewServer(Config{In: clientToServerR, Out: serverToClientW, Registry: NewRegistry(nil)})

	ctx := t.Context()
	go func() {
		if err := srv.Serve(ctx); err != nil {
			t.Logf("Serve error: %v", err)
		}
		_ = serverToClientW.Close()
	}()
	defer clientToServerW.Close()

	sendRequestAndReadResponse(t, clientToServerW, serverToClientR, map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{},
	})

	resp := sendRequestAndReadResponse(t, clientToServerW, serverToClientR, map[string]any{
		"jsonrpc": "2.0", "id": 2, "method": "ping", "params": map[string]any{},
	})
	if resp["error"] != nil {
		t.Fatalf("ping should succeed: %v", resp["error"])
	}
	if resp["result"] == nil {
		t.Error("ping should return result")
	}
}

func TestServerShutdownSuccess(t *testing.T) {
	clientToServerR, clientToServerW := io.Pipe()
	serverToClientR, serverToClientW := io.Pipe()
	srv := NewServer(Config{In: clientToServerR, Out: serverToClientW, Registry: NewRegistry(nil)})

	ctx := t.Context()
	go func() {
		if err := srv.Serve(ctx); err != nil {
			t.Logf("Serve error: %v", err)
		}
		_ = serverToClientW.Close()
	}()
	defer clientToServerW.Close()

	sendRequestAndReadResponse(t, clientToServerW, serverToClientR, map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{},
	})

	resp := sendRequestAndReadResponse(t, clientToServerW, serverToClientR, map[string]any{
		"jsonrpc": "2.0", "id": 2, "method": "shutdown", "params": map[string]any{},
	})
	if resp["error"] != nil {
		t.Fatalf("shutdown should succeed: %v", resp["error"])
	}
}

func TestServerToolsCallHandlerError(t *testing.T) {
	handlerErr := context.DeadlineExceeded
	reg := NewRegistry([]Tool{
		{
			Name:        "failing_tool",
			Description: "always fails",
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return nil, handlerErr
			},
		},
	})

	clientToServerR, clientToServerW := io.Pipe()
	serverToClientR, serverToClientW := io.Pipe()
	srv := NewServer(Config{
		In:       clientToServerR,
		Out:      serverToClientW,
		Registry: reg,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	go func() {
		if err := srv.Serve(ctx); err != nil {
			t.Logf("Serve error: %v", err)
		}
		if err := serverToClientW.Close(); err != nil {
			t.Logf("Close serverToClientW: %v", err)
		}
	}()
	defer func() {
		if err := clientToServerW.Close(); err != nil {
			t.Logf("Close clientToServerW: %v", err)
		}
	}()

	initReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params":  map[string]any{},
	}
	sendRequestAndReadResponse(t, clientToServerW, serverToClientR, initReq)

	callReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "failing_tool",
			"arguments": map[string]any{},
		},
	}
	resp := sendRequestAndReadResponse(t, clientToServerW, serverToClientR, callReq)
	if resp["error"] != nil {
		t.Fatalf("handler error returns result with isError, not JSON-RPC error: %v", resp["error"])
	}
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("result should be object")
	}
	if !result["isError"].(bool) {
		t.Errorf("handler error should set isError: true")
	}
}
