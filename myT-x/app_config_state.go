package main

import "myT-x/internal/config"

// getConfigSnapshot returns a deep-copied config protected by cfgMu.
// All read access to App.cfg should go through this helper.
func (a *App) getConfigSnapshot() config.Config {
	a.cfgMu.RLock()
	defer a.cfgMu.RUnlock()
	return config.Clone(a.cfg)
}

// getConfigInternalUnsafe returns the current config without cloning.
//
// WARNING: Reference-type fields (maps/slices/pointers) are shared with internal
// state. Callers MUST NOT modify the returned value or its nested fields.
// Use only for short-lived read-only access on the current goroutine.
// Callers that retain values or pass config data to long-lived goroutines must
// use getConfigSnapshot instead.
//
// Call sites: currently used only in test code (app_config_state_test.go) to verify
// that reference-type fields share internal state. No production call sites exist.
// If a production call site is added, document it here and audit the caller for
// accidental mutation.
func (a *App) getConfigInternalUnsafe() config.Config {
	a.cfgMu.RLock()
	defer a.cfgMu.RUnlock()
	return a.cfg
}

// setConfigSnapshot stores a deep-copied config protected by cfgMu.
// All write access to App.cfg should go through this helper.
func (a *App) setConfigSnapshot(cfg config.Config) {
	a.cfgMu.Lock()
	a.cfg = config.Clone(cfg)
	a.cfgMu.Unlock()
}
