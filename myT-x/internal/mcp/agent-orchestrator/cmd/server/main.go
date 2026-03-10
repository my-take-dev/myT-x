package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	orchestrator "myT-x/internal/mcp/agent-orchestrator"
)

func main() {
	dbPath := flag.String("db", ".myT-x/orchestrator.db", "SQLite database path")
	serverName := flag.String("name", orchestrator.DefaultServerName, "MCP server name")
	serverVersion := flag.String("version", orchestrator.DefaultServerVersion, "MCP server version")
	verbose := flag.Bool("v", false, "enable verbose logging")
	flag.Parse()

	var logger *log.Logger
	if *verbose {
		logger = log.New(os.Stderr, "[orchestrator] ", log.LstdFlags)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cfg := orchestrator.Config{
		DBPath:        *dbPath,
		ServerName:    *serverName,
		ServerVersion: *serverVersion,
		Logger:        logger,
	}

	if err := orchestrator.Run(ctx, cfg); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			log.Println("server stopped:", err)
			os.Exit(0)
		}
		log.Fatalf("server error: %v", err)
	}
}
