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
type RealExecutor struct{}

// NewExecutor は新しい RealExecutor を返す。
func NewExecutor() *RealExecutor {
	return &RealExecutor{}
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

// ListPanes は全セッションの全ペイン情報を取得する。
func (e *RealExecutor) ListPanes(ctx context.Context) ([]domain.PaneInfo, error) {
	out, err := newTmuxCommand(ctx, "list-panes", "-a", "-F", "#{pane_id}\t#{pane_title}\t#{session_name}\t#{window_index}").Output()
	if err != nil {
		return nil, fmt.Errorf("list panes: %w", err)
	}
	return ParseListPanesOutput(string(out)), nil
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
