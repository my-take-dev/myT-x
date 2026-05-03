package main

// LoadSessionMemo loads the memo for the provided session from app-config session storage.
func (a *App) LoadSessionMemo(sessionName string) (string, error) {
	return a.sessionMemoService.Load(sessionName)
}

// SaveSessionMemo saves the memo for the provided session into app-config session storage.
func (a *App) SaveSessionMemo(sessionName string, memo string) error {
	return a.sessionMemoService.Save(sessionName, memo)
}
