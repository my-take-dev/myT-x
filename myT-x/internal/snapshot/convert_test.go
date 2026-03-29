package snapshot

import (
	"bytes"
	"testing"

	"myT-x/internal/tmux"
)

// ---------------------------------------------------------------------------
// paneOutputEventFromPayload
// ---------------------------------------------------------------------------

func TestPaneOutputEventFromPayload(t *testing.T) {
	tests := []struct {
		name        string
		payload     any
		wantHandled bool
		wantNil     bool
		wantPaneID  string
		wantData    []byte
	}{
		{
			name:        "pointer_non_nil",
			payload:     &tmux.PaneOutputEvent{PaneID: "%5", Data: []byte("hello")},
			wantHandled: true,
			wantNil:     false,
			wantPaneID:  "%5",
			wantData:    []byte("hello"),
		},
		{
			name:        "value_type",
			payload:     tmux.PaneOutputEvent{PaneID: "%3", Data: []byte("world")},
			wantHandled: true,
			wantNil:     false,
			wantPaneID:  "%3",
			wantData:    []byte("world"),
		},
		{
			name:        "nil_pointer",
			payload:     (*tmux.PaneOutputEvent)(nil),
			wantHandled: true,
			wantNil:     true,
		},
		{
			name:        "unsupported_type_int",
			payload:     42,
			wantHandled: false,
		},
		{
			name:        "unsupported_type_string",
			payload:     "not-an-event",
			wantHandled: false,
		},
		{
			name:        "unsupported_type_map",
			payload:     map[string]any{"paneId": "%0"},
			wantHandled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evt, handled := paneOutputEventFromPayload(tt.payload)

			if handled != tt.wantHandled {
				t.Fatalf("handled = %v, want %v", handled, tt.wantHandled)
			}
			if !handled {
				return
			}
			if tt.wantNil {
				if evt != nil {
					t.Fatalf("expected nil event, got %+v", evt)
				}
				return
			}
			if evt == nil {
				t.Fatal("expected non-nil event")
			}
			if evt.PaneID != tt.wantPaneID {
				t.Errorf("PaneID = %q, want %q", evt.PaneID, tt.wantPaneID)
			}
			if !bytes.Equal(evt.Data, tt.wantData) {
				t.Errorf("Data = %q, want %q", evt.Data, tt.wantData)
			}
		})
	}
}

func TestPaneOutputEventFromPayloadValueCopiesData(t *testing.T) {
	original := []byte("original")
	evt := tmux.PaneOutputEvent{PaneID: "%0", Data: original}

	result, handled := paneOutputEventFromPayload(evt)
	if !handled || result == nil {
		t.Fatal("expected handled non-nil result")
	}

	// Mutate the original; the copy must not be affected.
	original[0] = 'X'
	if result.Data[0] == 'X' {
		t.Error("value-type path should deep-copy Data to avoid aliasing")
	}
}

// ---------------------------------------------------------------------------
// toString
// ---------------------------------------------------------------------------

func TestToString(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  string
	}{
		{"nil", nil, ""},
		{"string", "hello", "hello"},
		{"int", 42, "42"},
		{"bool", true, "true"},
		{"empty_string", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toString(tt.input)
			if got != tt.want {
				t.Errorf("toString(%v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// toBytes
// ---------------------------------------------------------------------------

func TestToBytes(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		wantNil bool
		want    []byte
	}{
		{"nil", nil, true, nil},
		{"byte_slice", []byte("data"), false, []byte("data")},
		{"string", "str", false, []byte("str")},
		{"int_unsupported", 42, true, nil},
		{"struct_unsupported", struct{ X int }{1}, true, nil},
		{"empty_string", "", false, []byte("")},
		{"empty_byte_slice", []byte{}, false, []byte{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toBytes(tt.input)
			if tt.wantNil {
				if got != nil {
					t.Errorf("toBytes(%v) = %v, want nil", tt.input, got)
				}
				return
			}
			if !bytes.Equal(got, tt.want) {
				t.Errorf("toBytes(%v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// parseLegacyMapPaneOutput
// ---------------------------------------------------------------------------

func TestParseLegacyMapPaneOutput(t *testing.T) {
	tests := []struct {
		name       string
		data       map[string]any
		wantPaneID string
		wantChunk  []byte
		wantNil    bool
	}{
		{
			name:       "valid_string_data",
			data:       map[string]any{"paneId": "%0", "data": "hello"},
			wantPaneID: "%0",
			wantChunk:  []byte("hello"),
		},
		{
			name:       "valid_byte_data",
			data:       map[string]any{"paneId": "%1", "data": []byte("world")},
			wantPaneID: "%1",
			wantChunk:  []byte("world"),
		},
		{
			name:       "missing_pane_id",
			data:       map[string]any{"data": []byte("orphan")},
			wantPaneID: "",
			wantChunk:  []byte("orphan"),
		},
		{
			name:       "nil_data",
			data:       map[string]any{"paneId": "%0", "data": nil},
			wantPaneID: "%0",
			wantNil:    true,
		},
		{
			name:       "unsupported_data_type_returns_nil",
			data:       map[string]any{"paneId": "%0", "data": 12345},
			wantPaneID: "%0",
			wantNil:    true,
		},
		{
			name:       "empty_map",
			data:       map[string]any{},
			wantPaneID: "",
			wantNil:    true,
		},
		{
			name:       "whitespace_pane_id_trimmed",
			data:       map[string]any{"paneId": "  %2  ", "data": "x"},
			wantPaneID: "%2",
			wantChunk:  []byte("x"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paneID, chunk := parseLegacyMapPaneOutput(tt.data)
			if paneID != tt.wantPaneID {
				t.Errorf("paneID = %q, want %q", paneID, tt.wantPaneID)
			}
			if tt.wantNil {
				if chunk != nil {
					t.Errorf("chunk = %q, want nil", chunk)
				}
				return
			}
			if !bytes.Equal(chunk, tt.wantChunk) {
				t.Errorf("chunk = %q, want %q", chunk, tt.wantChunk)
			}
		})
	}
}
