package main

import (
	"embed"
	"errors"
	"log/slog"

	"myT-x/internal/ipc"
	"myT-x/internal/singleinstance"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Single-instance check BEFORE any Wails/WebView2 initialization.
	// Two simultaneous instances corrupt WebView2 browser process IME state.
	mutexLock, err := singleinstance.TryLock(singleinstance.DefaultMutexName())
	if errors.Is(err, singleinstance.ErrAlreadyRunning) {
		slog.Info("[DEBUG-SINGLE] another instance is already running, signaling activation")
		if _, sendErr := ipc.Send("", ipc.TmuxRequest{Command: "activate-window"}); sendErr != nil {
			slog.Warn("[DEBUG-SINGLE] failed to signal existing instance", "error", sendErr)
		}
		return
	}
	if err != nil {
		// Mutex creation failed for unexpected reason. Continue startup defensively.
		slog.Warn("[DEBUG-SINGLE] mutex creation failed, proceeding without single-instance guard", "error", err)
	}
	if mutexLock != nil {
		defer func() {
			if releaseErr := mutexLock.Release(); releaseErr != nil {
				slog.Warn("[DEBUG-SINGLE] mutex release failed", "error", releaseErr)
			}
		}()
	}

	app := NewApp()

	err = wails.Run(&options.App{
		Title:     "myT-x v0.0.7",
		Width:     1440,
		Height:    900,
		MinWidth:  980,
		MinHeight: 620,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 10, G: 16, B: 22, A: 1},
		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop: true,
		},
		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
		Bind: []any{
			app,
		},
	})

	if err != nil {
		slog.Error("[DEBUG-SINGLE] wails run failed", "error", err)
	}
}
