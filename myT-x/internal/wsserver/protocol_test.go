package wsserver

import (
	"strings"
	"testing"
)

func TestEncodeDecode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		paneID     string
		data       []byte
		wantPaneID string // expected paneID after decode (may differ from input if truncated)
		wantData   []byte
	}{
		{
			name:       "RoundTrip_NormalPaneID",
			paneID:     "%0",
			data:       []byte("hello"),
			wantPaneID: "%0",
			wantData:   []byte("hello"),
		},
		{
			name:       "RoundTrip_EmptyData",
			paneID:     "%1",
			data:       []byte{},
			wantPaneID: "%1",
			wantData:   []byte{},
		},
		{
			name:       "RoundTrip_MaxPaneIDLength",
			paneID:     strings.Repeat("a", 255),
			data:       []byte("data"),
			wantPaneID: strings.Repeat("a", 255),
			wantData:   []byte("data"),
		},
		{
			name:       "RoundTrip_BinaryData",
			paneID:     "%2",
			data:       []byte{0x00, 0x01, 0x7f, 0x80, 0xfe, 0xff},
			wantPaneID: "%2",
			wantData:   []byte{0x00, 0x01, 0x7f, 0x80, 0xfe, 0xff},
		},
		{
			name:       "Encode_PaneIDTruncation",
			paneID:     strings.Repeat("b", 256),
			data:       []byte("truncated"),
			wantPaneID: strings.Repeat("b", 255),
			wantData:   []byte("truncated"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			frame, err := EncodePaneData(tt.paneID, tt.data)
			if err != nil {
				t.Fatalf("EncodePaneData returned unexpected error: %v", err)
			}

			// Verify frame structure: first byte is pane ID length.
			expectedIDLen := len(tt.wantPaneID)
			if int(frame[0]) != expectedIDLen {
				t.Fatalf("frame[0] = %d, want %d", frame[0], expectedIDLen)
			}

			// Verify total frame size: 1 + len(paneID) + len(data).
			expectedFrameLen := 1 + expectedIDLen + len(tt.wantData)
			if len(frame) != expectedFrameLen {
				t.Fatalf("frame length = %d, want %d", len(frame), expectedFrameLen)
			}

			gotPaneID, gotData, decErr := DecodePaneData(frame)
			if decErr != nil {
				t.Fatalf("DecodePaneData returned unexpected error: %v", decErr)
			}
			if gotPaneID != tt.wantPaneID {
				t.Errorf("paneID = %q, want %q", gotPaneID, tt.wantPaneID)
			}
			if len(gotData) != len(tt.wantData) {
				t.Fatalf("data length = %d, want %d", len(gotData), len(tt.wantData))
			}
			for i := range gotData {
				if gotData[i] != tt.wantData[i] {
					t.Errorf("data[%d] = %d, want %d", i, gotData[i], tt.wantData[i])
				}
			}
		})
	}
}

func TestEncodePaneData_EmptyPaneIDError(t *testing.T) {
	t.Parallel()

	_, err := EncodePaneData("", []byte("noID"))
	if err == nil {
		t.Fatal("EncodePaneData should return error for empty paneID")
	}
}

func TestDecodePaneData_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		frame         []byte
		wantErrSubstr string
	}{
		{
			name:          "Decode_NilFrame",
			frame:         nil,
			wantErrSubstr: "empty frame",
		},
		{
			name:          "Decode_EmptyFrame",
			frame:         []byte{},
			wantErrSubstr: "empty frame",
		},
		{
			name:          "Decode_TooShort",
			frame:         []byte{5}, // declares paneID length 5, but no data follows
			wantErrSubstr: "frame too short",
		},
		{
			name:          "Decode_PaneIDLengthExceedsFrame",
			frame:         []byte{10, 'a'}, // declares paneID length 10, only 1 byte follows
			wantErrSubstr: "frame too short",
		},
		{
			name:          "Decode_ValidPaneIDLenButTruncated",
			frame:         []byte{3, 'a', 'b'}, // declares paneID length 3, but only 2 bytes of paneID follow
			wantErrSubstr: "frame too short",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, _, err := DecodePaneData(tt.frame)
			if err == nil {
				t.Fatal("DecodePaneData should have returned an error for malformed frame")
			}
			if !strings.Contains(err.Error(), tt.wantErrSubstr) {
				t.Errorf("error = %q, want substring %q", err.Error(), tt.wantErrSubstr)
			}
		})
	}
}

func TestEncodePaneData_SingleAllocation(t *testing.T) {
	t.Parallel()

	paneID := "%0"
	data := []byte("test data for allocation check")

	// Verify the frame is built correctly with a single contiguous buffer.
	frame, err := EncodePaneData(paneID, data)
	if err != nil {
		t.Fatalf("EncodePaneData returned unexpected error: %v", err)
	}

	// The encoded frame must be exactly 1 + len(paneID) + len(data) bytes.
	expectedLen := 1 + len(paneID) + len(data)
	if len(frame) != expectedLen {
		t.Errorf("frame length = %d, want %d", len(frame), expectedLen)
	}
	if cap(frame) != expectedLen {
		t.Errorf("frame capacity = %d, want %d (should be single allocation)", cap(frame), expectedLen)
	}
}

func BenchmarkEncodePaneData(b *testing.B) {
	paneID := "%0"
	data := make([]byte, 4096) // typical terminal output chunk
	for i := range data {
		data[i] = byte(i % 256)
	}

	for b.Loop() {
		_, _ = EncodePaneData(paneID, data)
	}
}

func BenchmarkDecodePaneData(b *testing.B) {
	paneID := "%0"
	data := make([]byte, 4096)
	frame, _ := EncodePaneData(paneID, data)

	for b.Loop() {
		_, _, _ = DecodePaneData(frame)
	}
}
