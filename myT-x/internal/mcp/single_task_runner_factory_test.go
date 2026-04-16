package mcp

import (
	"bytes"
	"context"
	"testing"

	"myT-x/internal/singletaskrunner"
	"myT-x/internal/workerutil"
)

func testSingleTaskRunnerDeps(sessionName string) singletaskrunner.Deps {
	return singletaskrunner.Deps{
		IsShuttingDown:   func() bool { return false },
		CheckPaneAlive:   func(string) error { return nil },
		SendMessagePaste: func(string, string) error { return nil },
		SendClearCommand: func(string, string) error { return nil },
		NewContext: func() (context.Context, context.CancelFunc) {
			return context.WithCancel(context.Background())
		},
		LaunchWorker: func(string, context.Context, func(context.Context), workerutil.RecoveryOptions) {},
		BaseRecoveryOptions: func() workerutil.RecoveryOptions {
			return workerutil.RecoveryOptions{}
		},
		SessionName: sessionName,
	}
}

func TestSingleTaskRunnerRuntimeFactoryRejectsNilManager(t *testing.T) {
	factory := singleTaskRunnerRuntimeFactory("session-a", nil)
	_, err := factory(bytes.NewBuffer(nil), &bytes.Buffer{})
	if err == nil {
		t.Fatal("factory error = nil, want nil manager rejection")
	}
}

func TestSingleTaskRunnerRuntimeFactoryCreatesRuntime(t *testing.T) {
	manager := singletaskrunner.NewServiceManager(testSingleTaskRunnerDeps)
	factory := singleTaskRunnerRuntimeFactory("session-a", manager)

	runtime, err := factory(bytes.NewBuffer(nil), &bytes.Buffer{})
	if err != nil {
		t.Fatalf("factory error = %v, want nil", err)
	}
	if runtime == nil {
		t.Fatal("runtime = nil, want runtime instance")
	}
}
