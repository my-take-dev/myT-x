package snapshot

// convert.go — Payload type conversion helpers for pane output events.

import (
	"fmt"
	"strings"

	"myT-x/internal/tmux"
)

// paneOutputEventFromPayload extracts a PaneOutputEvent from a generic payload.
// Returns (event, handled). When handled is true but event is nil, the payload
// was a typed nil pointer (no-op for backward compatibility).
func paneOutputEventFromPayload(payload any) (*tmux.PaneOutputEvent, bool) {
	switch event := payload.(type) {
	case *tmux.PaneOutputEvent:
		if event == nil {
			return nil, true
		}
		return event, true
	case tmux.PaneOutputEvent:
		copied := event
		copied.Data = append([]byte(nil), event.Data...)
		return &copied, true
	default:
		return nil, false
	}
}

// toString converts a value to its string representation.
// Returns empty string for nil.
func toString(value any) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	default:
		return fmt.Sprintf("%v", value)
	}
}

// toBytes converts a value to []byte for supported types.
//
// Supported types:
//   - nil      -> returns nil
//   - []byte   -> returns the slice as-is (no copy; caller must copy before async use)
//   - string   -> returns []byte(v)
//
// Unsupported types (int, struct, etc.) return nil. Callers should check
// for nil with a non-nil input to detect unsupported type mismatches.
//
// WARNING: The []byte path returns the original slice without copying. The caller
// aliases the original backing array, so mutations are visible to the original
// holder. If the returned slice will be used asynchronously (e.g. sent to a
// channel or goroutine), the caller must copy it first to avoid data races.
func toBytes(value any) []byte {
	switch v := value.(type) {
	case nil:
		return nil
	case []byte:
		return v
	case string:
		return []byte(v)
	default:
		return nil
	}
}

// parseLegacyMapPaneOutput extracts paneID and chunk from a legacy map[string]any payload.
// Returns empty paneID or nil chunk if the payload is invalid.
func parseLegacyMapPaneOutput(data map[string]any) (paneID string, chunk []byte) {
	paneID = strings.TrimSpace(toString(data["paneId"]))
	rawData := data["data"]
	chunk = toBytes(rawData)
	if chunk == nil && rawData != nil {
		return paneID, nil
	}
	return paneID, chunk
}
