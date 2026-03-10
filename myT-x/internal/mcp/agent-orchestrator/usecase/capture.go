package usecase

import (
	"context"
	"log"
	"strings"

	"myT-x/internal/mcp/agent-orchestrator/domain"
)

// CapturePaneCmd はペインキャプチャコマンド。
type CapturePaneCmd struct {
	AgentName string
	Lines     int
}

// CapturePaneResult はペインキャプチャ結果。
type CapturePaneResult struct {
	AgentName string
	PaneID    string
	Lines     int
	Content   string
	Warning   string
}

// CaptureService はペインキャプチャを管理する。
type CaptureService struct {
	agents   domain.AgentRepository
	capturer domain.PaneCapturer
	resolver domain.SelfPaneResolver
	logger   *log.Logger
}

// NewCaptureService は CaptureService を構築する。
func NewCaptureService(
	agents domain.AgentRepository,
	capturer domain.PaneCapturer,
	resolver domain.SelfPaneResolver,
	logger *log.Logger,
) *CaptureService {
	return &CaptureService{
		agents:   agents,
		capturer: capturer,
		resolver: resolver,
		logger:   ensureLogger(logger),
	}
}

// Capture はペインの表示内容を取得する。
func (s *CaptureService) Capture(ctx context.Context, cmd CapturePaneCmd) (CapturePaneResult, error) {
	logf(s.logger, "capture_pane start agent=%s lines=%d", cmd.AgentName, cmd.Lines)
	caller, err := resolveCaller(ctx, s.resolver, s.agents, s.logger)
	if err != nil {
		return CapturePaneResult{}, err
	}
	logf(s.logger, "capture_pane caller=%s target=%s", caller.Name, cmd.AgentName)

	agent, err := s.agents.GetAgent(ctx, cmd.AgentName)
	if err != nil {
		return CapturePaneResult{}, operationError(s.logger, "target agent is not available", err)
	}
	logf(s.logger, "capture_pane target resolved agent=%s pane_id=%s", agent.Name, agent.PaneID)

	content, err := s.capturer.CapturePaneOutput(ctx, agent.PaneID, cmd.Lines)
	if err != nil {
		warning := "pane capture failed"
		if isUnsupportedPaneCapture(err) {
			warning = "pane capture is unavailable on the current tmux shim"
		}
		logf(s.logger, "capture pane %s (%s) warning=%s error=%v", cmd.AgentName, agent.PaneID, warning, err)
		return CapturePaneResult{
			AgentName: cmd.AgentName,
			PaneID:    agent.PaneID,
			Lines:     cmd.Lines,
			Warning:   warning,
		}, nil
	}
	logf(s.logger, "capture_pane success agent=%s pane_id=%s bytes=%d", agent.Name, agent.PaneID, len(content))

	return CapturePaneResult{
		AgentName: cmd.AgentName,
		PaneID:    agent.PaneID,
		Lines:     cmd.Lines,
		Content:   content,
	}, nil
}

func isUnsupportedPaneCapture(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "access is denied") ||
		strings.Contains(msg, "not supported") ||
		strings.Contains(msg, "unsupported")
}
