package main

import (
	"embed"
	"errors"
	"log/slog"
	"os"
	"path/filepath"

	"myT-x/internal/ipc"
	"myT-x/internal/singleinstance"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	os.Exit(run())
}

func run() int {
	if handled, exitCode := runMCPCLIMode(os.Args[1:]); handled {
		return exitCode
	}

	// Single-instance check BEFORE any Wails/WebView2 initialization.
	// Two simultaneous instances corrupt WebView2 browser process IME state.
	mutexLock, err := singleinstance.TryLock(singleinstance.DefaultMutexName())
	if errors.Is(err, singleinstance.ErrAlreadyRunning) {
		slog.Debug("[DEBUG-SINGLE] another instance is already running, signaling activation")
		if _, sendErr := ipc.Send("", ipc.TmuxRequest{Command: "activate-window"}); sendErr != nil {
			slog.Warn("[WARN-SINGLE] failed to signal existing instance", "error", sendErr)
		}
		return 0
	}
	if err != nil {
		// Mutex creation failed for unexpected reason. Continue startup defensively.
		slog.Warn("[WARN-SINGLE] mutex creation failed, proceeding without single-instance guard", "error", err)
	}
	if mutexLock != nil {
		defer func() {
			if releaseErr := mutexLock.Release(); releaseErr != nil {
				slog.Warn("[WARN-SINGLE] mutex release failed", "error", releaseErr)
			}
		}()
	}

	app := NewApp()

	// Isolate the WebView2 browser process from Edge and other WebView2 apps.
	// Each unique WebviewUserDataPath creates a separate process group with its
	// own TSF (Text Services Framework) context, preventing process-level IME
	// state corruption that causes Japanese IME conversion failure.
	var windowsOpts *windows.Options
	if appData := os.Getenv("APPDATA"); appData != "" {
		windowsOpts = &windows.Options{
			WebviewUserDataPath: filepath.Join(appData, "myT-x", "WebView2"),
		}
	} else {
		slog.Error("[ERROR-IME] APPDATA not set, WebView2 process isolation disabled")
	}

	err = wails.Run(&options.App{
		Title:     "myT-x v1.0.3",
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
		Windows:    windowsOpts,
		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
		Bind: []any{
			app,
		},
	})

	if err != nil {
		slog.Error("[ERROR-SINGLE] wails run failed", "error", err)
		return 1
	}
	return 0
}
