import type {NewSessionAction, NewSessionState} from "./types";

export const INITIAL_STATE: NewSessionState = {
    directory: "",
    sessionName: "",
    isGitRepo: false,
    currentBranch: "",
    branches: [],
    worktrees: [],
    useWorktree: false,
    worktreeSource: "new",
    selectedWorktree: null,
    worktreeConflict: "",
    directoryConflict: "",
    baseBranch: "",
    branchName: "",
    pullBefore: true,
    enableAgentTeam: false,
    useClaudeEnv: false,
    usePaneEnv: false,
    useSessionPaneScope: true,
    shimAvailable: false,
    loading: false,
    gitCheckLoading: false,
    worktreeDataLoading: false,
    error: "",
    configLoadFailed: false,
};

export function newSessionReducer(state: NewSessionState, action: NewSessionAction): NewSessionState {
    switch (action.type) {
        case "RESET":
            return {...INITIAL_STATE};
        case "SET_FIELD":
            return {...state, [action.field]: action.value};
        case "START_SUBMIT":
            return {...state, loading: true, error: ""};
        case "PICK_DIRECTORY":
            return {
                ...state,
                directory: action.directory,
                sessionName: action.sessionName,
                error: "",
                directoryConflict: "",
                isGitRepo: false,
                currentBranch: "",
                worktreeSource: "new",
                selectedWorktree: null,
                worktreeConflict: "",
                branches: [],
                worktrees: [],
                useWorktree: false,
            };
        case "LOAD_GIT_DATA":
            return {
                ...state,
                branches: action.branches,
                worktrees: action.worktrees,
                baseBranch: action.baseBranch,
            };
        case "SELECT_WORKTREE":
            return {
                ...state,
                selectedWorktree: action.worktree,
                ...(action.sessionName ? {sessionName: action.sessionName} : {}),
            };
        default: {
            const _exhaustive: never = action;
            void _exhaustive;
            return state;
        }
    }
}
