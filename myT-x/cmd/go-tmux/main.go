package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"myT-x/internal/ipc"
	"myT-x/internal/tmux"
)

func main() {
	logger := log.New(os.Stdout, "[myT-x] ", log.LstdFlags|log.Lmsgprefix)

	sessions := tmux.NewSessionManager()
	emitter := tmux.EventEmitterFunc(func(name string, payload any) {
		logger.Printf("event=%s payload=%v", name, payload)
	})

	router := tmux.NewCommandRouter(sessions, emitter, tmux.RouterOptions{
		DefaultShell: "powershell.exe",
		PipeName:     ipc.DefaultPipeName(),
		HostPID:      os.Getpid(),
	})

	server := ipc.NewPipeServer(router.PipeName(), router)
	if err := server.Start(); err != nil {
		logger.Fatalf("failed to start pipe server: %v", err)
	}
	logger.Printf("pipe server listening on %s", server.PipeName())

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig

	logger.Printf("shutdown started at %s", time.Now().Format(time.RFC3339))
	if err := server.Stop(); err != nil {
		logger.Printf("failed to stop pipe server cleanly: %v", err)
	}
	sessions.Close()
}
