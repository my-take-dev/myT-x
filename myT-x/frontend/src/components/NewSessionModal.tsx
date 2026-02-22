import {useCallback, useEffect, useMemo, useState} from "react";
import {api} from "../api";
import {useEscapeClose} from "../hooks/useEscapeClose";
import type {git} from "../../wailsjs/go/models";

interface NewSessionModalProps {
    open: boolean;
    onClose: () => void;
    onCreated: (sessionName: string) => void;
}

type WorktreeSource = "existing" | "new";

export function NewSessionModal({open, onClose, onCreated}: NewSessionModalProps) {
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
    const [shimAvailable, setShimAvailable] = useState(false);
    const [loading, setLoading] = useState(false);
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
        setLoading(false);
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
        // (conservative default). This may differ from user's default_enabled setting,
        // but prevents unintended env injection when config state is unknown.
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

    const handlePickDirectory = useCallback(async () => {
        try {
            const dir = await api.PickSessionDirectory();
            if (!dir) return;
            setDirectory(dir);
            const folderName = dir.split(/[\\/]/).filter(Boolean).pop() || "";
            setSessionName(folderName);
            setError("");
            setWorktreeSource("new");
            setSelectedWorktree(null);
            setWorktreeConflict("");

            // Check for directory conflict with existing sessions.
            try {
                const conflict = await api.CheckDirectoryConflict(dir);
                setDirectoryConflict(conflict);
            } catch (err) {
                console.error("[NewSessionModal] CheckDirectoryConflict failed:", err);
                setDirectoryConflict("");
            }

            const gitRepo = await api.IsGitRepository(dir);
            setIsGitRepo(gitRepo);
            if (gitRepo) {
                // NOTE: 各 Promise に個別 catch を付与し、部分失敗時もデフォルト値で続行する。
                // Promise.allSettled は不要（個別 catch で同等の耐障害性を実現済み）。
                const [branchList, wtList, curBranch] = await Promise.all([
                    api.ListBranches(dir).catch(() => [] as string[]),
                    api.ListWorktreesByRepo(dir).catch(() => [] as git.WorktreeInfo[]),
                    api.GetCurrentBranch(dir).catch(() => ""),
                ]);
                setBranches(branchList);
                setWorktrees(wtList);
                setCurrentBranch(curBranch);
                if (branchList.length > 0) {
                    setBaseBranch(branchList[0]);
                }
            } else {
                setUseWorktree(false);
                setBranches([]);
                setWorktrees([]);
                setCurrentBranch("");
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
                        });
                } else {
                    const opts = {
                        branch_name: branchName.trim(),
                        base_branch: baseBranch,
                        pull_before_create: pullBefore,
                        enable_agent_team: enableAgentTeam,
                        use_claude_env: useClaudeEnv,
                        use_pane_env: usePaneEnv,
                    };
                    created = await api.CreateSessionWithWorktree(directory, sessionName.trim(), opts);
                }
            } else {
                created = await api.CreateSession(directory, sessionName.trim(), {
                    enable_agent_team: enableAgentTeam,
                    use_claude_env: useClaudeEnv,
                    use_pane_env: usePaneEnv,
                });
            }
            onCreated(created.name);
            onClose();
        } catch (err) {
            setError(String(err));
        } finally {
            setLoading(false);
        }
    }, [directory, sessionName, useWorktree, isGitRepo, worktreeSource, selectedWorktree, branchName, baseBranch, pullBefore, enableAgentTeam, useClaudeEnv, usePaneEnv, onCreated, onClose]);

    const canSubmit = useMemo(() => {
        if (!directory || !sessionName.trim() || loading) return false;
        if (!useWorktree) return !directoryConflict;
        if (worktreeSource === "existing") {
            return !!selectedWorktree && !worktreeConflict;
        }
        // new worktree: always requires branch name
        return !!branchName.trim();
    }, [directory, sessionName, loading, useWorktree, directoryConflict, worktreeSource, selectedWorktree, worktreeConflict, branchName]);

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
                    <h2 id="new-session-title">新規セッション</h2>
                </div>
                <div className="modal-body">
                    {configLoadFailed && (
                        <p className="form-warning">
                            設定の読み込みに失敗しました。デフォルト値で表示しています。
                            Claude Code環境変数とペイン環境変数はOFFで作成されます。
                        </p>
                    )}
                    {/* Directory selection */}
                    <div className="form-group">
                        <span className="form-label">作業ディレクトリ</span>
                        <button type="button" className="modal-btn" onClick={handlePickDirectory}>
                            {directory ? directory : "フォルダを選択..."}
                        </button>
                    </div>

                    {/* Session name */}
                    {directory && (
                        <div className="form-group">
                            <span className="form-label">セッション名</span>
                            <input
                                className="form-input"
                                value={sessionName}
                                onChange={(e) => setSessionName(e.target.value)}
                                onKeyDown={(e) => {
                                    if (e.key === "Enter" && canSubmit) void handleSubmit();
                                }}
                                placeholder="セッション名を入力"
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
                                Agent Team として開始
                                {!shimAvailable && <span className="form-hint"> (シム未インストール)</span>}
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
                                Claude Code 環境変数を利用する
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
                                追加ペイン専用環境変数を利用する
                            </label>
                        </div>
                    )}

                    {/* Git info & worktree options */}
                    {directory && isGitRepo && (
                        <>
                            {/* Current branch display */}
                            {currentBranch && (
                                <div className="current-branch-info">
                                    現在のブランチ: <span className="current-branch-name">{currentBranch}</span>
                                </div>
                            )}

                            {/* Use worktree checkbox */}
                            <div className="form-checkbox-row">
                                <input
                                    type="checkbox"
                                    id="use-worktree"
                                    checked={useWorktree}
                                    onChange={(e) => setUseWorktree(e.target.checked)}
                                />
                                <label htmlFor="use-worktree">Git Worktree を使用</label>
                            </div>

                            {useWorktree && (
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
                                                <label htmlFor="wt-source-existing">既存worktreeを使用</label>
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
                                                        <option value="">選択してください...</option>
                                                        {nonMainWorktrees.map((wt) => (
                                                            <option key={wt.path} value={wt.path}>
                                                                {wt.branch || "(detached)"} - {wt.path}
                                                            </option>
                                                        ))}
                                                    </select>
                                                    {worktreeConflict && (
                                                        <p className="form-error">
                                                            このworktreeはセッション「{worktreeConflict}」で使用中です
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
                                        <label htmlFor="wt-source-new">新規worktreeを作成</label>
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
                                                <label htmlFor="pull-before">作成前に pull（最新取得）</label>
                                            </div>

                                            {/* Base branch */}
                                            <div className="form-group">
                                                <span className="form-label">ベースブランチ</span>
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
                                                <span className="form-label">ブランチ名</span>
                                                <input
                                                    className="form-input"
                                                    value={branchName}
                                                    onChange={(e) => setBranchName(e.target.value)}
                                                    placeholder="feature/my-branch"
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
              セッション開始不可（{directoryConflict} が使用中）
            </span>
                    )}
                    <button type="button" className="modal-btn" onClick={onClose} disabled={loading}>
                        キャンセル
                    </button>
                    <button
                        type="button"
                        className="modal-btn primary"
                        onClick={handleSubmit}
                        disabled={!canSubmit}
                    >
                        {loading ? "作成中..." : "作成"}
                    </button>
                </div>
            </div>
        </div>
    );
}
