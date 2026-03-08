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
	"sync"

	"myT-x/internal/mcp/lspmcp/internal/jsonrpc"
)

// defaultProtocolVersion は本サーバーが既定で利用する MCP プロトコルバージョン。
const defaultProtocolVersion = "2024-11-05"

// Config は MCP サーバーのトランスポートとメタデータを設定する。
type Config struct {
	// Name は initialize 応答に含めるサーバー名。
	Name string
	// Version は initialize 応答に含めるサーバー実装バージョン。
	Version string
	// ProtocolVersion は initialize 応答で通知する MCP プロトコルバージョン。
	ProtocolVersion string
	// In は MCP リクエストを受信する入力ストリーム。
	In io.Reader
	// Out は MCP レスポンスを書き出す出力ストリーム。
	Out io.Writer
	// Logger はサーバー内部の診断ログ出力先。
	Logger *log.Logger
	// Registry は tools/list と tools/call に使うツール定義。
	Registry *Registry
}

// Server は stdio 経由で MCP リクエストを処理する。
type Server struct {
	cfg Config

	reader *bufio.Reader
	writer io.Writer
	mode   jsonrpc.Framing

	wmu sync.Mutex

	// shutdown は将来の graceful shutdown 用に予約。Serve ループでは未使用。
	shutdown bool
}

// NewServer は妥当な既定値でサーバーを構築する。
func NewServer(cfg Config) *Server {
	if cfg.Name == "" {
		cfg.Name = "generic-lspmcp"
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
		cfg.Registry = NewRegistry(nil)
	}

	return &Server{
		cfg:    cfg,
		reader: bufio.NewReader(cfg.In),
		writer: cfg.Out,
		mode:   jsonrpc.FramingUnknown,
	}
}

// Serve は EOF またはコンテキストキャンセルまで MCP メッセージを処理するためにブロックする。
func (s *Server) Serve(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		payload, mode, err := jsonrpc.ReadMessageWithFraming(s.reader)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		if s.mode == jsonrpc.FramingUnknown && mode != jsonrpc.FramingUnknown {
			s.mode = mode
		}

		var msg jsonrpc.Message
		if err := json.Unmarshal(payload, &msg); err != nil {
			if writeErr := s.writeError(nil, -32700, "Parse error", err.Error()); writeErr != nil {
				return writeErr
			}
			continue
		}

		if msg.IsNotification() {
			if msg.Method == "notifications/initialized" {
				continue
			}
			if msg.Method == "exit" {
				return nil
			}
			continue
		}

		if !msg.IsRequest() {
			continue
		}

		id := jsonrpc.ParseID(msg.ID)
		result, rpcErr := s.handleRequest(ctx, msg.Method, msg.Params)
		if rpcErr != nil {
			if err := s.writeError(id, rpcErr.Code, rpcErr.Message, rpcErr.Data); err != nil {
				return err
			}
			continue
		}
		if err := s.writeResult(id, result); err != nil {
			return err
		}
	}
}

func (s *Server) handleRequest(ctx context.Context, method string, paramsRaw json.RawMessage) (any, *jsonrpc.Error) {
	switch method {
	case "initialize":
		var params struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		if err := json.Unmarshal(paramsRaw, &params); err != nil && s.cfg.Logger != nil {
			s.cfg.Logger.Printf("initialize params の解析に失敗: %v", err)
		}

		protocolVersion := s.cfg.ProtocolVersion
		if params.ProtocolVersion != "" {
			protocolVersion = params.ProtocolVersion
		}

		return map[string]any{
			"protocolVersion": protocolVersion,
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

	case "tools/list":
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

	case "tools/call":
		var params struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal(paramsRaw, &params); err != nil {
			return nil, &jsonrpc.Error{Code: -32602, Message: "Invalid params", Data: err.Error()}
		}
		if params.Name == "" {
			return nil, &jsonrpc.Error{Code: -32602, Message: "Invalid params", Data: "name is required"}
		}
		if params.Arguments == nil {
			params.Arguments = map[string]any{}
		}

		tool, ok := s.cfg.Registry.Get(params.Name)
		if !ok {
			return nil, &jsonrpc.Error{
				Code:    -32601,
				Message: fmt.Sprintf("Unknown tool: %s", params.Name),
			}
		}

		value, err := tool.Handler(ctx, params.Arguments)
		if err != nil {
			return map[string]any{
				"content": []map[string]any{
					{"type": "text", "text": err.Error()},
				},
				"isError": true,
			}, nil
		}

		text := renderResultText(value)
		resp := map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": text},
			},
		}
		if value != nil {
			resp["structuredContent"] = value
		}
		return resp, nil

	case "ping":
		return map[string]any{}, nil

	case "shutdown":
		s.shutdown = true
		return map[string]any{}, nil
	}

	return nil, &jsonrpc.Error{
		Code:    -32601,
		Message: fmt.Sprintf("Method not found: %s", method),
	}
}

func (s *Server) writeResult(id any, result any) error {
	response := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	}
	return s.writeMessage(response)
}

func (s *Server) writeError(id any, code int, message string, data any) error {
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
