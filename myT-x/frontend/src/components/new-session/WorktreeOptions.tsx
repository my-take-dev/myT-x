import type {git} from "../../../wailsjs/go/models";
import {useI18n} from "../../i18n";
import type {NewSessionDispatch, NewSessionState} from "./types";

interface WorktreeOptionsProps {
    s: NewSessionState;
    dispatch: NewSessionDispatch;
    onSelectWorktree: (wt: git.WorktreeInfo) => void;
}

export function WorktreeOptions({s, dispatch, onSelectWorktree}: WorktreeOptionsProps) {
    const {language, t} = useI18n();
    const isEn = language === "en";

    const nonMainWorktrees = s.worktrees.filter((w) => !w.isMain);

    return (
        <>
            {/* Current branch display */}
            {s.currentBranch && (
                <div className="current-branch-info">
                    {isEn ? "Current branch:" : t("newSession.git.currentBranch", "現在のブランチ:")}
                    {" "}
                    <span className="current-branch-name">{s.currentBranch}</span>
                </div>
            )}

            {/* Use worktree checkbox */}
            <div className="form-checkbox-row">
                <input
                    type="checkbox"
                    id="use-worktree"
                    checked={s.useWorktree}
                    onChange={(e) => dispatch({type: "SET_FIELD", field: "useWorktree", value: e.target.checked})}
                    disabled={s.gitCheckLoading}
                />
                <label htmlFor="use-worktree">
                    {isEn ? "Use Git Worktree" : t("newSession.worktree.enable", "Git Worktree を使用")}
                </label>
            </div>

            {s.useWorktree && s.worktreeDataLoading && (
                <div className="form-inline-loading">
                    <span className="form-spinner" />
                    {isEn ? "Loading branches..." : t("newSession.worktree.loading", "ブランチを読み込み中...")}
                </div>
            )}

            {s.useWorktree && !s.worktreeDataLoading && (
                <div className="session-mode-selector">
                    {/* Existing worktree option (only if non-main worktrees exist) */}
                    {nonMainWorktrees.length > 0 && (
                        <>
                            <div className="form-radio-row">
                                <input
                                    type="radio"
                                    id="wt-source-existing"
                                    name="wt-source"
                                    checked={s.worktreeSource === "existing"}
                                    onChange={() => dispatch({type: "SET_FIELD", field: "worktreeSource", value: "existing"})}
                                />
                                <label htmlFor="wt-source-existing">
                                    {isEn
                                        ? "Use existing worktree"
                                        : t("newSession.worktree.source.existing", "既存worktreeを使用")}
                                </label>
                            </div>
                            {s.worktreeSource === "existing" && (
                                <div className="form-group indented">
                                    <select
                                        className="form-select"
                                        value={s.selectedWorktree?.path || ""}
                                        onChange={(e) => {
                                            const wt = nonMainWorktrees.find((w) => w.path === e.target.value);
                                            if (wt) void onSelectWorktree(wt);
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
                                    {s.worktreeConflict && (
                                        <p className="form-error">
                                            {isEn
                                                ? `This worktree is already used by session "${s.worktreeConflict}".`
                                                : t("newSession.worktree.conflict", "このworktreeはセッション「{sessionName}」で使用中です", {
                                                    sessionName: s.worktreeConflict,
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
                            checked={s.worktreeSource === "new"}
                            onChange={() => dispatch({type: "SET_FIELD", field: "worktreeSource", value: "new"})}
                        />
                        <label htmlFor="wt-source-new">
                            {isEn
                                ? "Create new worktree"
                                : t("newSession.worktree.source.new", "新規worktreeを作成")}
                        </label>
                    </div>
                    {s.worktreeSource === "new" && (
                        <div className="form-group indented">
                            {/* Pull before create */}
                            <div className="form-checkbox-row">
                                <input
                                    type="checkbox"
                                    id="pull-before"
                                    checked={s.pullBefore}
                                    onChange={(e) => dispatch({type: "SET_FIELD", field: "pullBefore", value: e.target.checked})}
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
                                    value={s.baseBranch}
                                    onChange={(e) => dispatch({type: "SET_FIELD", field: "baseBranch", value: e.target.value})}
                                >
                                    {s.branches.map((b) => (
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
                                    value={s.branchName}
                                    onChange={(e) => dispatch({type: "SET_FIELD", field: "branchName", value: e.target.value})}
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
    );
}
