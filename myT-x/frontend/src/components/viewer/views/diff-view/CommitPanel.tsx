import {useCallback, useRef} from "react";
import type {BranchInfo, OperationType} from "./sourceControlTypes";

interface CommitPanelProps {
    branchInfo: BranchInfo | null;
    commitMessage: string;
    onSetCommitMessage: (msg: string) => void;
    onCommit: (message: string) => Promise<boolean>;
    onCommitAndPush: (message: string) => Promise<boolean>;
    onPush: () => Promise<void>;
    onPull: () => Promise<void>;
    onFetch: () => Promise<void>;
    operationInFlight: OperationType;
    stagedCount: number;
}

export function CommitPanel({
    branchInfo,
    commitMessage,
    onSetCommitMessage,
    onCommit,
    onCommitAndPush,
    onPush,
    onPull,
    onFetch,
    operationInFlight,
    stagedCount,
}: CommitPanelProps) {
    const textareaRef = useRef<HTMLTextAreaElement>(null);
    const isDisabled = operationInFlight !== null;
    const hasConflicts = (branchInfo?.conflicted?.length ?? 0) > 0;
    const canCommit = commitMessage.trim().length > 0 && stagedCount > 0 && !isDisabled && !hasConflicts;

    const handleCommit = useCallback(() => {
        if (canCommit) void onCommit(commitMessage.trim());
    }, [canCommit, commitMessage, onCommit]);

    const handleCommitAndPush = useCallback(() => {
        if (canCommit) void onCommitAndPush(commitMessage.trim());
    }, [canCommit, commitMessage, onCommitAndPush]);

    const handleKeyDown = useCallback(
        (e: React.KeyboardEvent) => {
            if (e.key === "Enter" && (e.ctrlKey || e.metaKey)) {
                e.preventDefault();
                if (e.shiftKey) {
                    handleCommitAndPush();
                } else {
                    handleCommit();
                }
            }
        },
        [handleCommit, handleCommitAndPush],
    );

    const spinnerFor = (op: string) =>
        operationInFlight === op ? " ..." : "";

    // Show distinct phase labels during commitAndPush to indicate progress.
    function getCommitAndPushLabel(op: OperationType): string {
        switch (op) {
            case "commit": return "Committing...";
            case "push":   return "Pushing...";
            default:        return "Commit & Push";
        }
    }
    const commitAndPushLabel = getCommitAndPushLabel(operationInFlight);

    return (
        <div className="commit-panel">
            {branchInfo && (
                <div className="commit-branch-info">
                    <span className="commit-branch-name" title={`Branch: ${branchInfo.branch || (branchInfo.statusFetchFailed ? "(status unavailable)" : "(unknown)")}`}>
                        {branchInfo.statusFetchFailed ? "\u26A0" : ""}{branchInfo.branch || (branchInfo.statusFetchFailed ? "(status unavailable)" : "(detached)")}
                    </span>
                    {branchInfo.upstreamConfigured ? (
                        <>
                            {branchInfo.ahead > 0 && (
                                <span className="commit-branch-ahead" title={`${branchInfo.ahead} ahead`}>
                                    &uarr;{branchInfo.ahead}
                                </span>
                            )}
                            {branchInfo.behind > 0 && (
                                <span className="commit-branch-behind" title={`${branchInfo.behind} behind`}>
                                    &darr;{branchInfo.behind}
                                </span>
                            )}
                        </>
                    ) : (
                        <span className="commit-branch-no-upstream" title="No upstream branch configured">
                            (no upstream)
                        </span>
                    )}
                </div>
            )}
            {hasConflicts && branchInfo && (
                <div className="commit-conflict-warning" title="Resolve conflicts before committing">
                    {branchInfo.conflicted.length} conflicted file(s) — resolve in terminal
                </div>
            )}
            <textarea
                ref={textareaRef}
                className="commit-textarea"
                placeholder="Commit message..."
                value={commitMessage}
                onChange={(e) => onSetCommitMessage(e.target.value)}
                onKeyDown={handleKeyDown}
                rows={3}
                disabled={isDisabled}
            />
            <div className="commit-actions">
                <button
                    type="button"
                    className="commit-btn commit-btn--primary"
                    disabled={!canCommit}
                    title="Commit (Ctrl+Enter)"
                    onClick={handleCommit}
                >
                    Commit{spinnerFor("commit")}
                </button>
                <button
                    type="button"
                    className="commit-btn commit-btn--secondary"
                    disabled={!canCommit}
                    title="Commit & Push (Ctrl+Shift+Enter)"
                    onClick={handleCommitAndPush}
                >
                    {commitAndPushLabel}
                </button>
            </div>
            <div className="commit-actions commit-actions--secondary">
                <button
                    type="button"
                    className="commit-btn commit-btn--small"
                    disabled={isDisabled}
                    title="Push to remote"
                    onClick={() => void onPush()}
                >
                    Push{spinnerFor("push")}
                </button>
                <button
                    type="button"
                    className="commit-btn commit-btn--small"
                    disabled={isDisabled}
                    title="Pull from remote (fast-forward only)"
                    onClick={() => void onPull()}
                >
                    Pull{spinnerFor("pull")}
                </button>
                <button
                    type="button"
                    className="commit-btn commit-btn--small"
                    disabled={isDisabled}
                    title="Fetch from remote"
                    onClick={() => void onFetch()}
                >
                    Fetch{spinnerFor("fetch")}
                </button>
            </div>
        </div>
    );
}
