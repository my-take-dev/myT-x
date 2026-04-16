package jsonrpc

import (
	"encoding/json"
	"testing"
)

func TestParseID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		raw   json.RawMessage
		want  any
		valid bool
	}{
		{name: "string", raw: json.RawMessage(`"abc"`), want: "abc", valid: true},
		{name: "integer", raw: json.RawMessage(`1`), want: json.Number("1"), valid: true},
		{name: "integer with exponent", raw: json.RawMessage(`1e3`), want: json.Number("1e3"), valid: true},
		{name: "float", raw: json.RawMessage(`1.5`), want: nil, valid: false},
		{name: "null", raw: json.RawMessage(`null`), want: nil, valid: true},
		{name: "object", raw: json.RawMessage(`{"bad":true}`), want: nil, valid: false},
		{name: "array", raw: json.RawMessage(`[1]`), want: nil, valid: false},
		{name: "bool", raw: json.RawMessage(`true`), want: nil, valid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, valid := ParseID(tt.raw)
			if valid != tt.valid {
				t.Fatalf("valid = %v, want %v", valid, tt.valid)
			}
			if got != tt.want {
				t.Fatalf("ParseID(%s) = %#v, want %#v", string(tt.raw), got, tt.want)
			}
		})
	}
}

func TestMessageClassification(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		message          Message
		wantRequest      bool
		wantNotification bool
		wantResponse     bool
	}{
		{
			name:             "request with null id",
			message:          Message{Method: "ping", ID: json.RawMessage(`null`)},
			wantRequest:      true,
			wantNotification: false,
			wantResponse:     false,
		},
		{
			name:             "invalid float id request",
			message:          Message{Method: "ping", ID: json.RawMessage(`1.5`)},
			wantRequest:      false,
			wantNotification: false,
			wantResponse:     false,
		},
		{
			name:             "invalid object id response",
			message:          Message{ID: json.RawMessage(`{"bad":true}`)},
			wantRequest:      false,
			wantNotification: false,
			wantResponse:     false,
		},
		{
			name:             "notification without id",
			message:          Message{Method: "ping"},
			wantRequest:      false,
			wantNotification: true,
			wantResponse:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.message.IsRequest(); got != tt.wantRequest {
				t.Fatalf("IsRequest() = %v, want %v", got, tt.wantRequest)
			}
			if got := tt.message.IsNotification(); got != tt.wantNotification {
				t.Fatalf("IsNotification() = %v, want %v", got, tt.wantNotification)
			}
			if got := tt.message.IsResponse(); got != tt.wantResponse {
				t.Fatalf("IsResponse() = %v, want %v", got, tt.wantResponse)
			}
		})
	}
}
