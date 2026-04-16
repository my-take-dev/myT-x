package main

import "context"

func (a *App) registerSetupWorker(cancel context.CancelFunc) (func(), bool) {
	a.setupCancelMu.Lock()
	defer a.setupCancelMu.Unlock()

	if a.shuttingDown.Load() {
		if cancel != nil {
			cancel()
		}
		return func() {}, false
	}

	a.setupWG.Add(1)
	if cancel == nil {
		return func() {
			a.setupWG.Done()
		}, true
	}

	id := a.nextSetupCancelID.Add(1)
	a.setupCancels[id] = cancel

	return func() {
		a.setupCancelMu.Lock()
		delete(a.setupCancels, id)
		a.setupCancelMu.Unlock()
		a.setupWG.Done()
	}, true
}

func (a *App) trackSetupCancel(cancel context.CancelFunc) func() {
	if cancel == nil {
		return func() {}
	}

	id := a.nextSetupCancelID.Add(1)

	a.setupCancelMu.Lock()
	a.setupCancels[id] = cancel
	a.setupCancelMu.Unlock()

	return func() {
		a.setupCancelMu.Lock()
		delete(a.setupCancels, id)
		a.setupCancelMu.Unlock()
	}
}

func (a *App) cancelTrackedSetupWorkers() int {
	a.setupCancelMu.Lock()
	cancels := make([]context.CancelFunc, 0, len(a.setupCancels))
	for _, cancel := range a.setupCancels {
		cancels = append(cancels, cancel)
	}
	a.setupCancelMu.Unlock()

	for _, cancel := range cancels {
		cancel()
	}
	return len(cancels)
}
