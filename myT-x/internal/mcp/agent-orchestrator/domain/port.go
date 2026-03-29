package domain

import (
	"context"
	"errors"
)

// ErrNotFound はエンティティが見つからない場合のエラー。
var ErrNotFound = errors.New("not found")

// AgentRepository はエージェントの永続化操作を定義する。
type AgentRepository interface {
	UpsertAgent(ctx context.Context, agent Agent) error
	GetAgent(ctx context.Context, name string) (Agent, error)
	GetAgentByPaneID(ctx context.Context, paneID string) (Agent, error)
	ListAgents(ctx context.Context) ([]Agent, error)
	DeleteAgentsByPaneID(ctx context.Context, paneID string) error
}

// TaskRepository はタスクの永続化操作を定義する。
type TaskRepository interface {
	CreateTask(ctx context.Context, task Task) error
	GetTask(ctx context.Context, taskID string) (Task, error)
	ListTasks(ctx context.Context, filter TaskFilter) ([]Task, error)
	CompleteTask(ctx context.Context, taskID string, responseID string, completedAt string) error
	MarkTaskFailed(ctx context.Context, taskID string) error
	AbandonTasksByPaneID(ctx context.Context, paneID string) error
	EndSessionByInstanceID(ctx context.Context, instanceID string) error
	GetTaskBySendMessageID(ctx context.Context, sendMessageID string) (Task, error)
}

// MessageRepository はタスクメッセージの永続化操作を定義する。
type MessageRepository interface {
	SaveMessage(ctx context.Context, msg TaskMessage) error
	SaveResponse(ctx context.Context, msg TaskMessage) error
	GetMessage(ctx context.Context, id string) (TaskMessage, error)
}

// InstanceRegistry は MCP インスタンスの生存追跡とstaleデータのクリーンアップを提供する。
type InstanceRegistry interface {
	RegisterInstance(ctx context.Context, instanceID string) error
	UnregisterInstance(ctx context.Context, instanceID string) error
	ListActiveInstances(ctx context.Context) ([]string, error)
	CleanupStaleAgents(ctx context.Context, activeInstanceIDs []string) (int64, error)
	CleanupStaleTasks(ctx context.Context, activeInstanceIDs []string) (int64, error)
}

// PaneSender はペインにキーストロークを送信する。
type PaneSender interface {
	SendKeys(ctx context.Context, paneID string, text string) error
}

// PaneLister は全ペイン情報を取得する。
type PaneLister interface {
	ListPanes(ctx context.Context) ([]PaneInfo, error)
}

// PaneCapturer はペインの表示内容を取得する。
type PaneCapturer interface {
	CapturePaneOutput(ctx context.Context, paneID string, lines int) (string, error)
}

// SelfPaneResolver は自ペインIDを解決する。
type SelfPaneResolver interface {
	GetPaneID(ctx context.Context) (string, error)
}

// PaneTitleSetter はペインタイトルを設定する。
type PaneTitleSetter interface {
	SetPaneTitle(ctx context.Context, paneID string, title string) error
}

// PaneSplitter は既存ペインを分割して新ペインを作成する。
type PaneSplitter interface {
	SplitPane(ctx context.Context, targetPaneID string, horizontal bool) (string, error)
}

// PanePasteSender はブラケットペーストモードでテキストを送信する。
// Claude Code等、\nをsubmitとして解釈するCLI向け。
type PanePasteSender interface {
	SendKeysPaste(ctx context.Context, paneID string, text string) error
}
