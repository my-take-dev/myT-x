package main

import (
	"context"
	"testing"

	"myT-x/internal/devpanel"
	"myT-x/internal/mcp"
	"myT-x/internal/singletaskrunner"
	"myT-x/internal/taskscheduler"
)

func TestSessionScopedLifecycleParticipantsIncludeAllSessionManagers(t *testing.T) {
	app := NewApp()
	app.taskSchedulerManager = taskscheduler.NewServiceManager(func(sessionName string) taskscheduler.Deps {
		return taskscheduler.Deps{}
	})
	app.singleTaskRunnerManager = singletaskrunner.NewServiceManager(func(sessionName string) singletaskrunner.Deps {
		return singletaskrunner.Deps{SessionName: sessionName}
	})
	app.devpanelService = devpanel.NewService(devpanel.Deps{
		ResolveSessionDir: func(sessionName string, preferWorktree bool) (string, error) { return "", nil },
		IsPathWithinBase:  func(path, base string) bool { return true },
	})
	app.mcpManager = mcp.NewManager(mcp.ManagerConfig{
		Registry: mcp.NewRegistry(),
		EmitFn:   func(string, any) {},
	})

	participants := app.sessionScopedLifecycleParticipants()
	if len(participants) != expectedSessionScopedLifecycleParticipantCount {
		t.Fatalf("participant count = %d, want %d", len(participants), expectedSessionScopedLifecycleParticipantCount)
	}

	gotNames := make([]string, 0, len(participants))
	for _, participant := range participants {
		gotNames = append(gotNames, participant.name)
		if participant.cleanup == nil {
			t.Fatalf("participant %q cleanup should not be nil", participant.name)
		}
		if participant.rename == nil {
			t.Fatalf("participant %q rename should not be nil", participant.name)
		}
	}

	wantNames := []string{"task scheduler", "single task runner", "devpanel", "mcp"}
	for i, wantName := range wantNames {
		if gotNames[i] != wantName {
			t.Fatalf("participant[%d] = %q, want %q", i, gotNames[i], wantName)
		}
	}
}

func TestCleanupSessionScopedParticipantsEmitsDegradedEvent(t *testing.T) {
	app := NewApp()
	app.setRuntimeContext(context.Background())

	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() { runtimeEventsEmitFn = origEmit })

	type emittedEvent struct {
		name    string
		payload any
	}

	var events []emittedEvent
	runtimeEventsEmitFn = func(_ context.Context, name string, payload ...any) {
		var firstPayload any
		if len(payload) > 0 {
			firstPayload = payload[0]
		}
		events = append(events, emittedEvent{name: name, payload: firstPayload})
	}

	app.cleanupSessionScopedParticipants("session-a", []sessionScopedLifecycleParticipant{
		{
			name: "devpanel",
			cleanup: func(string) error {
				return context.DeadlineExceeded
			},
		},
	})

	if len(events) != 1 {
		t.Fatalf("runtime event count = %d, want 1", len(events))
	}
	if events[0].name != "session:cleanup-degraded" {
		t.Fatalf("event name = %q, want %q", events[0].name, "session:cleanup-degraded")
	}

	payload, ok := events[0].payload.(map[string]string)
	if !ok {
		t.Fatalf("event payload type = %T, want map[string]string", events[0].payload)
	}
	if payload["component"] != "devpanel" {
		t.Fatalf("component = %q, want %q", payload["component"], "devpanel")
	}
	if payload["session_name"] != "session-a" {
		t.Fatalf("session_name = %q, want %q", payload["session_name"], "session-a")
	}
	if payload["message"] == "" {
		t.Fatal("message = empty, want cleanup failure details")
	}
}

func TestCleanupSessionScopedParticipantsSkipsDegradedEventWithoutRuntimeContext(t *testing.T) {
	app := NewApp()

	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() { runtimeEventsEmitFn = origEmit })

	emitted := false
	runtimeEventsEmitFn = func(context.Context, string, ...any) {
		emitted = true
	}

	app.cleanupSessionScopedParticipants("session-a", []sessionScopedLifecycleParticipant{
		{
			name: "devpanel",
			cleanup: func(string) error {
				return context.DeadlineExceeded
			},
		},
	})

	if emitted {
		t.Fatal("runtimeEventsEmitFn should not be called when runtime context is nil")
	}
}
