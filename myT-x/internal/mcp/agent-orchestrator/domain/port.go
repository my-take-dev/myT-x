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
	CompleteTask(ctx context.Context, taskID string, notes string, completedAt string) error
	MarkTaskFailed(ctx context.Context, taskID string, notes string) error
	AbandonTasksByPaneID(ctx context.Context, paneID string) error
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
