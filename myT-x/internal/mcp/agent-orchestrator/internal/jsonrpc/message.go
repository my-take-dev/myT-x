package jsonrpc

import (
	"bytes"
	"encoding/json"
	"regexp"
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
	return m.Method != "" && hasIDMember(m.ID) && hasValidID(m.ID)
}

// IsNotification はメッセージが通知（method はあるが id がない）の場合に true を返す。
func (m Message) IsNotification() bool {
	return m.Method != "" && !hasIDMember(m.ID)
}

// IsResponse はメッセージがレスポンス（id はあるが method がない）の場合に true を返す。
func (m Message) IsResponse() bool {
	return m.Method == "" && hasIDMember(m.ID) && hasValidID(m.ID)
}

func hasIDMember(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	return strings.TrimSpace(string(raw)) != ""
}

// IDKey は id の生値を安定したマップキーに変換する。
func IDKey(raw json.RawMessage) string {
	return strings.TrimSpace(string(raw))
}

func hasValidID(raw json.RawMessage) bool {
	_, ok := ParseID(raw)
	return ok
}

// ParseID は id の生値を JSON マーシャル可能な id 値に変換する。
// 返り値の bool は JSON-RPC の有効な ID 型かどうかを表す。
func ParseID(raw json.RawMessage) (any, bool) {
	if len(raw) == 0 {
		return nil, false
	}

	var id any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&id); err != nil {
		return nil, false
	}

	switch v := id.(type) {
	case nil:
		return nil, true
	case string:
		return v, true
	case json.Number:
		if !integralJSONRPCNumberPattern.MatchString(v.String()) {
			return nil, false
		}
		return v, true
	default:
		return nil, false
	}
}

var integralJSONRPCNumberPattern = regexp.MustCompile(`^-?(0|[1-9]\d*)([eE][+-]?\d+)?$`)
