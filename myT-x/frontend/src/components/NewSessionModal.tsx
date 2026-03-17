import {useCallback, useEffect, useMemo, useState} from "react";
import {api} from "../api";
import {useEscapeClose} from "../hooks/useEscapeClose";
import type {git} from "../../wailsjs/go/models";
import {useI18n} from "../i18n";

interface NewSessionModalProps {
    open: boolean;
    onClose: () => void;
    onCreated: (sessionName: string) => void;
}

type WorktreeSource = "existing" | "new";

export function NewSessionModal({open, onClose, onCreated}: NewSessionModalProps) {
    const {language, t} = useI18n();
    const isEn = language === "en";

    const [directory, setDirectory] = useState("");
    const [sessionName, setSessionName] = useState("");
    const [isGitRepo, setIsGitRepo] = useState(false);
    const [currentBranch, setCurrentBranch] = useState("");
    const [branches, setBranches] = useState<string[]>([]);
    const [worktrees, setWorktrees] = useState<git.WorktreeInfo[]>([]);
    const [useWorktree, setUseWorktree] = useState(false);
    const [worktreeSource, setWorktreeSource] = useState<WorktreeSource>("new");
    const [selectedWorktree, setSelectedWorktree] = useState<git.WorktreeInfo | null>(null);
    const [worktreeConflict, setWorktreeConflict] = useState("");
    const [directoryConflict, setDirectoryConflict] = useState("");
    const [baseBranch, setBaseBranch] = useState("");
    const [branchName, setBranchName] = useState("");
    const [pullBefore, setPullBefore] = useState(true);
    const [enableAgentTeam, setEnableAgentTeam] = useState(false);
    const [useClaudeEnv, setUseClaudeEnv] = useState(false);
    const [usePaneEnv, setUsePaneEnv] = useState(false);
    const [useSessionPaneScope, setUseSessionPaneScope] = useState(true);
    const [shimAvailable, setShimAvailable] = useState(false);
    const [loading, setLoading] = useState(false);
    const [gitCheckLoading, setGitCheckLoading] = useState(false);
    const [worktreeDataLoading, setWorktreeDataLoading] = useState(false);
    const [error, setError] = useState("");
    const [configLoadFailed, setConfigLoadFailed] = useState(false);

    // TODO: 22個の useState を useReducer に統合する（SettingsModal の formReducer パターン参照）。
    // reset() を dispatch({ type: "RESET" }) に置き換え、状態管理を一元化する。
    // 現時点では useState setter は React が安定参照を保証するため、依存配列は空で正しい。
    const reset = useCallback(() => {
        setDirectory("");
        setSessionName("");
        setIsGitRepo(false);
        setCurrentBranch("");
        setBranches([]);
        setWorktrees([]);
        setUseWorktree(false);
        setWorktreeSource("new");
        setSelectedWorktree(null);
        setWorktreeConflict("");
        setDirectoryConflict("");
        setBaseBranch("");
        setBranchName("");
        setPullBefore(true);
        setEnableAgentTeam(false);
        setUseClaudeEnv(false);
        setUsePaneEnv(false);
        setUseSessionPaneScope(true);
        setLoading(false);
        setGitCheckLoading(false);
        setWorktreeDataLoading(false);
        setError("");
        setConfigLoadFailed(false);
    }, []);

    useEffect(() => {
        if (!open) {
            reset();
            return;
        }
        api.IsAgentTeamsAvailable()
            .then(setShimAvailable)
            .catch((err) => {
                console.warn("[NewSessionModal] IsAgentTeamsAvailable failed", err);
                setShimAvailable(false);
            });
        // NOTE: On config load failure, useClaudeEnv / usePaneEnv fall back to false
        // (conservative default). Session pane scope defaults to true independently.
        api.GetConfig()
            .then((cfg) => {
                setUseClaudeEnv(cfg.claude_env?.default_enabled ?? false);
                setUsePaneEnv(cfg.pane_env_default_enabled ?? false);
            })
            .catch((err) => {
                console.warn("[NewSessionModal] failed to load config defaults", err);
                setConfigLoadFailed(true);
            });
    }, [open, reset]);

    useEscapeClose(open, onClose);

    // Deferred loading: fetch branch/worktree data only when the user enables
    // the "Use Git Worktree" checkbox, not eagerly on folder selection.
    // This reduces folder selection from 10 git subprocesses to 3.
    useEffect(() => {
        if (!useWorktree || !isGitRepo || !directory) return;
        // Skip if data is already loaded for this directory (prevents
        // redundant refetch on checkbox OFF→ON toggle for same directory).
        // When directory changes, handlePickDirectory resets these to [].
        if (branches.length > 0 || worktrees.length > 0) return;

        let cancelled = false;
        setWorktreeDataLoading(true);
        Promise.all([
            api.ListBranches(directory).catch(() => [] as string[]),
            api.ListWorktreesByRepo(directory).catch(() => [] as git.WorktreeInfo[]),
        ]).then(([branchList, wtList]) => {
            if (cancelled) return;
            setBranches(branchList);
            setWorktrees(wtList);
            if (branchList.length > 0) setBaseBranch(branchList[0]);
        }).finally(() => {
            if (!cancelled) setWorktreeDataLoading(false);
        });
        return () => { cancelled = true; };
    // eslint-disable-next-line react-hooks/exhaustive-deps -- branches.length/worktrees.length
    // are intentionally excluded: they serve as a runtime guard against redundant refetch,
    // not as reactive triggers. directory change resets them via handlePickDirectory.
    }, [useWorktree, isGitRepo, directory]);

    const handlePickDirectory = useCallback(async () => {
        try {
            const dir = await api.PickSessionDirectory();
            if (!dir) return;
            setDirectory(dir);
            setSessionName(dir.split(/[\\/]/).filter(Boolean).pop() || "");
            setError("");
            setWorktreeSource("new");
            setSelectedWorktree(null);
            setWorktreeConflict("");
            // Reset worktree data from any previous folder selection
            setBranches([]);
            setWorktrees([]);
            setUseWorktree(false);

            setGitCheckLoading(true);
            try {
                // Parallel: CheckDirectoryConflict is in-memory (<1ms),
                // IsGitRepository is lightweight (1 git subprocess, ~20-50ms)
                const [conflict, gitRepo] = await Promise.all([
                    api.CheckDirectoryConflict(dir).catch((err: unknown) => {
                        console.error("[NewSessionModal] CheckDirectoryConflict failed:", err);
                        return "";
                    }),
                    api.IsGitRepository(dir),
                ]);
                setDirectoryConflict(conflict);
                setIsGitRepo(gitRepo);

                if (gitRepo) {
                    // Only GetCurrentBranch here -- lightweight (2 git subprocesses).
                    // ListBranches and ListWorktreesByRepo are deferred until
                    // the user enables the "Use Git Worktree" checkbox.
                    const curBranch = await api.GetCurrentBranch(dir).catch(() => "");
                    setCurrentBranch(curBranch);
                } else {
                    setCurrentBranch("");
                }
            } finally {
                setGitCheckLoading(false);
            }
        } catch (err) {
            setError(String(err));
        }
    }, []);

    const handleSelectWorktree = useCallback(async (wt: git.WorktreeInfo) => {
        setSelectedWorktree(wt);
        if (wt.branch) {
            setSessionName(wt.branch.replace(/\//g, "-"));
        }
        try {
            const conflict = await api.CheckWorktreePathConflict(wt.path);
            setWorktreeConflict(conflict);
        } catch (err) {
            console.error("[NewSessionModal] CheckWorktreePathConflict failed:", err);
            setWorktreeConflict("");
        }
    }, []);

    // NOTE: SettingsModal では config.Config.createFrom(payload) でWails型インスタンスを生成しているが、
    // CreateSessionOptions は単純なオブジェクトリテラルで十分な為、createFrom() は使用しない。
    // Wails側の型が複雑化した場合は createFrom() パターンへの移行を検討する。
    const handleSubmit = useCallback(async () => {
        if (!directory || !sessionName.trim()) return;
        setLoading(true);
        setError("");
        try {
            let created;
            if (useWorktree && isGitRepo) {
                if (worktreeSource === "existing" && selectedWorktree) {
                    created = await api.CreateSessionWithExistingWorktree(
                        directory, sessionName.trim(), selectedWorktree.path, {
                            enable_agent_team: enableAgentTeam,
                            use_claude_env: useClaudeEnv,
                            use_pane_env: usePaneEnv,
                            use_session_pane_scope: useSessionPaneScope,
                        });
                } else {
                    const opts = {
                        branch_name: branchName.trim(),
                        base_branch: baseBranch,
                        pull_before_create: pullBefore,
                        enable_agent_team: enableAgentTeam,
                        use_claude_env: useClaudeEnv,
                        use_pane_env: usePaneEnv,
                        use_session_pane_scope: useSessionPaneScope,
                    };
                    created = await api.CreateSessionWithWorktree(directory, sessionName.trim(), opts);
                }
            } else {
                created = await api.CreateSession(directory, sessionName.trim(), {
                    enable_agent_team: enableAgentTeam,
                    use_claude_env: useClaudeEnv,
                    use_pane_env: usePaneEnv,
                    use_session_pane_scope: useSessionPaneScope,
                });
            }
            onCreated(created.name);
            onClose();
        } catch (err) {
            setError(String(err));
        } finally {
            setLoading(false);
        }
    }, [directory, sessionName, useWorktree, isGitRepo, worktreeSource, selectedWorktree, branchName, baseBranch, pullBefore, enableAgentTeam, useClaudeEnv, usePaneEnv, useSessionPaneScope, onCreated, onClose]);

    const canSubmit = useMemo(() => {
        if (!directory || !sessionName.trim() || loading || worktreeDataLoading) return false;
        if (!useWorktree) return !directoryConflict;
        if (worktreeSource === "existing") {
            return !!selectedWorktree && !worktreeConflict;
        }
        // new worktree: always requires branch name
        return !!branchName.trim();
    }, [directory, sessionName, loading, worktreeDataLoading, useWorktree, directoryConflict, worktreeSource, selectedWorktree, worktreeConflict, branchName]);

    if (!open) return null;

    const nonMainWorktrees = worktrees.filter((w) => !w.isMain);

    return (
        <div className="modal-overlay" onClick={onClose}>
            <div
                className="modal-panel"
                role="dialog"
                aria-modal="true"
                aria-labelledby="new-session-title"
                onClick={(e) => e.stopPropagation()}
            >
                <div className="modal-header">
                    <h2 id="new-session-title">
                        {isEn ? "New Session" : t("newSession.title", "新規セッション")}
                    </h2>
                </div>
                <div className="modal-body">
                    {configLoadFailed && (
                        <p className="form-warning">
                            {isEn
                                ? "Failed to load settings. Showing defaults."
                                : t("newSession.warning.configLoadFailedLine1", "設定の読み込みに失敗しました。デフォルト値で表示しています。")}
                            <br/>
                            {isEn
                                ? "Claude Code env vars and pane env vars will start as OFF."
                                : t("newSession.warning.configLoadFailedLine2", "Claude Code環境変数とペイン環境変数はOFFで作成されます。")}
                        </p>
                    )}
                    {/* Directory selection */}
                    <div className="form-group">
                        <span className="form-label">
                            {isEn ? "Working Directory" : t("newSession.directory.label", "作業ディレクトリ")}
                        </span>
                        <button type="button" className="modal-btn" onClick={handlePickDirectory}>
                            {directory
                                ? directory
                                : (isEn
                                    ? "Select folder..."
                                    : t("newSession.directory.selectButton", "フォルダを選択..."))}
                        </button>
                    </div>

                    {/* Git repository check loading indicator */}
                    {directory && gitCheckLoading && (
                        <div className="form-inline-loading">
                            <span className="form-spinner" />
                            {isEn ? "Checking repository..." : t("newSession.git.checking", "リポジトリを確認中...")}
                        </div>
                    )}

                    {/* Session name */}
                    {directory && (
                        <div className="form-group">
                            <span className="form-label">
                                {isEn ? "Session Name" : t("newSession.sessionName.label", "セッション名")}
                            </span>
                            <input
                                className="form-input"
                                value={sessionName}
                                onChange={(e) => setSessionName(e.target.value)}
                                onKeyDown={(e) => {
                                    if (e.key === "Enter" && canSubmit) void handleSubmit();
                                }}
                                placeholder={
                                    isEn
                                        ? "Enter session name"
                                        : t("newSession.sessionName.placeholder", "セッション名を入力")
                                }
                                autoFocus
                            />
                        </div>
                    )}

                    {/* Agent Team option */}
                    {directory && (
                        <div className="form-checkbox-row">
                            <input
                                type="checkbox"
                                id="enable-agent-team"
                                checked={enableAgentTeam}
                                onChange={(e) => setEnableAgentTeam(e.target.checked)}
                                disabled={!shimAvailable}
                            />
                            <label htmlFor="enable-agent-team">
                                {isEn ? "Start as Agent Team" : t("newSession.agentTeam.enable", "Agent Team として開始")}
                                {!shimAvailable && (
                                    <span className="form-hint">
                                        {isEn
                                            ? " (shim not installed)"
                                            : t("newSession.agentTeam.shimMissing", " (シム未インストール)")}
                                    </span>
                                )}
                            </label>
                        </div>
                    )}

                    {/* Claude Code env option */}
                    {directory && (
                        <div className="form-checkbox-row">
                            <input
                                type="checkbox"
                                id="use-claude-env"
                                checked={useClaudeEnv}
                                onChange={(e) => setUseClaudeEnv(e.target.checked)}
                            />
                            <label htmlFor="use-claude-env">
                                {isEn
                                    ? "Use Claude Code environment variables"
                                    : t("newSession.env.claude", "Claude Code 環境変数を利用する")}
                            </label>
                        </div>
                    )}

                    {/* Pane env option */}
                    {directory && (
                        <div className="form-checkbox-row">
                            <input
                                type="checkbox"
                                id="use-pane-env"
                                checked={usePaneEnv}
                                onChange={(e) => setUsePaneEnv(e.target.checked)}
                            />
                            <label htmlFor="use-pane-env">
                                {isEn
                                    ? "Use additional pane-only environment variables"
                                    : t("newSession.env.pane", "追加ペイン専用環境変数を利用する")}
                            </label>
                        </div>
                    )}

                    {/* Session pane scope option */}
                    {directory && (
                        <div className="form-checkbox-row">
                            <input
                                type="checkbox"
                                id="use-session-pane-scope"
                                checked={useSessionPaneScope}
                                onChange={(e) => setUseSessionPaneScope(e.target.checked)}
                            />
                            <label htmlFor="use-session-pane-scope">
                                {isEn
                                    ? "Use session-based pane management"
                                    : t("newSession.env.sessionPaneScope", "セッション単位ペイン管理を利用する")}
                            </label>
                        </div>
                    )}

                    {/* Git info & worktree options */}
                    {directory && isGitRepo && (
                        <>
                            {/* Current branch display */}
                            {currentBranch && (
                                <div className="current-branch-info">
                                    {isEn ? "Current branch:" : t("newSession.git.currentBranch", "現在のブランチ:")}
                                    {" "}
                                    <span className="current-branch-name">{currentBranch}</span>
                                </div>
                            )}

                            {/* Use worktree checkbox */}
                            <div className="form-checkbox-row">
                                <input
                                    type="checkbox"
                                    id="use-worktree"
                                    checked={useWorktree}
                                    onChange={(e) => setUseWorktree(e.target.checked)}
                                    disabled={gitCheckLoading}
                                />
                                <label htmlFor="use-worktree">
                                    {isEn ? "Use Git Worktree" : t("newSession.worktree.enable", "Git Worktree を使用")}
                                </label>
                            </div>

                            {useWorktree && worktreeDataLoading && (
                                <div className="form-inline-loading">
                                    <span className="form-spinner" />
                                    {isEn ? "Loading branches..." : t("newSession.worktree.loading", "ブランチを読み込み中...")}
                                </div>
                            )}
                            {useWorktree && !worktreeDataLoading && (
                                <div className="session-mode-selector">
                                    {/* Existing worktree option (only if non-main worktrees exist) */}
                                    {nonMainWorktrees.length > 0 && (
                                        <>
                                            <div className="form-radio-row">
                                                <input
                                                    type="radio"
                                                    id="wt-source-existing"
                                                    name="wt-source"
                                                    checked={worktreeSource === "existing"}
                                                    onChange={() => setWorktreeSource("existing")}
                                                />
                                                <label htmlFor="wt-source-existing">
                                                    {isEn
                                                        ? "Use existing worktree"
                                                        : t("newSession.worktree.source.existing", "既存worktreeを使用")}
                                                </label>
                                            </div>
                                            {worktreeSource === "existing" && (
                                                <div className="form-group indented">
                                                    <select
                                                        className="form-select"
                                                        value={selectedWorktree?.path || ""}
                                                        onChange={(e) => {
                                                            const wt = nonMainWorktrees.find((w) => w.path === e.target.value);
                                                            if (wt) void handleSelectWorktree(wt);
                                                        }}
                                                    >
                                                        <option value="">
                                                            {isEn
                                                                ? "Please select..."
                                                                : t("newSession.worktree.select.placeholder", "選択してください...")}
                                                        </option>
                                                        {nonMainWorktrees.map((wt) => (
                                                            <option key={wt.path} value={wt.path}>
                                                                {wt.branch
                                                                    || (isEn
                                                                        ? "(detached)"
                                                                        : t("newSession.worktree.detached", "(detached)"))}
                                                                {" - "}
                                                                {wt.path}
                                                            </option>
                                                        ))}
                                                    </select>
                                                    {worktreeConflict && (
                                                        <p className="form-error">
                                                            {isEn
                                                                ? `This worktree is already used by session "${worktreeConflict}".`
                                                                : t("newSession.worktree.conflict", "このworktreeはセッション「{sessionName}」で使用中です", {
                                                                    sessionName: worktreeConflict,
                                                                })}
                                                        </p>
                                                    )}
                                                </div>
                                            )}
                                        </>
                                    )}

                                    {/* New worktree option */}
                                    <div className="form-radio-row">
                                        <input
                                            type="radio"
                                            id="wt-source-new"
                                            name="wt-source"
                                            checked={worktreeSource === "new"}
                                            onChange={() => setWorktreeSource("new")}
                                        />
                                        <label htmlFor="wt-source-new">
                                            {isEn
                                                ? "Create new worktree"
                                                : t("newSession.worktree.source.new", "新規worktreeを作成")}
                                        </label>
                                    </div>
                                    {worktreeSource === "new" && (
                                        <div className="form-group indented">
                                            {/* Pull before create */}
                                            <div className="form-checkbox-row">
                                                <input
                                                    type="checkbox"
                                                    id="pull-before"
                                                    checked={pullBefore}
                                                    onChange={(e) => setPullBefore(e.target.checked)}
                                                />
                                                <label htmlFor="pull-before">
                                                    {isEn
                                                        ? "Pull latest before create"
                                                        : t("newSession.worktree.pullBefore", "作成前に pull（最新取得）")}
                                                </label>
                                            </div>

                                            {/* Base branch */}
                                            <div className="form-group">
                                                <span className="form-label">
                                                    {isEn
                                                        ? "Base Branch"
                                                        : t("newSession.worktree.baseBranch.label", "ベースブランチ")}
                                                </span>
                                                <select
                                                    className="form-select"
                                                    value={baseBranch}
                                                    onChange={(e) => setBaseBranch(e.target.value)}
                                                >
                                                    {branches.map((b) => (
                                                        <option key={b} value={b}>{b}</option>
                                                    ))}
                                                </select>
                                            </div>

                                            {/* Branch name */}
                                            <div className="form-group">
                                                <span className="form-label">
                                                    {isEn
                                                        ? "Branch Name"
                                                        : t("newSession.worktree.branchName.label", "ブランチ名")}
                                                </span>
                                                <input
                                                    className="form-input"
                                                    value={branchName}
                                                    onChange={(e) => setBranchName(e.target.value)}
                                                    placeholder={
                                                        isEn
                                                            ? "feature/my-branch"
                                                            : t("newSession.worktree.branchName.placeholder", "feature/my-branch")
                                                    }
                                                />
                                            </div>
                                        </div>
                                    )}
                                </div>
                            )}
                        </>
                    )}

                    {error && <p className="form-error">{error}</p>}
                </div>
                <div className="modal-footer">
                    {directoryConflict && !useWorktree && (
                        <span className="form-error" style={{marginRight: "auto"}}>
                            {isEn
                                ? `Cannot start session (${directoryConflict} is already in use)`
                                : t("newSession.error.directoryConflict", "セッション開始不可（{sessionName} が使用中）", {
                                    sessionName: directoryConflict,
                                })}
                        </span>
                    )}
                    <button type="button" className="modal-btn" onClick={onClose} disabled={loading}>
                        {isEn ? "Cancel" : t("common.cancel", "キャンセル")}
                    </button>
                    <button
                        type="button"
                        className="modal-btn primary"
                        onClick={handleSubmit}
                        disabled={!canSubmit}
                    >
                        {loading
                            ? (isEn ? "Creating..." : t("newSession.action.creating", "作成中..."))
                            : (isEn ? "Create" : t("newSession.action.create", "作成"))}
                    </button>
                </div>
            </div>
        </div>
    );
}
