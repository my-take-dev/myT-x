//go:build !windows

package main

import (
	"fmt"

	"myT-x/internal/tmux"
)

// GetPaneProcessStatus はセッション内の全ペインのプロセス実行状態を返す。
// Windows以外の環境ではプロセスツリー取得APIが異なるため、全ペインの HasChildProcess=false を返す。
func (a *App) GetPaneProcessStatus(sessionName string) ([]PaneProcessStatus, error) {
	sessions, err := a.requireSessions()
	if err != nil {
		return nil, err
	}

	panePIDs, err := sessions.GetSessionPanePIDs(sessionName)
	if err != nil {
		return nil, fmt.Errorf("[DEBUG-canvas] get pane PIDs: %w", err)
	}

	result := make([]PaneProcessStatus, len(panePIDs))
	for i, p := range panePIDs {
		result[i] = PaneProcessStatus{PaneID: p.PaneID}
	}
	return result, nil
}

// buildChildPIDSet はWindows以外の環境ではスタブとして nil を返す。
func buildChildPIDSet(_ []tmux.PanePIDInfo) (map[uint32]bool, error) {
	return nil, nil
}
