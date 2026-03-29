import {useCallback, useEffect, useMemo, useReducer, useRef} from "react";
import {api} from "../api";
import {useEscapeClose} from "../hooks/useEscapeClose";
import {useI18n} from "../i18n";
import {logFrontendEventSafe} from "../utils/logFrontendEventSafe";
import type {git} from "../../wailsjs/go/models";
import {INITIAL_STATE, newSessionReducer} from "./new-session/newSessionReducer";
import {NewSessionForm} from "./new-session/NewSessionForm";
import {WorktreeOptions} from "./new-session/WorktreeOptions";

interface NewSessionModalProps {
    open: boolean;
    onClose: () => void;
    onCreated: (sessionName: string) => void;
}

export function NewSessionModal({open, onClose, onCreated}: NewSessionModalProps) {
    const {language, t} = useI18n();
    const isEn = language === "en";

    const [s, dispatch] = useReducer(newSessionReducer, INITIAL_STATE);

    useEffect(() => {
        if (!open) {
            dispatch({type: "RESET"});
            return;
        }
        api.IsAgentTeamsAvailable()
            .then((available) => dispatch({type: "SET_FIELD", field: "shimAvailable", value: available}))
            .catch((err) => {
                if (import.meta.env.DEV) {
                    console.warn("[NewSessionModal] IsAgentTeamsAvailable failed", err);
                }
                dispatch({type: "SET_FIELD", field: "shimAvailable", value: false});
            });
        // NOTE: On config load failure, useClaudeEnv / usePaneEnv fall back to false
        // (conservative default). Session pane scope defaults to true independently.
        api.GetConfig()
            .then((cfg) => {
                dispatch({type: "SET_FIELD", field: "useClaudeEnv", value: cfg.claude_env?.default_enabled ?? false});
                dispatch({type: "SET_FIELD", field: "usePaneEnv", value: cfg.pane_env_default_enabled ?? false});
            })
            .catch((err) => {
                if (import.meta.env.DEV) {
                    console.warn("[NewSessionModal] failed to load config defaults", err);
                }
                logFrontendEventSafe("warn", `GetConfig failed: ${String(err)}`, "NewSessionModal");
                dispatch({type: "SET_FIELD", field: "configLoadFailed", value: true});
            });
    }, [open]);

    useEscapeClose(open, onClose);

    // Deferred loading: fetch branch/worktree data only when the user enables
    // the "Use Git Worktree" checkbox, not eagerly on folder selection.
    // This reduces folder selection from 10 git subprocesses to 3.
    useEffect(() => {
        if (!s.useWorktree || !s.isGitRepo || !s.directory) return;
        // Skip if data is already loaded for this directory (prevents
        // redundant refetch on checkbox OFF->ON toggle for same directory).
        // When directory changes, PICK_DIRECTORY resets these to [].
        if (s.branches.length > 0 || s.worktrees.length > 0) return;

        let cancelled = false;
        dispatch({type: "SET_FIELD", field: "worktreeDataLoading", value: true});
        Promise.all([
            api.ListBranches(s.directory).catch((err: unknown) => {
                if (import.meta.env.DEV) {
                    console.warn("[NewSessionModal] ListBranches failed:", err);
                }
                return [] as string[];
            }),
            api.ListWorktreesByRepo(s.directory).catch((err: unknown) => {
                if (import.meta.env.DEV) {
                    console.warn("[NewSessionModal] ListWorktreesByRepo failed:", err);
                }
                return [] as git.WorktreeInfo[];
            }),
        ]).then(([branchList, wtList]) => {
            if (cancelled) return;
            dispatch({
                type: "LOAD_GIT_DATA",
                branches: branchList,
                worktrees: wtList,
                baseBranch: branchList.length > 0 ? branchList[0] : "",
            });
        }).finally(() => {
            if (!cancelled) dispatch({type: "SET_FIELD", field: "worktreeDataLoading", value: false});
        });
        return () => { cancelled = true; };
    // eslint-disable-next-line react-hooks/exhaustive-deps -- s.branches.length/s.worktrees.length
    // are intentionally excluded: they serve as a runtime guard against redundant refetch,
    // not as reactive triggers. Directory change resets them via PICK_DIRECTORY action.
    }, [s.useWorktree, s.isGitRepo, s.directory]);

    const handlePickDirectory = useCallback(async () => {
        try {
            const dir = await api.PickSessionDirectory();
            if (!dir) return;

            dispatch({
                type: "PICK_DIRECTORY",
                directory: dir,
                sessionName: dir.split(/[\\/]/).filter(Boolean).pop() || "",
            });

            dispatch({type: "SET_FIELD", field: "gitCheckLoading", value: true});
            try {
                // Parallel: CheckDirectoryConflict is in-memory (<1ms),
                // IsGitRepository is lightweight (1 git subprocess, ~20-50ms)
                const [conflict, gitRepo] = await Promise.all([
                    api.CheckDirectoryConflict(dir).catch((err: unknown) => {
                        if (import.meta.env.DEV) {
                            console.error("[NewSessionModal] CheckDirectoryConflict failed:", err);
                        }
                        return "";
                    }),
                    api.IsGitRepository(dir),
                ]);
                dispatch({type: "SET_FIELD", field: "directoryConflict", value: conflict});
                dispatch({type: "SET_FIELD", field: "isGitRepo", value: gitRepo});

                if (gitRepo) {
                    // Only GetCurrentBranch here -- lightweight (2 git subprocesses).
                    // ListBranches and ListWorktreesByRepo are deferred until
                    // the user enables the "Use Git Worktree" checkbox.
                    const curBranch = await api.GetCurrentBranch(dir).catch((err: unknown) => {
                        if (import.meta.env.DEV) {
                            console.warn("[NewSessionModal] GetCurrentBranch failed:", err);
                        }
                        return "";
                    });
                    dispatch({type: "SET_FIELD", field: "currentBranch", value: curBranch});
                } else {
                    dispatch({type: "SET_FIELD", field: "currentBranch", value: ""});
                }
            } finally {
                dispatch({type: "SET_FIELD", field: "gitCheckLoading", value: false});
            }
        } catch (err) {
            dispatch({type: "SET_FIELD", field: "error", value: String(err)});
        }
    }, []);

    const worktreeCheckSeqRef = useRef(0);
    const handleSelectWorktree = useCallback(async (wt: git.WorktreeInfo) => {
        const seq = ++worktreeCheckSeqRef.current;
        dispatch({
            type: "SELECT_WORKTREE",
            worktree: wt,
            sessionName: wt.branch ? wt.branch.replace(/\//g, "-") : "",
        });
        try {
            const conflict = await api.CheckWorktreePathConflict(wt.path);
            if (seq !== worktreeCheckSeqRef.current) return; // stale response
            dispatch({type: "SET_FIELD", field: "worktreeConflict", value: conflict});
        } catch (err) {
            if (seq !== worktreeCheckSeqRef.current) return; // stale response
            if (import.meta.env.DEV) {
                console.error("[NewSessionModal] CheckWorktreePathConflict failed:", err);
            }
            dispatch({type: "SET_FIELD", field: "worktreeConflict", value: ""});
        }
    }, []);

    // NOTE: SettingsModal uses config.Config.createFrom(payload) for Wails type instances,
    // but CreateSessionOptions works with plain object literals. If Wails-side types grow
    // more complex, consider migrating to the createFrom() pattern.
    const handleSubmit = useCallback(async () => {
        if (!s.directory || !s.sessionName.trim()) return;
        dispatch({type: "START_SUBMIT"});
        try {
            let created;
            if (s.useWorktree && s.isGitRepo) {
                if (s.worktreeSource === "existing" && s.selectedWorktree) {
                    created = await api.CreateSessionWithExistingWorktree(
                        s.directory, s.sessionName.trim(), s.selectedWorktree.path, {
                            enable_agent_team: s.enableAgentTeam,
                            use_claude_env: s.useClaudeEnv,
                            use_pane_env: s.usePaneEnv,
                            use_session_pane_scope: s.useSessionPaneScope,
                        });
                } else {
                    const opts = {
                        branch_name: s.branchName.trim(),
                        base_branch: s.baseBranch,
                        pull_before_create: s.pullBefore,
                        enable_agent_team: s.enableAgentTeam,
                        use_claude_env: s.useClaudeEnv,
                        use_pane_env: s.usePaneEnv,
                        use_session_pane_scope: s.useSessionPaneScope,
                    };
                    created = await api.CreateSessionWithWorktree(s.directory, s.sessionName.trim(), opts);
                }
            } else {
                created = await api.CreateSession(s.directory, s.sessionName.trim(), {
                    enable_agent_team: s.enableAgentTeam,
                    use_claude_env: s.useClaudeEnv,
                    use_pane_env: s.usePaneEnv,
                    use_session_pane_scope: s.useSessionPaneScope,
                });
            }
            onCreated(created.name);
            onClose();
        } catch (err) {
            dispatch({type: "SET_FIELD", field: "error", value: String(err)});
        } finally {
            dispatch({type: "SET_FIELD", field: "loading", value: false});
        }
    }, [s, onCreated, onClose]);

    const canSubmit = useMemo(() => {
        if (!s.directory || !s.sessionName.trim() || s.loading || s.worktreeDataLoading || s.gitCheckLoading) return false;
        if (!s.useWorktree) return !s.directoryConflict;
        if (s.worktreeSource === "existing") {
            return !!s.selectedWorktree && !s.worktreeConflict;
        }
        // new worktree: always requires branch name
        return !!s.branchName.trim();
    }, [s.directory, s.sessionName, s.loading, s.worktreeDataLoading, s.gitCheckLoading, s.useWorktree, s.directoryConflict, s.worktreeSource, s.selectedWorktree, s.worktreeConflict, s.branchName]);

    if (!open) return null;

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
                    {s.configLoadFailed && (
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
                            {s.directory
                                ? s.directory
                                : (isEn
                                    ? "Select folder..."
                                    : t("newSession.directory.selectButton", "フォルダを選択..."))}
                        </button>
                    </div>

                    {/* Git repository check loading indicator */}
                    {s.directory && s.gitCheckLoading && (
                        <div className="form-inline-loading">
                            <span className="form-spinner" />
                            {isEn ? "Checking repository..." : t("newSession.git.checking", "リポジトリを確認中...")}
                        </div>
                    )}

                    {s.directory && (
                        <NewSessionForm
                            s={s}
                            dispatch={dispatch}
                            canSubmit={canSubmit}
                            onSubmit={handleSubmit}
                        />
                    )}

                    {s.directory && s.isGitRepo && (
                        <WorktreeOptions
                            s={s}
                            dispatch={dispatch}
                            onSelectWorktree={handleSelectWorktree}
                        />
                    )}

                    {s.error && <p className="form-error">{s.error}</p>}
                </div>
                <div className="modal-footer">
                    {s.directoryConflict && !s.useWorktree && (
                        <span className="form-error" style={{marginRight: "auto"}}>
                            {isEn
                                ? `Cannot start session (${s.directoryConflict} is already in use)`
                                : t("newSession.error.directoryConflict", "セッション開始不可（{sessionName} が使用中）", {
                                    sessionName: s.directoryConflict,
                                })}
                        </span>
                    )}
                    <button type="button" className="modal-btn" onClick={onClose} disabled={s.loading}>
                        {isEn ? "Cancel" : t("common.cancel", "キャンセル")}
                    </button>
                    <button
                        type="button"
                        className="modal-btn primary"
                        onClick={handleSubmit}
                        disabled={!canSubmit}
                    >
                        {s.loading
                            ? (isEn ? "Creating..." : t("newSession.action.creating", "作成中..."))
                            : (isEn ? "Create" : t("newSession.action.create", "作成"))}
                    </button>
                </div>
            </div>
        </div>
    );
}
