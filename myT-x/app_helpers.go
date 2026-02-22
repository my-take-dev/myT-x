package main

import (
	"fmt"
)

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
