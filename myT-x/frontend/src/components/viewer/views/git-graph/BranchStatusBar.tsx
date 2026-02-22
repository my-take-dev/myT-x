import type { GitStatusResult } from "./gitGraphTypes";

interface BranchStatusBarProps {
  status: GitStatusResult | null;
}

export function BranchStatusBar({ status }: BranchStatusBarProps) {
  if (!status) {
    return null;
  }

  const modifiedCount = status.modified?.length ?? 0;
  const stagedCount = status.staged?.length ?? 0;
  const untrackedCount = status.untracked?.length ?? 0;

  return (
    <div className="git-status-bar">
      <span className="git-status-branch">
        {"\u2387"} {status.branch || "unknown"}
      </span>

      <span className="git-status-counts">
        {status.ahead > 0 && (
          <span className="git-status-ahead">{"\u2191"}{status.ahead}</span>
        )}
        {status.behind > 0 && (
          <span className="git-status-behind">{"\u2193"}{status.behind}</span>
        )}
      </span>

      <span className="git-status-changes">
        {modifiedCount > 0 && (
          <span className="git-status-modified">{modifiedCount} modified</span>
        )}
        {stagedCount > 0 && (
          <span className="git-status-staged">{stagedCount} staged</span>
        )}
        {untrackedCount > 0 && (
          <span className="git-status-untracked">{untrackedCount} untracked</span>
        )}
      </span>
    </div>
  );
}
