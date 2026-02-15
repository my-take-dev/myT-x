package main

import "myT-x/internal/config"

// getConfigSnapshot returns a deep-copied config protected by cfgMu.
// All read access to App.cfg should go through this helper.
func (a *App) getConfigSnapshot() config.Config {
	a.cfgMu.RLock()
	defer a.cfgMu.RUnlock()
	return config.Clone(a.cfg)
}

// setConfigSnapshot stores a deep-copied config protected by cfgMu.
// All write access to App.cfg should go through this helper.
func (a *App) setConfigSnapshot(cfg config.Config) {
	a.cfgMu.Lock()
	a.cfg = config.Clone(cfg)
	a.cfgMu.Unlock()
}
