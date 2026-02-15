package main

import "context"

func (a *App) setRuntimeContext(ctx context.Context) {
	a.ctxMu.Lock()
	a.ctx = ctx
	a.ctxMu.Unlock()
}

func (a *App) runtimeContext() context.Context {
	a.ctxMu.RLock()
	ctx := a.ctx
	a.ctxMu.RUnlock()
	return ctx
}
