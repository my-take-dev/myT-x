package main

import (
	"context"
	"testing"
	"time"
)

func TestRegisterSetupWorkerMakesCancelVisibleBeforeShutdownWait(t *testing.T) {
	app := NewApp()
	app.setRuntimeContext(context.Background())

	canceled := make(chan struct{}, 1)
	release, shouldStart := app.registerSetupWorker(func() {
		select {
		case canceled <- struct{}{}:
		default:
		}
	})
	if !shouldStart {
		t.Fatal("registerSetupWorker() should accept workers before shutdown")
	}

	done := make(chan struct{})
	go func() {
		app.shutdown(context.Background())
		close(done)
	}()

	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("shutdown() did not cancel the tracked setup worker")
	}

	select {
	case <-done:
		t.Fatal("shutdown() returned before the setup worker released itself")
	case <-time.After(100 * time.Millisecond):
	}

	release()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown() did not finish after the setup worker release")
	}
}

func TestRegisterSetupWorkerRejectsNewWorkersDuringShutdown(t *testing.T) {
	app := NewApp()
	app.shuttingDown.Store(true)

	cancelCalled := false
	release, shouldStart := app.registerSetupWorker(func() {
		cancelCalled = true
	})
	if shouldStart {
		t.Fatal("registerSetupWorker() should reject workers during shutdown")
	}
	if !cancelCalled {
		t.Fatal("registerSetupWorker() should cancel skipped workers immediately")
	}

	release()

	done := make(chan struct{})
	go func() {
		app.setupWG.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("setupWG should not retain a skipped worker")
	}
}
