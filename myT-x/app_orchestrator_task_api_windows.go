//go:build windows

package main

import (
	"fmt"
	"log/slog"
	"syscall"
	"unsafe"

	"myT-x/internal/tmux"

	"golang.org/x/sys/windows"
)

// GetPaneProcessStatus はセッション内の全ペインのプロセス実行状態を返す。
// シェルプロセスに子プロセスが存在する場合に HasChildProcess=true を返す。
func (a *App) GetPaneProcessStatus(sessionName string) ([]PaneProcessStatus, error) {
	sessions, err := a.requireSessions()
	if err != nil {
		return nil, err
	}

	panePIDs, err := sessions.GetSessionPanePIDs(sessionName)
	if err != nil {
		return nil, fmt.Errorf("[DEBUG-canvas] get pane PIDs: %w", err)
	}

	// プロセスツリーのスナップショットを一度だけ取得して全ペインを評価する。
	childPIDs, snapshotErr := buildChildPIDSet(panePIDs)
	if snapshotErr != nil {
		// I-10: スナップショット取得失敗をログに記録
		slog.Warn("[WARN-canvas] process snapshot failed, returning all HasChildProcess=false",
			"error", snapshotErr)
		result := make([]PaneProcessStatus, len(panePIDs))
		for i, p := range panePIDs {
			result[i] = PaneProcessStatus{PaneID: p.PaneID}
		}
		return result, nil
	}

	result := make([]PaneProcessStatus, len(panePIDs))
	for i, p := range panePIDs {
		result[i] = PaneProcessStatus{
			PaneID:          p.PaneID,
			HasChildProcess: childPIDs[uint32(p.PID)],
		}
	}
	return result, nil
}

// buildChildPIDSet は指定ペインPIDのいずれかを親に持つ子プロセスが存在するか判定する。
// 戻り値は shellPID → true のマップ（子プロセスが存在するPIDのみ true）。
func buildChildPIDSet(panePIDs []tmux.PanePIDInfo) (map[uint32]bool, error) {
	if len(panePIDs) == 0 {
		return nil, nil
	}

	// 対象PIDのセットを構築
	targetPIDs := make(map[uint32]struct{}, len(panePIDs))
	for _, p := range panePIDs {
		if p.PID > 0 {
			targetPIDs[uint32(p.PID)] = struct{}{}
		}
	}
	if len(targetPIDs) == 0 {
		return nil, nil
	}

	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, fmt.Errorf("[DEBUG-canvas] CreateToolhelp32Snapshot: %w", err)
	}
	defer windows.CloseHandle(snap)

	result := make(map[uint32]bool, len(targetPIDs))

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	err = windows.Process32First(snap, &entry)

	// M-3: Process32Firstが ERROR_NO_MORE_FILES 以外のエラーを返した場合にログ出力
	if err != nil {
		if err != syscall.ERROR_NO_MORE_FILES {
			slog.Warn("[WARN-canvas] Process32First failed with unexpected error", "error", err)
		}
		return result, nil
	}

	for err == nil {
		if _, ok := targetPIDs[entry.ParentProcessID]; ok {
			if entry.ProcessID != entry.ParentProcessID {
				result[entry.ParentProcessID] = true
			}
		}
		err = windows.Process32Next(snap, &entry)
	}

	return result, nil
}
