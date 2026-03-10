package jsonrpc

import (
	"encoding/json"
	"strconv"
	"strings"
)

// Error は JSON-RPC のエラーオブジェクトを表す。
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Message は JSON-RPC エンベロープ。
type Message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

// IsRequest はメッセージがリクエスト（method と id を持つ）の場合に true を返す。
func (m Message) IsRequest() bool {
	return m.Method != "" && hasID(m.ID)
}

// IsNotification はメッセージが通知（method はあるが id がない）の場合に true を返す。
func (m Message) IsNotification() bool {
	return m.Method != "" && !hasID(m.ID)
}

// IsResponse はメッセージがレスポンス（id はあるが method がない）の場合に true を返す。
func (m Message) IsResponse() bool {
	return m.Method == "" && hasID(m.ID)
}

func hasID(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	trimmed := strings.TrimSpace(string(raw))
	return trimmed != "" && trimmed != "null"
}

// IDKey は id の生値を安定したマップキーに変換する。
func IDKey(raw json.RawMessage) string {
	return strings.TrimSpace(string(raw))
}

// ParseID は id の生値を JSON マーシャル可能な id 値に変換する。
func ParseID(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}

	var intID int64
	if err := json.Unmarshal(raw, &intID); err == nil {
		return intID
	}

	var floatID float64
	if err := json.Unmarshal(raw, &floatID); err == nil {
		return floatID
	}

	var strID string
	if err := json.Unmarshal(raw, &strID); err == nil {
		return strID
	}

	return strconv.Quote(string(raw))
}
