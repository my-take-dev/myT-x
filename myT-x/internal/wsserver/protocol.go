// Package wsserver provides a WebSocket server for streaming terminal output
// to the frontend.
//
// # Binary frame protocol
//
// Binary frame format: [1 byte: paneID length][paneID bytes][data bytes]
//
//   - Byte 0: uint8 length of the pane ID (0..255).
//   - Bytes 1..1+paneIDLen: pane ID encoded as ASCII/UTF-8.
//   - Remaining bytes: raw terminal data (may be empty).
//
// EncodePaneData produces frames in this format; DecodePaneData parses them.
package wsserver

import (
	"fmt"
	"log/slog"
)

// maxPaneIDLen is the maximum pane ID length that fits in the 1-byte length
// prefix of the binary frame protocol. Pane IDs exceeding this are truncated.
const maxPaneIDLen = 255

// EncodePaneData constructs a binary frame for streaming terminal output to
// the frontend.
//
// Frame format:
//
//	[1 byte: len(paneID) as uint8] [paneID bytes (ASCII)] [data bytes]
//
// The frame avoids JSON serialization overhead on the hot path (~60Hz per pane).
// A single allocation is used: make([]byte, 1+len(paneID)+len(data)).
//
// Precondition: len(paneID) must fit in uint8 (max 255 bytes). Longer pane IDs
// are silently truncated to 255 bytes with a debug log.
func EncodePaneData(paneID string, data []byte) ([]byte, error) {
	if len(paneID) == 0 {
		return nil, fmt.Errorf("wsserver: encode pane data: paneID must not be empty")
	}

	id := paneID
	if len(id) > maxPaneIDLen {
		// Warn (not Debug) because truncation changes the pane ID used for
		// routing, risking data delivery to the wrong pane if two IDs share
		// the same 255-byte prefix.
		slog.Warn("[DEBUG-WS] paneID truncated — collision risk: different panes may receive each other's data",
			"originalLen", len(id), "truncatedTo", maxPaneIDLen, "paneID", id[:maxPaneIDLen])
		id = id[:maxPaneIDLen]
	}

	idLen := len(id)
	buf := make([]byte, 1+idLen+len(data))
	buf[0] = byte(idLen)
	copy(buf[1:1+idLen], id)
	copy(buf[1+idLen:], data)
	return buf, nil
}

// DecodePaneData parses a binary frame produced by EncodePaneData.
// Returns the pane ID and raw terminal data, or an error if the frame is
// malformed (empty frame, insufficient length for declared pane ID).
//
// Zero-copy: The returned data slice shares memory with frame.
// Callers must not modify frame after calling this function.
func DecodePaneData(frame []byte) (paneID string, data []byte, err error) {
	if len(frame) < 1 {
		return "", nil, fmt.Errorf("wsserver: decode pane data: empty frame")
	}

	idLen := int(frame[0])
	// The frame must contain at least the length byte + idLen bytes of pane ID.
	if len(frame) < 1+idLen {
		return "", nil, fmt.Errorf("wsserver: decode pane data: frame too short for paneID length %d (frame length %d)", idLen, len(frame))
	}

	paneID = string(frame[1 : 1+idLen])
	data = frame[1+idLen:]
	return paneID, data, nil
}
