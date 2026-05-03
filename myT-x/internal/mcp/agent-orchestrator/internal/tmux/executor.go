package tmux

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"myT-x/internal/mcp/agent-orchestrator/domain"
	"myT-x/internal/procutil"
)

const (
	maxSendKeysLength = 500
	sendKeysDelay     = 300 * time.Millisecond
)

// RealExecutor は実際の tmux コマンドを実行する。
type RealExecutor struct {
	// SessionName は空の場合、セッション無指定（全セッション対象）。
	SessionName string
	// SessionAllPanes が false（デフォルト）かつ SessionName が非空の場合、
	// ListPanes は自セッションのペインのみ返す。
	SessionAllPanes bool

	hooks *executorHooks
}

type executorHooks struct {
	combinedOutput func(ctx context.Context, args ...string) ([]byte, error)
	output         func(ctx context.Context, args ...string) ([]byte, error)
	sleep          func(ctx context.Context, delay time.Duration) error
}

// NewExecutor は新しい RealExecutor を返す（全セッション対象、既存互換）。
func NewExecutor() *RealExecutor {
	return &RealExecutor{}
}

// NewSessionAwareExecutor はセッションスコープ対応の RealExecutor を返す。
func NewSessionAwareExecutor(sessionName string, allPanes bool) *RealExecutor {
	return &RealExecutor{SessionName: sessionName, SessionAllPanes: allPanes}
}

func newTmuxCommand(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "tmux", args...)
	procutil.HideWindow(cmd)
	return cmd
}

func (e *RealExecutor) combinedOutput(ctx context.Context, args ...string) ([]byte, error) {
	if e.hooks != nil && e.hooks.combinedOutput != nil {
		return e.hooks.combinedOutput(ctx, args...)
	}
	return newTmuxCommand(ctx, args...).CombinedOutput()
}

func (e *RealExecutor) output(ctx context.Context, args ...string) ([]byte, error) {
	if e.hooks != nil && e.hooks.output != nil {
		return e.hooks.output(ctx, args...)
	}
	return newTmuxCommand(ctx, args...).Output()
}

func (e *RealExecutor) sleep(ctx context.Context, delay time.Duration) error {
	if e.hooks != nil && e.hooks.sleep != nil {
		return e.hooks.sleep(ctx, delay)
	}
	return sleepContext(ctx, delay)
}

// GetPaneID は自ペインのIDを取得する。
func (e *RealExecutor) GetPaneID(_ context.Context) (string, error) {
	paneID := strings.TrimSpace(os.Getenv("TMUX_PANE"))
	if paneID == "" {
		return "", fmt.Errorf("get pane id: TMUX_PANE is unavailable")
	}
	if err := ValidatePaneID(paneID); err != nil {
		return "", fmt.Errorf("get pane id: %w", err)
	}
	return paneID, nil
}

// ListPanes はペイン情報を取得する。
// SessionAllPanes=false かつ SessionName が非空の場合、2層フィルタで自セッションのみ返す:
//   - Layer 1: tmux コマンドレベルで -s -t <session> によるフィルタ（データ量削減）
//   - Layer 2: アプリケーションレベルで PaneInfo.Session による防御的フィルタ
//
// SessionName が空、または SessionAllPanes=true の場合は -a で全セッションを返す。
func (e *RealExecutor) ListPanes(ctx context.Context) ([]domain.PaneInfo, error) {
	args := []string{"list-panes"}
	if !e.SessionAllPanes && e.SessionName != "" {
		// Layer 1: tmux コマンドレベルで自セッションに絞る
		args = append(args, "-s", "-t", e.SessionName)
	} else {
		args = append(args, "-a")
	}
	args = append(args, "-F", "#{pane_id}\t#{pane_title}\t#{session_name}\t#{window_index}")
	out, err := e.output(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("list panes: %w", err)
	}
	panes := ParseListPanesOutput(string(out))
	// Layer 2: アプリケーションレベルでセッションフィルタ（防御的）
	if !e.SessionAllPanes && e.SessionName != "" {
		panes = filterPanesBySession(panes, e.SessionName)
	}
	return panes, nil
}

// filterPanesBySession は PaneInfo.Session が一致するペインのみ返す。
func filterPanesBySession(panes []domain.PaneInfo, sessionName string) []domain.PaneInfo {
	filtered := make([]domain.PaneInfo, 0, len(panes))
	for _, p := range panes {
		if strings.EqualFold(p.Session, sessionName) {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

// SetPaneTitle はペインタイトルを設定する。
func (e *RealExecutor) SetPaneTitle(ctx context.Context, paneID string, title string) error {
	if err := ValidatePaneID(paneID); err != nil {
		return err
	}
	if out, err := e.combinedOutput(ctx, "select-pane", "-t", paneID, "-T", title); err != nil {
		return fmt.Errorf("set pane title: %w: %s", err, out)
	}
	return nil
}

// SendKeys は select-pane → send-keys -l → send-keys C-m の3ステップで確実に送信する。
func (e *RealExecutor) SendKeys(ctx context.Context, paneID string, text string) error {
	if err := ValidatePaneID(paneID); err != nil {
		return err
	}
	text = strings.TrimRight(text, "\n\r")

	if err := e.selectPane(ctx, paneID); err != nil {
		return err
	}

	if len([]rune(text)) > maxSendKeysLength {
		return e.sendKeysChunked(ctx, paneID, text)
	}

	if err := e.sendText(ctx, paneID, text); err != nil {
		return err
	}
	// Keep text-send and paste-send ordering symmetric: wait for tmux to deliver
	// the payload before Enter so C-m cannot overtake pending text delivery.
	if err := e.sleep(ctx, sendKeysDelay); err != nil {
		return err
	}
	return e.sendEnter(ctx, paneID)
}

func (e *RealExecutor) selectPane(ctx context.Context, paneID string) error {
	if out, err := e.combinedOutput(ctx, "select-pane", "-t", paneID); err != nil {
		return fmt.Errorf("select-pane: %w: %s", err, out)
	}
	return nil
}

func (e *RealExecutor) sendText(ctx context.Context, paneID string, text string) error {
	if out, err := e.combinedOutput(ctx, "send-keys", "-t", paneID, "-l", "--", text); err != nil {
		return fmt.Errorf("send-keys text: %w: %s", err, out)
	}
	return nil
}

func (e *RealExecutor) sendEnter(ctx context.Context, paneID string) error {
	if out, err := e.combinedOutput(ctx, "send-keys", "-t", paneID, "C-m"); err != nil {
		return fmt.Errorf("send-keys C-m: %w: %s", err, out)
	}
	return nil
}

func (e *RealExecutor) sendKeysChunked(ctx context.Context, paneID string, text string) error {
	runes := []rune(text)
	for i := 0; i < len(runes); i += maxSendKeysLength {
		end := min(i+maxSendKeysLength, len(runes))
		chunk := string(runes[i:end])
		if err := e.sendText(ctx, paneID, chunk); err != nil {
			return err
		}
		if err := e.sleep(ctx, sendKeysDelay); err != nil {
			return err
		}
	}
	return e.sendEnter(ctx, paneID)
}

// SplitPane は既存ペインを分割して新ペインを作成し、新ペインIDを返す。
// horizontal=true で左右分割（-h）、false で上下分割。
func (e *RealExecutor) SplitPane(ctx context.Context, targetPaneID string, horizontal bool) (string, error) {
	if err := ValidatePaneID(targetPaneID); err != nil {
		return "", err
	}
	args := []string{"split-window", "-t", targetPaneID}
	if horizontal {
		args = append(args, "-h")
	}
	args = append(args, "-P", "-F", "#{pane_id}")
	out, err := e.output(ctx, args...)
	if err != nil {
		return "", fmt.Errorf("split-window: %w", err)
	}
	newPaneID := strings.TrimSpace(string(out))
	if err := ValidatePaneID(newPaneID); err != nil {
		return "", fmt.Errorf("split-window returned invalid pane id %q: %w", newPaneID, err)
	}
	return newPaneID, nil
}

// SendKeysPaste loads the content into a tmux buffer, pastes it in one shot,
// then sends Enter. paste-buffer -p adds bracketed-paste markers when the
// target application requests them.
func (e *RealExecutor) SendKeysPaste(ctx context.Context, paneID string, text string) error {
	if err := ValidatePaneID(paneID); err != nil {
		return err
	}
	text = strings.TrimRight(text, "\n\r")

	if err := e.selectPane(ctx, paneID); err != nil {
		return err
	}
	bufferName := fmt.Sprintf("orch-paste-%d", time.Now().UTC().UnixNano())
	bufferFile, err := os.CreateTemp("", "orch-paste-*.txt")
	if err != nil {
		return fmt.Errorf("create paste temp file: %w", err)
	}
	bufferPath := bufferFile.Name()
	defer func() {
		if err := os.Remove(bufferPath); err != nil && !os.IsNotExist(err) {
			slog.Debug("[DEBUG-ORCH-TMUX] failed to remove paste temp file", "path", bufferPath, "error", err)
		}
	}()
	if _, err := bufferFile.WriteString(text); err != nil {
		if closeErr := bufferFile.Close(); closeErr != nil {
			return fmt.Errorf("write paste temp file: %w; close paste temp file: %v", err, closeErr)
		}
		return fmt.Errorf("write paste temp file: %w", err)
	}
	if err := bufferFile.Close(); err != nil {
		return fmt.Errorf("close paste temp file: %w", err)
	}

	if err := e.loadBufferFromFile(ctx, bufferName, bufferPath); err != nil {
		return err
	}
	if err := e.pasteBuffer(ctx, paneID, bufferName); err != nil {
		if cleanupErr := e.deleteBuffer(context.Background(), bufferName); cleanupErr != nil {
			slog.Warn("[WARN-ORCH-TMUX] failed to delete paste buffer",
				"bufferName", bufferName,
				"paneID", paneID,
				"error", cleanupErr,
			)
			return errors.Join(err, fmt.Errorf("delete paste buffer %q: %w", bufferName, cleanupErr))
		}
		return err
	}
	// paste-buffer can complete asynchronously relative to the target pane.
	// Wait before Enter so C-m cannot arrive before the pasted payload.
	if err := e.sleep(ctx, sendKeysDelay); err != nil {
		return err
	}

	return e.sendEnter(ctx, paneID)
}

// CapturePaneOutput はペインの表示内容を取得する。
func (e *RealExecutor) CapturePaneOutput(ctx context.Context, paneID string, lines int) (string, error) {
	if err := ValidatePaneID(paneID); err != nil {
		return "", err
	}
	arg := fmt.Sprintf("-%d", lines)
	out, err := e.combinedOutput(ctx, "capture-pane", "-t", paneID, "-p", "-S", arg)
	if err != nil {
		return "", fmt.Errorf("capture-pane: %w: %s", err, out)
	}
	return string(out), nil
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (e *RealExecutor) loadBufferFromFile(ctx context.Context, bufferName string, path string) error {
	if out, err := e.combinedOutput(ctx, "load-buffer", "-b", bufferName, path); err != nil {
		return fmt.Errorf("load-buffer: %w: %s", err, out)
	}
	return nil
}

func (e *RealExecutor) pasteBuffer(ctx context.Context, paneID string, bufferName string) error {
	if out, err := e.combinedOutput(ctx, "paste-buffer", "-d", "-p", "-b", bufferName, "-t", paneID); err != nil {
		return fmt.Errorf("paste-buffer: %w: %s", err, out)
	}
	return nil
}

func (e *RealExecutor) deleteBuffer(ctx context.Context, bufferName string) error {
	if out, err := e.combinedOutput(ctx, "delete-buffer", "-b", bufferName); err != nil {
		return fmt.Errorf("delete-buffer: %w: %s", err, out)
	}
	return nil
}
