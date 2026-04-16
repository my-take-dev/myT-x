package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	singletaskrunnermcp "myT-x/internal/mcp/single-task-runner"
	"myT-x/internal/singletaskrunner"
	"myT-x/internal/workerutil"
)

func main() {
	serverName := flag.String("name", singletaskrunnermcp.DefaultServerName, "MCP server name")
	serverVersion := flag.String("version", singletaskrunnermcp.DefaultServerVersion, "MCP server version")
	verbose := flag.Bool("v", false, "enable verbose logging")
	flag.Parse()

	var logger *log.Logger
	if *verbose {
		logger = log.New(os.Stderr, "[single-task-runner] ", log.LstdFlags)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cfg := singletaskrunnermcp.Config{
		ServerName:    *serverName,
		ServerVersion: *serverVersion,
		Logger:        logger,
		Service: singletaskrunner.NewService(singletaskrunner.Deps{
			CheckPaneAlive: func(string) error { return errors.New("standalone mode requires an injected service implementation") },
			SendMessagePaste: func(string, string) error {
				return errors.New("standalone mode requires an injected service implementation")
			},
			SendClearCommand: func(string, string) error {
				return errors.New("standalone mode requires an injected service implementation")
			},
			NewContext: func() (context.Context, context.CancelFunc) {
				return context.WithCancel(context.Background())
			},
			LaunchWorker: func(_ string, _ context.Context, _ func(context.Context), _ workerutil.RecoveryOptions) {},
			BaseRecoveryOptions: func() workerutil.RecoveryOptions {
				return workerutil.RecoveryOptions{MaxRetries: 0}
			},
			SessionName: "standalone",
		}),
	}

	if err := singletaskrunnermcp.Run(ctx, cfg); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			log.Println("server stopped:", err)
			os.Exit(0)
		}
		log.Fatalf("server error: %v", err)
	}
}
