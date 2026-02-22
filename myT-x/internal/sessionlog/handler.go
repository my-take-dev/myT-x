package sessionlog

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"runtime/debug"
	"time"
)

// EntryCallback is invoked for each log record at or above the capture threshold.
// Parameters:
//   - ts: timestamp of the log record
//   - level: severity level of the record
//   - msg: the log message text
//   - group: the accumulated handler group name (dot-separated) or empty string.
//     NOTE: This parameter is named "group" to match slog terminology. Callers
//     typically map it to SessionLogEntry.Source for UI display. The naming
//     difference is intentional: "group" reflects the slog origin, "Source"
//     reflects the domain-level concept exposed to the frontend.
type EntryCallback func(ts time.Time, level slog.Level, msg string, group string)

// TeeHandler wraps a base [slog.Handler] and tees records at or above minLevel
// to a callback function. All records are forwarded to the base handler regardless
// of level; only the callback invocation is gated by minLevel.
type TeeHandler struct {
	base     slog.Handler
	callback EntryCallback
	minLevel slog.Level
	group    string // accumulated dot-separated slog group name (mapped to SessionLogEntry.Source by callers)
}

// NewTeeHandler creates a TeeHandler that delegates to base and invokes callback
// for every record whose level is >= minLevel.
//
// Passing a nil callback is safe; the handler will simply delegate to base without
// teeing.
func NewTeeHandler(base slog.Handler, minLevel slog.Level, callback EntryCallback) *TeeHandler {
	return &TeeHandler{
		base:     base,
		callback: callback,
		minLevel: minLevel,
	}
}

// Enabled reports whether the base handler is enabled for the given level.
// The callback threshold (minLevel) does not affect this; we always let the
// base handler decide visibility.
func (h *TeeHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.base.Enabled(ctx, level)
}

// Handle forwards the record to the base handler, then conditionally invokes
// the callback if the record's level meets or exceeds minLevel.
//
// NOTE: The callback is invoked regardless of base handler error because UI
// notification of the log event should not depend on base handler success.
// The callback does not receive the base error; it only observes the log record.
func (h *TeeHandler) Handle(ctx context.Context, record slog.Record) error {
	err := h.base.Handle(ctx, record)

	if h.callback != nil && record.Level >= h.minLevel {
		func() {
			defer func() {
				if r := recover(); r != nil {
					// NOTE: Callback panic is logged to stderr (not slog) to avoid
					// recursive TeeHandler invocation. The base handler result is
					// preserved and returned to the caller.
					fmt.Fprintf(os.Stderr, "[session-log] callback panicked: %v\n%s\n", r, debug.Stack())
				}
			}()
			h.callback(record.Time, record.Level, record.Message, h.group)
		}()
	}

	// Returning the base handler error is intentional. slog.Logger emits it to stderr
	// as an internal fallback ("slog: <error>"), making handler failures visible.
	return err
}

// WithAttrs returns a new TeeHandler whose base handler has the given attributes
// applied. The callback, minLevel, and accumulated group are preserved.
func (h *TeeHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	return &TeeHandler{
		base:     h.base.WithAttrs(attrs),
		callback: h.callback,
		minLevel: h.minLevel,
		group:    h.group,
	}
}

// WithGroup returns a new TeeHandler whose base handler is wrapped with the
// given group name. The group name is appended to the accumulated group string,
// separated by "." if a prefix already exists.
func (h *TeeHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h // slog.Handler spec: empty group name returns the receiver unchanged.
	}
	newGroup := name
	if h.group != "" {
		newGroup = h.group + "." + name
	}

	return &TeeHandler{
		base:     h.base.WithGroup(name),
		callback: h.callback,
		minLevel: h.minLevel,
		group:    newGroup,
	}
}
