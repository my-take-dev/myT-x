package singletaskrunner

import (
	"context"

	"myT-x/internal/apptypes"
	"myT-x/internal/workerutil"
)

// Deps holds external dependencies injected at construction time.
// Emitter and IsShuttingDown are optional (defaulted by applyDefaults).
// All other function fields and SessionName are required and must be non-nil/non-empty.
type Deps struct {
	Emitter apptypes.RuntimeEventEmitter

	IsShuttingDown func() bool

	CheckPaneAlive func(paneID string) error

	SendMessagePaste func(paneID, message string) error

	SendClearCommand func(paneID, command string) error

	NewContext func() (context.Context, context.CancelFunc)

	LaunchWorker func(name string, ctx context.Context, fn func(ctx context.Context), opts workerutil.RecoveryOptions)

	BaseRecoveryOptions func() workerutil.RecoveryOptions

	SessionName string
}

func (d *Deps) validateRequired() {
	if d.CheckPaneAlive == nil || d.SendMessagePaste == nil || d.SendClearCommand == nil ||
		d.NewContext == nil || d.LaunchWorker == nil || d.BaseRecoveryOptions == nil {
		panic("singletaskrunner.NewService: required function fields in Deps must be non-nil " +
			"(CheckPaneAlive, SendMessagePaste, SendClearCommand, NewContext, LaunchWorker, BaseRecoveryOptions)")
	}
	if d.SessionName == "" {
		panic("singletaskrunner.NewService: SessionName must not be empty")
	}
}

func (d *Deps) applyDefaults() {
	if d.IsShuttingDown == nil {
		d.IsShuttingDown = func() bool { return false }
	}
	if d.Emitter == nil {
		d.Emitter = apptypes.NoopEmitter{}
	}
}
