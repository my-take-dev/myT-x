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
