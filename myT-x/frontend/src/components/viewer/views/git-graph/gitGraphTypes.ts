/** Backend GitGraphCommit returned by DevPanelGitLog. */
export interface GitGraphCommit {
  hash: string;
  full_hash: string;
  parents: string[] | null;
  subject: string;
  author_name: string;
  author_date: string;
  refs: string[] | null;
}

/** Backend GitStatusResult returned by DevPanelGitStatus. */
export interface GitStatusResult {
  branch: string;
  modified: string[] | null;
  staged: string[] | null;
  untracked: string[] | null;
  ahead: number;
  behind: number;
}

/** Lane assignment for a single commit in the graph. */
export interface LaneAssignment {
  commitHash: string;
  lane: number;
  connections: ParentConnection[];
  activeLaneCount: number;
}

/** Connection line from a commit to one of its parents. */
export interface ParentConnection {
  fromLane: number;
  toLane: number;
  type: "straight" | "merge-left" | "merge-right";
}
