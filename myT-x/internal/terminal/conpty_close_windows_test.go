//go:build windows

package terminal

import (
	"testing"
)

func TestConPtyCloseIdempotent(t *testing.T) {
	// Create a ConPty with nil handles to test Close idempotency
	// without requiring actual Windows pseudo console creation.
	cpty := &ConPty{
		// All handles are nil/zero — doClose will skip handle cleanup.
	}

	err1 := cpty.Close()
	err2 := cpty.Close()

	if err1 != err2 {
		t.Fatalf("Close() returned different errors: first=%v, second=%v", err1, err2)
	}
}
