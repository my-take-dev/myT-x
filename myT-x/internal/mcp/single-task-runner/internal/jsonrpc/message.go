package jsonrpc

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
)

// Error represents the JSON-RPC error object.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Message is the JSON-RPC envelope.
type Message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

// IsRequest returns true when the message is a request.
func (m Message) IsRequest() bool {
	return m.Method != "" && hasIDMember(m.ID) && hasValidID(m.ID)
}

// IsNotification returns true when the message is a notification.
func (m Message) IsNotification() bool {
	return m.Method != "" && !hasIDMember(m.ID)
}

// IsResponse returns true when the message is a response.
func (m Message) IsResponse() bool {
	return m.Method == "" && hasIDMember(m.ID) && hasValidID(m.ID)
}

func hasIDMember(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	return strings.TrimSpace(string(raw)) != ""
}

func hasValidID(raw json.RawMessage) bool {
	_, ok := ParseID(raw)
	return ok
}

// ParseID converts the raw ID into a JSON-marshallable value.
// The returned bool reports whether the raw value is a valid JSON-RPC ID type.
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
