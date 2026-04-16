package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"myT-x/internal/mcp/single-task-runner/internal/jsonrpc"
)

const defaultProtocolVersion = "2024-11-05"

// Config configures the MCP server transport and metadata.
type Config struct {
	Name            string
	Version         string
	ProtocolVersion string
	In              io.Reader
	Out             io.Writer
	Logger          *log.Logger
	Registry        *Registry
}

// Server serves MCP requests over stdio.
type Server struct {
	cfg Config

	reader   *bufio.Reader
	writer   io.Writer
	mode     jsonrpc.Framing
	handlers map[string]requestHandler

	wmu sync.Mutex

	shutdown atomic.Bool
}

type requestHandler func(context.Context, json.RawMessage) (any, *jsonrpc.Error)

// NewServer constructs a server with sensible defaults.
func NewServer(cfg Config) *Server {
	if cfg.Name == "" {
		cfg.Name = "single-task-runner"
	}
	if cfg.Version == "" {
		cfg.Version = "0.1.0"
	}
	if cfg.ProtocolVersion == "" {
		cfg.ProtocolVersion = defaultProtocolVersion
	}
	if cfg.In == nil {
		cfg.In = os.Stdin
	}
	if cfg.Out == nil {
		cfg.Out = os.Stdout
	}
	if cfg.Logger == nil {
		cfg.Logger = log.New(io.Discard, "", 0)
	}
	if cfg.Registry == nil {
		cfg.Registry = MustNewRegistry(nil)
	}

	server := &Server{
		cfg:    cfg,
		reader: bufio.NewReader(cfg.In),
		writer: cfg.Out,
		mode:   jsonrpc.FramingUnknown,
	}
	server.handlers = map[string]requestHandler{
		"initialize": server.handleInitialize,
		"tools/list": server.handleToolsList,
		"tools/call": server.handleToolsCall,
		"ping":       server.handlePing,
		"shutdown":   server.handleShutdown,
	}
	return server
}

// Serve processes MCP messages until EOF or context cancellation.
func (s *Server) Serve(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if s.shutdown.Load() {
			return nil
		}

		payload, mode, err := jsonrpc.ReadMessageWithFraming(s.reader)
		if err != nil {
			if errors.Is(err, io.EOF) || s.shutdown.Load() {
				return nil
			}
			if errors.Is(err, jsonrpc.ErrFrameTooLarge) {
				s.logf("reject oversized frame: %v", err)
				if writeErr := s.writeError(nil, -32700, "Parse error", nil); writeErr != nil {
					return writeErr
				}
				continue
			}
			return err
		}
		if s.mode == jsonrpc.FramingUnknown && mode != jsonrpc.FramingUnknown {
			s.mode = mode
		}

		var msg jsonrpc.Message
		if err := json.Unmarshal(payload, &msg); err != nil {
			s.logf("parse request: %v", err)
			if writeErr := s.writeError(nil, -32700, "Parse error", nil); writeErr != nil {
				return writeErr
			}
			continue
		}

		if msg.IsNotification() {
			if msg.Method == "notifications/initialized" || msg.Method == "exit" {
				if msg.Method == "exit" {
					return nil
				}
				continue
			}
			s.logf("ignoring unsupported notification: %s", msg.Method)
			continue
		}

		if !msg.IsRequest() {
			if msg.Method != "" && !msg.IsNotification() {
				s.logf("reject invalid request object")
				if writeErr := s.writeError(nil, -32600, "Invalid Request", nil); writeErr != nil {
					return writeErr
				}
				continue
			}
			s.logf("ignoring unknown message type")
			continue
		}

		result, rpcErr := s.handleRequest(ctx, msg.Method, msg.Params)
		if rpcErr != nil {
			if err := s.writeError(msg.ID, rpcErr.Code, rpcErr.Message, rpcErr.Data); err != nil {
				return err
			}
			continue
		}
		if err := s.writeResult(msg.ID, result); err != nil {
			return err
		}
	}
}

// Shutdown signals the server to stop processing. It sets the shutdown
// flag and, if the underlying reader implements io.Closer, closes it to
// unblock the Serve loop's blocking read.
func (s *Server) Shutdown() {
	s.shutdown.Store(true)
	if closer, ok := s.cfg.In.(io.Closer); ok {
		if err := closer.Close(); err != nil {
			s.logf("shutdown: failed to close input reader: %v", err)
		}
	}
}

func (s *Server) handleRequest(ctx context.Context, method string, paramsRaw json.RawMessage) (any, *jsonrpc.Error) {
	if s.shutdown.Load() && method != "shutdown" {
		return nil, &jsonrpc.Error{
			Code:    -32000,
			Message: "Server is shutting down",
		}
	}

	handler, ok := s.handlers[method]
	if ok {
		return handler(ctx, paramsRaw)
	}

	return nil, &jsonrpc.Error{
		Code:    -32601,
		Message: fmt.Sprintf("Method not found: %s", method),
	}
}

func (s *Server) handleInitialize(_ context.Context, paramsRaw json.RawMessage) (any, *jsonrpc.Error) {
	var params struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	if err := json.Unmarshal(paramsRaw, &params); err != nil && len(paramsRaw) > 0 {
		s.logf("parse initialize params: %v", err)
		return nil, &jsonrpc.Error{
			Code:    -32602, // Invalid params
			Message: fmt.Sprintf("invalid initialize params: %v", err),
		}
	}

	if params.ProtocolVersion != "" && params.ProtocolVersion != s.cfg.ProtocolVersion {
		s.logf("client requested unsupported protocol version %q, responding with %q",
			params.ProtocolVersion, s.cfg.ProtocolVersion)
	}

	return map[string]any{
		"protocolVersion": s.cfg.ProtocolVersion,
		"capabilities": map[string]any{
			"tools": map[string]any{
				"listChanged": false,
			},
		},
		"serverInfo": map[string]any{
			"name":    s.cfg.Name,
			"version": s.cfg.Version,
		},
	}, nil
}

func (s *Server) handleToolsList(_ context.Context, _ json.RawMessage) (any, *jsonrpc.Error) {
	tools := s.cfg.Registry.List()
	result := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		inputSchema := tool.InputSchema
		if inputSchema == nil {
			inputSchema = map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}
		}
		result = append(result, map[string]any{
			"name":        tool.Name,
			"description": tool.Description,
			"inputSchema": inputSchema,
		})
	}
	return map[string]any{"tools": result}, nil
}

func (s *Server) handleToolsCall(ctx context.Context, paramsRaw json.RawMessage) (any, *jsonrpc.Error) {
	var params struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.Unmarshal(paramsRaw, &params); err != nil {
		s.logf("parse tools/call params: %v", err)
		return nil, &jsonrpc.Error{Code: -32602, Message: "Invalid params"}
	}
	if params.Name == "" {
		return nil, &jsonrpc.Error{Code: -32602, Message: "Invalid params"}
	}
	if params.Arguments == nil {
		params.Arguments = map[string]any{}
	}
	s.logf("tool %s called with %s", params.Name, summarizeToolArguments(params.Arguments))

	tool, ok := s.cfg.Registry.Get(params.Name)
	if !ok {
		return nil, &jsonrpc.Error{
			Code:    -32601,
			Message: fmt.Sprintf("Unknown tool: %s", params.Name),
		}
	}

	value, err := tool.Handler(ctx, params.Arguments)
	if err != nil {
		s.logf("tool %s failed: %v", params.Name, err)
		return map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": "tool execution failed"},
			},
			"isError": true,
		}, nil
	}
	s.logf("tool %s succeeded", params.Name)

	text := renderResultText(value)
	resp := map[string]any{
		"content": []map[string]any{
			{"type": "text", "text": text},
		},
	}
	if structured, ok := value.(map[string]any); ok {
		if isError, ok := structured["isError"].(bool); ok {
			resp["isError"] = isError
		}
	}
	if value != nil {
		resp["structuredContent"] = value
	}
	return resp, nil
}

func (s *Server) handlePing(_ context.Context, _ json.RawMessage) (any, *jsonrpc.Error) {
	return map[string]any{}, nil
}

func (s *Server) handleShutdown(_ context.Context, _ json.RawMessage) (any, *jsonrpc.Error) {
	s.shutdown.Store(true)
	return map[string]any{}, nil
}

func (s *Server) writeResult(id json.RawMessage, result any) error {
	response := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	}
	return s.writeMessage(response)
}

func (s *Server) writeError(id json.RawMessage, code int, message string, data any) error {
	response := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"error": map[string]any{
			"code":    code,
			"message": message,
			"data":    data,
		},
	}
	return s.writeMessage(response)
}

func (s *Server) writeMessage(value any) error {
	s.wmu.Lock()
	defer s.wmu.Unlock()
	if s.mode == jsonrpc.FramingLineJSON {
		return jsonrpc.WriteJSONLine(s.writer, value)
	}
	return jsonrpc.WriteJSON(s.writer, value)
}

func (s *Server) logf(format string, args ...any) {
	if s.cfg.Logger != nil {
		s.cfg.Logger.Printf(format, args...)
	}
}

func renderResultText(value any) string {
	switch v := value.(type) {
	case string:
		return v
	default:
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return fmt.Sprintf("%v", value)
		}
		return string(b)
	}
}

func summarizeToolArguments(args map[string]any) string {
	if len(args) == 0 {
		return "no arguments"
	}

	keys := make([]string, 0, len(args))
	for key := range args {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		value := args[key]
		switch key {
		case "message", "result", "reason":
			if text, ok := value.(string); ok {
				parts = append(parts, fmt.Sprintf("%s=%d chars", key, len([]rune(text))))
				continue
			}
		case "tasks":
			if items, ok := value.([]any); ok {
				parts = append(parts, fmt.Sprintf("%s=%d items", key, len(items)))
				continue
			}
		}

		switch v := value.(type) {
		case string:
			parts = append(parts, fmt.Sprintf("%s=%q", key, truncateLogValue(v, 60)))
		case bool, float64, int:
			parts = append(parts, fmt.Sprintf("%s=%v", key, v))
		case []any:
			parts = append(parts, fmt.Sprintf("%s=%d items", key, len(v)))
		default:
			parts = append(parts, fmt.Sprintf("%s=%T", key, value))
		}
	}

	return strings.Join(parts, ", ")
}

func truncateLogValue(value string, maxLen int) string {
	runes := []rune(value)
	if len(runes) <= maxLen {
		return value
	}
	return string(runes[:maxLen]) + "..."
}
