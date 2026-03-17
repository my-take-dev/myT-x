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
    readonly branch: string;
    readonly ahead: number;
    readonly behind: number;
}
