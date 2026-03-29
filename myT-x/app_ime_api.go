package main

// RecoverIMEWindowFocus replays the native window raise path so WebView2 can
// rebuild its focus state without requiring an application restart.
func (a *App) RecoverIMEWindowFocus() error {
	return a.bringWindowToFront()
}
