package orchestrator

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"myT-x/internal/mcp/agent-orchestrator/adapter/mcptool"
	"myT-x/internal/mcp/agent-orchestrator/domain"
	"myT-x/internal/mcp/agent-orchestrator/internal/mcp"
	"myT-x/internal/mcp/agent-orchestrator/internal/store"
	"myT-x/internal/mcp/agent-orchestrator/internal/tmux"
	"myT-x/internal/mcp/agent-orchestrator/usecase"
)

const (
	DefaultServerName    = "agent-orchestrator"
	DefaultServerVersion = "0.1.0"
)

// Config はオーケストレーターの設定を表す。
type Config struct {
	DBPath        string
	In            io.Reader
	Out           io.Writer
	Logger        *log.Logger
	ServerName    string
	ServerVersion string

	// SessionName は MCP インスタンスが属するセッション名。
	// 空の場合はセッション名不明として扱い、list_panes は全セッションを返す。
	SessionName string
	// SessionAllPanes が true の場合、list_panes は全セッションのペインを返す（既存動作互換）。
	// false（デフォルト）の場合、SessionName が設定されていれば自セッションのみに絞り込む。
	SessionAllPanes bool

	// DI注入点（nil の場合はデフォルト実装を使用）
	AgentRepo       domain.AgentRepository
	AgentStatusRepo domain.AgentStatusRepository
	TaskRepo        domain.TaskRepository
	MessageRepo     domain.MessageRepository
	Sender          domain.PaneSender
	Lister          domain.PaneLister
	Capturer        domain.PaneCapturer
	SelfResolver    domain.SelfPaneResolver
	TitleSetter     domain.PaneTitleSetter
	Splitter        domain.PaneSplitter
	PasteSender     domain.PanePasteSender
}

// Runtime は MCP サーバーとライフサイクルを管理する。
type Runtime struct {
	cfg        Config
	store      *store.Store
	resolver   domain.SelfPaneResolver
	agentRepo  domain.AgentRepository
	taskRepo   domain.TaskRepository
	server     *mcp.Server
	selfPane   string
	instanceID string

	mu      sync.Mutex
	started bool
	closed  bool
}

// generateInstanceID は MCP インスタンスのユニークIDを生成する。
func generateInstanceID() (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate instance id: %w", err)
	}
	return "mcp-" + hex.EncodeToString(b), nil
}

// NewRuntime はランタイムを構築する。
func NewRuntime(cfg Config) (*Runtime, error) {
	instanceID, err := generateInstanceID()
	if err != nil {
		return nil, err
	}

	normalized := normalizeConfig(cfg)

	var st *store.Store
	agentRepo := normalized.AgentRepo
	agentStatusRepo := normalized.AgentStatusRepo
	taskRepo := normalized.TaskRepo
	messageRepo := normalized.MessageRepo
	sender := normalized.Sender
	lister := normalized.Lister
	capturer := normalized.Capturer
	resolver := normalized.SelfResolver
	titleSetter := normalized.TitleSetter
	// デフォルト実装の生成
	if agentRepo == nil || agentStatusRepo == nil || taskRepo == nil || messageRepo == nil {
		var err error
		st, err = store.Open(normalized.DBPath)
		if err != nil {
			return nil, err
		}
		if err := st.Migrate(); err != nil {
			if closeErr := st.Close(); closeErr != nil {
				return nil, fmt.Errorf("migrate database: %w (close store: %v)", err, closeErr)
			}
			return nil, err
		}
		if agentRepo == nil {
			agentRepo = st
		}
		if agentStatusRepo == nil {
			agentStatusRepo = st
		}
		if taskRepo == nil {
			taskRepo = st
		}
		if messageRepo == nil {
			messageRepo = st
		}
	}

	splitter := normalized.Splitter
	pasteSender := normalized.PasteSender

	if sender == nil || lister == nil || capturer == nil || resolver == nil || titleSetter == nil || splitter == nil || pasteSender == nil {
		exec := tmux.NewExecutor()
		if sender == nil {
			sender = exec
		}
		if lister == nil {
			// Use session-aware executor when SessionName is available.
			if normalized.SessionName != "" {
				lister = tmux.NewSessionAwareExecutor(normalized.SessionName, normalized.SessionAllPanes)
			} else {
				lister = exec
			}
		}
		if capturer == nil {
			capturer = exec
		}
		if resolver == nil {
			resolver = exec
		}
		if titleSetter == nil {
			titleSetter = exec
		}
		if splitter == nil {
			splitter = exec
		}
		if pasteSender == nil {
			pasteSender = exec
		}
	}
	stickyResolver := newStickySelfResolver(resolver, normalized.Logger)

	// usecase サービス構築
	agentSvc := usecase.NewAgentService(agentRepo, agentStatusRepo, stickyResolver, lister, titleSetter, normalized.Logger)
	dispatchSvc := usecase.NewTaskDispatchService(agentRepo, taskRepo, messageRepo, sender, stickyResolver, normalized.Logger)
	querySvc := usecase.NewTaskQueryService(agentRepo, taskRepo, messageRepo, sender, stickyResolver, normalized.Logger)
	taskUpdateSvc := usecase.NewTaskUpdateService(agentRepo, taskRepo, stickyResolver, normalized.Logger)
	responseSvc := usecase.NewResponseService(agentRepo, taskRepo, messageRepo, sender, stickyResolver, normalized.Logger)
	statusSvc := usecase.NewStatusService(agentRepo, agentStatusRepo, taskRepo, messageRepo, sender, stickyResolver, normalized.Logger)
	captureSvc := usecase.NewCaptureService(agentRepo, capturer, stickyResolver, normalized.Logger)

	// projectRoot: DBPath を絶対パスに変換してから2階層上を導出
	// （例: /project/.myT-x/orchestrator.db → /project）
	absDBPath, err := filepath.Abs(normalized.DBPath)
	if err != nil {
		var cleanupErrs []error
		if st != nil {
			if closeErr := st.Close(); closeErr != nil {
				cleanupErrs = append(cleanupErrs, fmt.Errorf("close store: %w", closeErr))
			}
		}
		if joined := errors.Join(cleanupErrs...); joined != nil {
			return nil, fmt.Errorf("resolve db path: %w (cleanup: %v)", err, joined)
		}
		return nil, fmt.Errorf("resolve db path: %w", err)
	}
	projectRoot := filepath.Dir(filepath.Dir(absDBPath))
	memberSvc := usecase.NewMemberBootstrapService(
		agentRepo, stickyResolver, splitter, titleSetter, sender, pasteSender, projectRoot, normalized.Logger,
	)

	handler := mcptool.NewHandler(agentSvc, dispatchSvc, querySvc, taskUpdateSvc, responseSvc, statusSvc, captureSvc, memberSvc, instanceID)
	registry, err := handler.BuildRegistry()
	if err != nil {
		var cleanupErrs []error
		if st != nil {
			if closeErr := st.Close(); closeErr != nil {
				cleanupErrs = append(cleanupErrs, fmt.Errorf("close store: %w", closeErr))
			}
		}
		if joined := errors.Join(cleanupErrs...); joined != nil {
			return nil, fmt.Errorf("build registry: %w (cleanup: %v)", err, joined)
		}
		return nil, fmt.Errorf("build registry: %w", err)
	}

	server := mcp.NewServer(mcp.Config{
		Name:     normalized.ServerName,
		Version:  normalized.ServerVersion,
		In:       normalized.In,
		Out:      normalized.Out,
		Logger:   normalized.Logger,
		Registry: registry,
	})

	return &Runtime{
		cfg:        normalized,
		store:      st,
		resolver:   stickyResolver,
		agentRepo:  agentRepo,
		taskRepo:   taskRepo,
		server:     server,
		instanceID: instanceID,
	}, nil
}

// Run はワンショット実行用エントリーポイント。
func Run(ctx context.Context, cfg Config) error {
	runtime, err := NewRuntime(cfg)
	if err != nil {
		return err
	}
	if err := runtime.Start(ctx); err != nil {
		return err
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if closeErr := runtime.Close(closeCtx); closeErr != nil && runtime.cfg.Logger != nil {
			runtime.cfg.Logger.Printf("ランタイム終了処理でエラー: %v", closeErr)
		}
	}()
	return runtime.Serve(ctx)
}

// Start は起動処理を行う（自ペインID取得・記録）。
func (r *Runtime) Start(ctx context.Context) error {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return errors.New("runtime is closed")
	}
	if r.started {
		r.mu.Unlock()
		return nil
	}
	r.mu.Unlock()

	paneID, err := r.resolver.GetPaneID(ctx)
	if err != nil {
		r.cfg.Logger.Printf("自ペインID取得に失敗（tmux外で実行中の可能性）: %v", err)
	} else if paneID == "" {
		r.cfg.Logger.Printf("自ペインID取得結果が空です。終了時のクリーンアップはスキップされる可能性があります")
	} else {
		r.selfPane = paneID
		if stickyResolver, ok := r.resolver.(*stickySelfResolver); ok {
			stickyResolver.SetPaneID(paneID)
		}
		r.cfg.Logger.Printf("自ペインID: %s", paneID)
	}

	// インスタンス登録とstaleデータのクリーンアップ
	if r.store != nil {
		if regErr := r.store.RegisterInstance(ctx, r.instanceID); regErr != nil {
			r.cfg.Logger.Printf("[DEBUG:instance-cleanup] インスタンス登録に失敗: %v", regErr)
		} else {
			r.cfg.Logger.Printf("[DEBUG:instance-cleanup] インスタンス登録完了: %s", r.instanceID)
			activeIDs, listErr := r.store.ListActiveInstances(ctx)
			if listErr != nil {
				r.cfg.Logger.Printf("[DEBUG:instance-cleanup] アクティブインスタンス取得に失敗: %v", listErr)
			} else {
				if n, cleanErr := r.store.CleanupStaleAgents(ctx, activeIDs); cleanErr != nil {
					r.cfg.Logger.Printf("[DEBUG:instance-cleanup] ゴーストエージェントクリーンアップに失敗: %v", cleanErr)
				} else if n > 0 {
					r.cfg.Logger.Printf("[DEBUG:instance-cleanup] ゴーストエージェント %d 件を削除しました", n)
				}
				if n, cleanErr := r.store.CleanupStaleTasks(ctx, activeIDs); cleanErr != nil {
					r.cfg.Logger.Printf("[DEBUG:instance-cleanup] 孤立タスククリーンアップに失敗: %v", cleanErr)
				} else if n > 0 {
					r.cfg.Logger.Printf("[DEBUG:instance-cleanup] 孤立タスク %d 件を abandoned に更新しました", n)
				}
			}
		}
	}

	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return errors.New("runtime is closed")
	}
	r.started = true
	r.mu.Unlock()
	return nil
}

// Serve は MCP リクエストを処理するためにブロックする。
func (r *Runtime) Serve(ctx context.Context) error {
	if err := r.Start(ctx); err != nil {
		return err
	}
	return r.server.Serve(ctx)
}

// Close は終了処理を行う（自ペインのエントリ削除・タスク abandoned 化）。
func (r *Runtime) Close(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil
	}
	r.closed = true
	started := r.started
	r.started = false
	r.mu.Unlock()

	if !started {
		return r.closeResources(ctx)
	}

	// セッション終了マーク: sender_instance_id ベースで全タスクの is_now_session を false に更新
	if err := r.taskRepo.EndSessionByInstanceID(ctx, r.instanceID); err != nil {
		r.cfg.Logger.Printf("セッション終了マーク更新に失敗: %v", err)
	}

	if r.selfPane != "" {
		if err := r.taskRepo.AbandonTasksByPaneID(ctx, r.selfPane); err != nil {
			r.cfg.Logger.Printf("タスク abandoned 化に失敗: %v", err)
		}
		if err := r.agentRepo.DeleteAgentsByPaneID(ctx, r.selfPane); err != nil {
			r.cfg.Logger.Printf("エージェント削除に失敗: %v", err)
		}
	} else {
		r.cfg.Logger.Printf("自ペインID不明のため終了時クリーンアップをスキップします")
	}

	// インスタンス登録解除
	if r.store != nil {
		if err := r.store.UnregisterInstance(ctx, r.instanceID); err != nil {
			r.cfg.Logger.Printf("[DEBUG:instance-cleanup] インスタンス登録解除に失敗: %v", err)
		}
	}

	return r.closeResources(ctx)
}

func (r *Runtime) closeResources(ctx context.Context) error {
	var errs []string
	if r.store != nil {
		if err := runWithContext(ctx, r.store.Close); err != nil {
			errs = append(errs, fmt.Sprintf("close store: %v", err))
		}
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func runWithContext(ctx context.Context, fn func() error) error {
	errCh := make(chan error, 1)
	go func() {
		errCh <- fn()
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func normalizeConfig(cfg Config) Config {
	if cfg.DBPath == "" {
		cfg.DBPath = ".myT-x/orchestrator.db"
	}
	if cfg.In == nil {
		cfg.In = os.Stdin
	}
	if cfg.Out == nil {
		cfg.Out = os.Stdout
	}
	if cfg.Logger == nil {
		cfg.Logger = log.New(io.Discard, "", 0)
	}
	if cfg.ServerName == "" {
		cfg.ServerName = DefaultServerName
	}
	if cfg.ServerVersion == "" {
		cfg.ServerVersion = DefaultServerVersion
	}
	return cfg
}
