package tmux

import (
	"context"
	"fmt"
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

// GetPaneID は自ペインのIDを取得する。
func (e *RealExecutor) GetPaneID(ctx context.Context) (string, error) {
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
	out, err := newTmuxCommand(ctx, args...).Output()
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
	cmd := newTmuxCommand(ctx, "select-pane", "-t", paneID, "-T", title)
	if out, err := cmd.CombinedOutput(); err != nil {
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
	if err := sleepContext(ctx, sendKeysDelay); err != nil {
		return err
	}
	return e.sendEnter(ctx, paneID)
}

func (e *RealExecutor) selectPane(ctx context.Context, paneID string) error {
	cmd := newTmuxCommand(ctx, "select-pane", "-t", paneID)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("select-pane: %w: %s", err, out)
	}
	return nil
}

func (e *RealExecutor) sendText(ctx context.Context, paneID string, text string) error {
	cmd := newTmuxCommand(ctx, "send-keys", "-t", paneID, "-l", "--", text)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("send-keys text: %w: %s", err, out)
	}
	return nil
}

func (e *RealExecutor) sendEnter(ctx context.Context, paneID string) error {
	cmd := newTmuxCommand(ctx, "send-keys", "-t", paneID, "C-m")
	if out, err := cmd.CombinedOutput(); err != nil {
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
		if err := sleepContext(ctx, sendKeysDelay); err != nil {
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
	out, err := newTmuxCommand(ctx, args...).Output()
	if err != nil {
		return "", fmt.Errorf("split-window: %w", err)
	}
	newPaneID := strings.TrimSpace(string(out))
	if err := ValidatePaneID(newPaneID); err != nil {
		return "", fmt.Errorf("split-window returned invalid pane id %q: %w", newPaneID, err)
	}
	return newPaneID, nil
}

// SendKeysPaste はブラケットペーストモードでテキストを送信する。
// ESC[200~ → テキスト（チャンク送信） → ESC[201~ → C-m の順で送信する。
// チャンク送信失敗時もペーストモード終了を保証する。
func (e *RealExecutor) SendKeysPaste(ctx context.Context, paneID string, text string) error {
	if err := ValidatePaneID(paneID); err != nil {
		return err
	}
	text = strings.TrimRight(text, "\n\r")

	if err := e.selectPane(ctx, paneID); err != nil {
		return err
	}

	// ペーストモード開始
	if err := e.sendText(ctx, paneID, "\x1b[200~"); err != nil {
		return fmt.Errorf("send paste-start: %w", err)
	}

	// テキスト送信（チャンク）
	runes := []rune(text)
	var sendErr error
	for i := 0; i < len(runes); i += maxSendKeysLength {
		end := min(i+maxSendKeysLength, len(runes))
		chunk := string(runes[i:end])
		if err := e.sendText(ctx, paneID, chunk); err != nil {
			sendErr = fmt.Errorf("send paste-text: %w", err)
			break
		}
		if err := sleepContext(ctx, sendKeysDelay); err != nil {
			sendErr = err
			break
		}
	}

	// ペーストモード終了（テキスト送信失敗時も必ず実行）
	if endErr := e.sendText(ctx, paneID, "\x1b[201~"); endErr != nil {
		if sendErr != nil {
			return sendErr
		}
		return fmt.Errorf("send paste-end: %w", endErr)
	}

	if sendErr != nil {
		return sendErr
	}

	return e.sendEnter(ctx, paneID)
}

// CapturePaneOutput はペインの表示内容を取得する。
func (e *RealExecutor) CapturePaneOutput(ctx context.Context, paneID string, lines int) (string, error) {
	if err := ValidatePaneID(paneID); err != nil {
		return "", err
	}
	arg := fmt.Sprintf("-%d", lines)
	cmd := newTmuxCommand(ctx, "capture-pane", "-t", paneID, "-p", "-S", arg)
	out, err := cmd.CombinedOutput()
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
