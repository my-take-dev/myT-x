package jsonrpc

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"testing"
)

func TestWriteReadRoundTrip(t *testing.T) {
	var buf bytes.Buffer

	first := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "ping",
	}
	second := map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"result":  map[string]any{"ok": true},
	}

	if err := WriteJSON(&buf, first); err != nil {
		t.Fatalf("write first message: %v", err)
	}
	if err := WriteJSON(&buf, second); err != nil {
		t.Fatalf("write second message: %v", err)
	}

	reader := bufio.NewReader(&buf)

	payload1, err := ReadMessage(reader)
	if err != nil {
		t.Fatalf("read first message: %v", err)
	}

	payload2, err := ReadMessage(reader)
	if err != nil {
		t.Fatalf("read second message: %v", err)
	}

	var got1, got2 map[string]any
	if err := json.Unmarshal(payload1, &got1); err != nil {
		t.Fatalf("unmarshal first payload: %v", err)
	}
	if err := json.Unmarshal(payload2, &got2); err != nil {
		t.Fatalf("unmarshal second payload: %v", err)
	}

	if got1["method"] != "ping" {
		t.Fatalf("unexpected first method: %v", got1["method"])
	}
	if got2["id"].(float64) != 2 {
		t.Fatalf("unexpected second id: %v", got2["id"])
	}
}

func TestReadMessageAcceptsLFOnlyHeaders(t *testing.T) {
	payload := []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`)
	framed := fmt.Sprintf("Content-Length: %d\n\n%s", len(payload), payload)

	reader := bufio.NewReader(bytes.NewBufferString(framed))
	got, err := ReadMessage(reader)
	if err != nil {
		t.Fatalf("read message with LF-only headers: %v", err)
	}

	if !bytes.Equal(got, payload) {
		t.Fatalf("payload mismatch: got=%q want=%q", got, payload)
	}
}

func TestReadMessageAcceptsLineDelimitedJSON(t *testing.T) {
	line := `{"jsonrpc":"2.0","id":1,"method":"initialize"}` + "\n"

	reader := bufio.NewReader(bytes.NewBufferString(line))
	got, err := ReadMessage(reader)
	if err != nil {
		t.Fatalf("read line-delimited message: %v", err)
	}

	if string(got) != `{"jsonrpc":"2.0","id":1,"method":"initialize"}` {
		t.Fatalf("payload mismatch: got=%q", got)
	}
}

func TestReadMessageWithFramingDetectsLineJSON(t *testing.T) {
	line := `{"jsonrpc":"2.0","id":1}` + "\n"

	reader := bufio.NewReader(bytes.NewBufferString(line))
	_, framing, err := ReadMessageWithFraming(reader)
	if err != nil {
		t.Fatalf("read message with framing: %v", err)
	}
	if framing != FramingLineJSON {
		t.Fatalf("unexpected framing: %v", framing)
	}
}
