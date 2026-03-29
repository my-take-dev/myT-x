import type {Dispatch} from "react";
import type {git} from "../../../wailsjs/go/models";

export type WorktreeSource = "existing" | "new";

export interface NewSessionState {
    // Directory / identity
    readonly directory: string;
    readonly sessionName: string;

    // Git detection
    readonly isGitRepo: boolean;
    readonly currentBranch: string;
    readonly branches: readonly string[];
    readonly worktrees: readonly git.WorktreeInfo[];

    // Worktree configuration
    readonly useWorktree: boolean;
    readonly worktreeSource: WorktreeSource;
    readonly selectedWorktree: git.WorktreeInfo | null;
    readonly worktreeConflict: string;
    readonly directoryConflict: string;
    readonly baseBranch: string;
    readonly branchName: string;
    readonly pullBefore: boolean;

    // Session options
    readonly enableAgentTeam: boolean;
    readonly useClaudeEnv: boolean;
    readonly usePaneEnv: boolean;
    readonly useSessionPaneScope: boolean;
    readonly shimAvailable: boolean;

    // Loading / error
    readonly loading: boolean;
    readonly gitCheckLoading: boolean;
    readonly worktreeDataLoading: boolean;
    readonly error: string;
    readonly configLoadFailed: boolean;
}

export type SetFieldAction = {
    [K in keyof NewSessionState]: { type: "SET_FIELD"; field: K; value: NewSessionState[K] };
}[keyof NewSessionState];

export type NewSessionAction =
    | { type: "RESET" }
    | SetFieldAction
    | { type: "START_SUBMIT" }
    | { type: "PICK_DIRECTORY"; directory: string; sessionName: string }
    | { type: "LOAD_GIT_DATA"; branches: string[]; worktrees: git.WorktreeInfo[]; baseBranch: string }
    | { type: "SELECT_WORKTREE"; worktree: git.WorktreeInfo; sessionName: string };

export type NewSessionDispatch = Dispatch<NewSessionAction>;
