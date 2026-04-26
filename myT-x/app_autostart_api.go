package main

import (
	"errors"
	"fmt"
	"strings"

	"myT-x/internal/config"
)

// StartAutoStartCommand splits a new pane and launches the configured command.
func (a *App) StartAutoStartCommand(paneID string, entry config.AutoStartCommand) (string, error) {
	paneID = strings.TrimSpace(paneID)
	if paneID == "" {
		return "", errors.New("pane id is required")
	}

	normalized, ok := config.NormalizeAutoStartCommand(entry)
	if !ok {
		return "", errors.New("auto start command is required")
	}

	router, err := a.requireRouter()
	if err != nil {
		return "", err
	}

	newPaneID, err := a.SplitPane(paneID, true)
	if err != nil {
		return "", err
	}

	if err := a.sendKeys.schedulerSendMessage(router, newPaneID, buildAutoStartCommandLine(normalized)); err != nil {
		if rollbackErr := a.KillPane(newPaneID); rollbackErr != nil {
			return "", errors.Join(
				fmt.Errorf("auto start command send failed: %w", err),
				fmt.Errorf("auto start pane rollback failed: %w", rollbackErr),
			)
		}
		return "", fmt.Errorf("auto start command send failed: %w", err)
	}
	return newPaneID, nil
}

func buildAutoStartCommandLine(entry config.AutoStartCommand) string {
	command := strings.TrimSpace(entry.Command)
	args := strings.TrimSpace(entry.Args)
	if args == "" {
		return command
	}
	return command + " " + args
}
