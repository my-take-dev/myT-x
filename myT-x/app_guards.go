package main

import (
	"errors"

	"myT-x/internal/tmux"
)

func (a *App) requireSessions() (*tmux.SessionManager, error) {
	if a.sessions == nil {
		return nil, errors.New("session manager is unavailable")
	}
	return a.sessions, nil
}

func (a *App) requireRouter() (*tmux.CommandRouter, error) {
	if a.router == nil {
		return nil, errors.New("router is unavailable")
	}
	return a.router, nil
}

func (a *App) requireSessionsAndRouter() (*tmux.SessionManager, *tmux.CommandRouter, error) {
	sessions, err := a.requireSessions()
	if err != nil {
		return nil, nil, err
	}
	router, err := a.requireRouter()
	if err != nil {
		return nil, nil, err
	}
	return sessions, router, nil
}
