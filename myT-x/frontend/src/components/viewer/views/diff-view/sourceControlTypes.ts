import type {WorkingDiffFile} from "./diffViewTypes";

/** Staging group classification. */
export type StagingGroup = "staged" | "unstaged";

/** Sidebar display mode toggle. */
export type DiffSidebarMode = "tree" | "flat";

/** A file entry in the flat staging list. */
export interface StagingFlatNode {
    readonly type: "file";
    readonly file: WorkingDiffFile;
    readonly group: StagingGroup;
}

/** A group header row in the flat staging list. */
export interface StagingGroupHeaderNode {
    readonly type: "group-header";
    readonly group: StagingGroup;
    readonly count: number;
    readonly isExpanded: boolean;
}

/** Union type for rows in the flat staging virtual list. */
export type StagingListItem = StagingGroupHeaderNode | StagingFlatNode;

/** Discriminated type for in-flight git operations (prevents concurrent mutations). */
export type OperationType =
    | "stage"
    | "unstage"
    | "discard"
    | "stageAll"
    | "unstageAll"
    | "commit"
    | "push"
    | "pull"
    | "fetch"
    | null;

/** Branch information from DevPanelGitStatus. */
export interface BranchInfo {
    /** Branch name. Empty string indicates a fresh repo with no commits yet —
     *  used by useGitOperations to choose full vs. lightweight refresh on staging. */
    readonly branch: string;
    readonly ahead: number;
    readonly behind: number;
    /** True when the branch has a resolvable upstream and ahead/behind counts are valid.
     *  When false, ahead/behind are 0 = "unknown", not "no diff". */
    readonly upstreamConfigured: boolean;
    /** Paths currently in a merge-conflict state (UU, AA, DD, etc). */
    readonly conflicted: readonly string[];
    /** True when the git status fetch failed and branch/ahead/behind data may be stale or empty. */
    readonly statusFetchFailed: boolean;
}
