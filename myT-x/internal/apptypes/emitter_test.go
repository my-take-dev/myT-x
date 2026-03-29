package apptypes

import (
	"context"
	"testing"
)

func TestEventEmitterFunc_Emit(t *testing.T) {
	tests := []struct {
		name    string
		event   string
		payload any
	}{
		{"simple string payload", "test:event", "hello"},
		{"nil payload", "test:nil", nil},
		{"struct payload", "test:struct", struct{ ID int }{42}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotName string
			var gotPayload any
			fn := EventEmitterFunc(func(name string, payload any) {
				gotName = name
				gotPayload = payload
			})

			fn.Emit(tt.event, tt.payload)

			if gotName != tt.event {
				t.Errorf("Emit() name = %q, want %q", gotName, tt.event)
			}
			if gotPayload != tt.payload {
				t.Errorf("Emit() payload = %v, want %v", gotPayload, tt.payload)
			}
		})
	}
}

func TestEventEmitterFunc_EmitWithContext(t *testing.T) {
	var gotName string
	var gotPayload any
	fn := EventEmitterFunc(func(name string, payload any) {
		gotName = name
		gotPayload = payload
	})

	ctx := context.Background()
	fn.EmitWithContext(ctx, "ctx:event", "data")

	if gotName != "ctx:event" {
		t.Errorf("EmitWithContext() name = %q, want %q", gotName, "ctx:event")
	}
	if gotPayload != "data" {
		t.Errorf("EmitWithContext() payload = %v, want %q", gotPayload, "data")
	}
}

func TestNoopEmitter(t *testing.T) {
	// NoopEmitter must satisfy RuntimeEventEmitter without panicking.
	var e RuntimeEventEmitter = NoopEmitter{}
	e.Emit("test", nil)
	e.EmitWithContext(context.Background(), "test", nil)
}
