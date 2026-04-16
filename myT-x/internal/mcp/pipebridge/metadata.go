package pipebridge

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
)

const callerPaneHandshakePrefix = "MYTX_CALLER_PANE "

var callerPanePattern = regexp.MustCompile(`^%\d+$`)

type callerPaneContextKey struct{}

// ContextWithCallerPaneID stores a caller pane id on the context for a single
// pipe connection.
func ContextWithCallerPaneID(ctx context.Context, paneID string) context.Context {
	paneID = normalizeCallerPaneID(paneID)
	if paneID == "" {
		return ctx
	}
	return context.WithValue(ctx, callerPaneContextKey{}, paneID)
}

// CallerPaneIDFromContext returns the caller pane id attached to the context.
func CallerPaneIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	paneID, _ := ctx.Value(callerPaneContextKey{}).(string)
	return normalizeCallerPaneID(paneID)
}

// WriteCallerPaneHandshake writes an optional best-effort caller pane header
// ahead of the normal MCP stream. Empty or invalid pane ids are ignored.
func WriteCallerPaneHandshake(w io.Writer, paneID string) error {
	paneID = normalizeCallerPaneID(paneID)
	if paneID == "" {
		return nil
	}
	if _, err := io.WriteString(w, callerPaneHandshakePrefix+paneID+"\n"); err != nil {
		return fmt.Errorf("write caller pane handshake: %w", err)
	}
	return nil
}

// ReadCallerPaneHandshake consumes an optional caller pane header and returns a
// buffered reader that preserves the remaining MCP payload. Missing handshakes
// are expected and simply mean the caller stays in trusted pipe mode.
func ReadCallerPaneHandshake(r io.Reader) (*bufio.Reader, string, error) {
	reader := bufio.NewReader(r)
	peeked, err := reader.Peek(len(callerPaneHandshakePrefix))
	if err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, bufio.ErrBufferFull) {
			return reader, "", nil
		}
		return nil, "", fmt.Errorf("peek caller pane handshake: %w", err)
	}
	if string(peeked) != callerPaneHandshakePrefix {
		return reader, "", nil
	}

	line, err := reader.ReadString('\n')
	if err != nil {
		return nil, "", fmt.Errorf("read caller pane handshake: %w", err)
	}
	paneID := normalizeCallerPaneID(strings.TrimPrefix(strings.TrimSpace(line), callerPaneHandshakePrefix))
	if paneID == "" {
		return nil, "", fmt.Errorf("invalid caller pane handshake %q", strings.TrimSpace(line))
	}
	return reader, paneID, nil
}

func normalizeCallerPaneID(paneID string) string {
	paneID = strings.TrimSpace(paneID)
	if !callerPanePattern.MatchString(paneID) {
		return ""
	}
	return paneID
}
