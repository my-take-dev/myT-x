package jsonrpc

import (
	"encoding/json"
	"testing"
)

func TestMessageIsRequest(t *testing.T) {
	tests := []struct {
		msg  Message
		want bool
	}{
		{Message{Method: "ping", ID: json.RawMessage(`1`)}, true},
		{Message{Method: "initialize", ID: json.RawMessage(`"abc"`)}, true},
		{Message{Method: "ping", ID: nil}, false},
		{Message{Method: "ping", ID: json.RawMessage(`null`)}, false},
		{Message{Method: "", ID: json.RawMessage(`1`)}, false},
		{Message{Method: "ping", ID: json.RawMessage(``)}, false},
	}
	for i, tt := range tests {
		got := tt.msg.IsRequest()
		if got != tt.want {
			t.Errorf("case %d: IsRequest() = %v, want %v", i, got, tt.want)
		}
	}
}

func TestMessageIsNotification(t *testing.T) {
	tests := []struct {
		msg  Message
		want bool
	}{
		{Message{Method: "initialized", ID: nil}, true},
		{Message{Method: "exit", ID: json.RawMessage(`null`)}, true},
		{Message{Method: "ping", ID: json.RawMessage(`1`)}, false},
		{Message{Method: "", ID: nil}, false},
	}
	for i, tt := range tests {
		got := tt.msg.IsNotification()
		if got != tt.want {
			t.Errorf("case %d: IsNotification() = %v, want %v", i, got, tt.want)
		}
	}
}

func TestMessageIsResponse(t *testing.T) {
	tests := []struct {
		msg  Message
		want bool
	}{
		{Message{Method: "", ID: json.RawMessage(`1`), Result: json.RawMessage(`{}`)}, true},
		{Message{Method: "", ID: json.RawMessage(`"req-1"`)}, true},
		{Message{Method: "ping", ID: json.RawMessage(`1`)}, false},
		{Message{Method: "", ID: nil}, false},
		{Message{Method: "", ID: json.RawMessage(`null`)}, false},
	}
	for i, tt := range tests {
		got := tt.msg.IsResponse()
		if got != tt.want {
			t.Errorf("case %d: IsResponse() = %v, want %v", i, got, tt.want)
		}
	}
}

func TestIDKey(t *testing.T) {
	tests := []struct {
		raw  json.RawMessage
		want string
	}{
		{json.RawMessage(`1`), "1"},
		{json.RawMessage(`123`), "123"},
		{json.RawMessage(`"abc"`), `"abc"`},
		{json.RawMessage(`null`), "null"},
		{json.RawMessage(``), ""},
		{json.RawMessage(`  1  `), "1"},
	}
	for i, tt := range tests {
		got := IDKey(tt.raw)
		if got != tt.want {
			t.Errorf("case %d: IDKey(%q) = %q, want %q", i, string(tt.raw), got, tt.want)
		}
	}
}

func TestParseID(t *testing.T) {
	tests := []struct {
		raw   json.RawMessage
		want  any
		equal func(any, any) bool
	}{
		{json.RawMessage(`1`), int64(1), func(a, b any) bool { return a.(int64) == b.(int64) }},
		{json.RawMessage(`123`), int64(123), func(a, b any) bool { return a.(int64) == b.(int64) }},
		{json.RawMessage(`"abc"`), "abc", func(a, b any) bool { return a.(string) == b.(string) }},
		{json.RawMessage(``), nil, func(a, b any) bool { return a == nil && b == nil }},
		// null は int64 に Unmarshal され 0 になる
		{json.RawMessage(`null`), int64(0), func(a, b any) bool { return a.(int64) == b.(int64) }},
	}
	for i, tt := range tests {
		got := ParseID(tt.raw)
		if !tt.equal(got, tt.want) {
			t.Errorf("case %d: ParseID(%q) = %v, want %v", i, string(tt.raw), got, tt.want)
		}
	}
}

func TestParseIDFallbackToQuoted(t *testing.T) {
	// 不正な形式の ID は strconv.Quote でラップされる
	got := ParseID(json.RawMessage(`[1,2]`))
	if s, ok := got.(string); !ok || s != `"[1,2]"` {
		t.Errorf("ParseID([1,2]) = %v, want quoted string", got)
	}
}
