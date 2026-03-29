import {describe, expect, it} from "vitest";
import {INITIAL_STATE, newSessionReducer} from "./newSessionReducer";
import type {NewSessionAction, NewSessionState} from "./types";
import type {git} from "../../../wailsjs/go/models";

describe("newSessionReducer", () => {
    describe("RESET", () => {
        it("returns a fresh INITIAL_STATE", () => {
            const dirty: NewSessionState = {
                ...INITIAL_STATE,
                directory: "/some/path",
                sessionName: "test-session",
                isGitRepo: true,
                loading: true,
                error: "some error",
            };
            const result = newSessionReducer(dirty, {type: "RESET"});
            expect(result).toEqual(INITIAL_STATE);
            expect(result).not.toBe(INITIAL_STATE); // fresh object, not shared ref
        });
    });

    describe("SET_FIELD", () => {
        it("updates a string field", () => {
            const result = newSessionReducer(INITIAL_STATE, {
                type: "SET_FIELD", field: "directory", value: "/new/path",
            });
            expect(result.directory).toBe("/new/path");
        });

        it("updates a boolean field", () => {
            const result = newSessionReducer(INITIAL_STATE, {
                type: "SET_FIELD", field: "isGitRepo", value: true,
            });
            expect(result.isGitRepo).toBe(true);
        });

        it("updates an array field", () => {
            const branches = ["main", "develop"];
            const result = newSessionReducer(INITIAL_STATE, {
                type: "SET_FIELD", field: "branches", value: branches,
            });
            expect(result.branches).toEqual(branches);
        });

        it("updates a nullable object field", () => {
            const wt = {path: "/wt", branch: "feat", isMain: false, isDetached: false} as git.WorktreeInfo;
            const result = newSessionReducer(INITIAL_STATE, {
                type: "SET_FIELD", field: "selectedWorktree", value: wt,
            });
            expect(result.selectedWorktree).toEqual(wt);
        });

        it("preserves other fields", () => {
            const state: NewSessionState = {...INITIAL_STATE, directory: "/keep"};
            const result = newSessionReducer(state, {
                type: "SET_FIELD", field: "error", value: "fail",
            });
            expect(result.directory).toBe("/keep");
            expect(result.error).toBe("fail");
        });
    });

    describe("START_SUBMIT", () => {
        it("sets loading to true and clears error", () => {
            const state: NewSessionState = {...INITIAL_STATE, error: "old error", loading: false};
            const result = newSessionReducer(state, {type: "START_SUBMIT"});
            expect(result.loading).toBe(true);
            expect(result.error).toBe("");
        });

        it("preserves other fields", () => {
            const state: NewSessionState = {...INITIAL_STATE, directory: "/keep", sessionName: "s1"};
            const result = newSessionReducer(state, {type: "START_SUBMIT"});
            expect(result.directory).toBe("/keep");
            expect(result.sessionName).toBe("s1");
        });
    });

    describe("PICK_DIRECTORY", () => {
        it("sets directory and sessionName", () => {
            const result = newSessionReducer(INITIAL_STATE, {
                type: "PICK_DIRECTORY", directory: "/proj", sessionName: "proj",
            });
            expect(result.directory).toBe("/proj");
            expect(result.sessionName).toBe("proj");
        });

        it("resets worktree-related fields", () => {
            const dirty: NewSessionState = {
                ...INITIAL_STATE,
                error: "old",
                worktreeSource: "existing",
                selectedWorktree: {path: "/wt", branch: "b", isMain: false, isDetached: false} as git.WorktreeInfo,
                worktreeConflict: "conflict",
                branches: ["main"],
                worktrees: [{path: "/wt", branch: "b", isMain: false, isDetached: false} as git.WorktreeInfo],
                useWorktree: true,
            };
            const result = newSessionReducer(dirty, {
                type: "PICK_DIRECTORY", directory: "/new", sessionName: "new",
            });
            expect(result.error).toBe("");
            expect(result.worktreeSource).toBe("new");
            expect(result.selectedWorktree).toBeNull();
            expect(result.worktreeConflict).toBe("");
            expect(result.branches).toEqual([]);
            expect(result.worktrees).toEqual([]);
            expect(result.useWorktree).toBe(false);
        });

        it("resets directory-check fields (I-01)", () => {
            const dirty: NewSessionState = {
                ...INITIAL_STATE,
                directoryConflict: "old-session",
                isGitRepo: true,
                currentBranch: "main",
                gitCheckLoading: true,
            };
            const result = newSessionReducer(dirty, {
                type: "PICK_DIRECTORY", directory: "/new", sessionName: "new",
            });
            expect(result.directoryConflict).toBe("");
            expect(result.isGitRepo).toBe(false);
            expect(result.currentBranch).toBe("");
        });

        it("preserves unrelated fields", () => {
            const state: NewSessionState = {...INITIAL_STATE, shimAvailable: true, useClaudeEnv: true};
            const result = newSessionReducer(state, {
                type: "PICK_DIRECTORY", directory: "/d", sessionName: "d",
            });
            expect(result.shimAvailable).toBe(true);
            expect(result.useClaudeEnv).toBe(true);
        });
    });

    describe("LOAD_GIT_DATA", () => {
        it("updates branches, worktrees, and baseBranch", () => {
            const branches = ["main", "develop"];
            const worktrees = [
                {path: "/wt1", branch: "feat", isMain: false, isDetached: false} as git.WorktreeInfo,
            ];
            const result = newSessionReducer(INITIAL_STATE, {
                type: "LOAD_GIT_DATA", branches, worktrees, baseBranch: "main",
            });
            expect(result.branches).toEqual(branches);
            expect(result.worktrees).toEqual(worktrees);
            expect(result.baseBranch).toBe("main");
        });

        it("preserves other fields", () => {
            const state: NewSessionState = {
                ...INITIAL_STATE,
                directory: "/proj",
                worktreeDataLoading: true,
            };
            const result = newSessionReducer(state, {
                type: "LOAD_GIT_DATA", branches: [], worktrees: [], baseBranch: "",
            });
            expect(result.directory).toBe("/proj");
            // worktreeDataLoading is managed by the effect, not the action
            expect(result.worktreeDataLoading).toBe(true);
        });
    });

    describe("SELECT_WORKTREE", () => {
        it("updates selectedWorktree and sessionName when sessionName is truthy", () => {
            const wt = {path: "/wt", branch: "feature/login", isMain: false, isDetached: false} as git.WorktreeInfo;
            const result = newSessionReducer(INITIAL_STATE, {
                type: "SELECT_WORKTREE", worktree: wt, sessionName: "feature-login",
            });
            expect(result.selectedWorktree).toEqual(wt);
            expect(result.sessionName).toBe("feature-login");
        });

        it("preserves sessionName when action.sessionName is empty string", () => {
            const state: NewSessionState = {...INITIAL_STATE, sessionName: "keep-this"};
            const wt = {path: "/wt", branch: "", isMain: false, isDetached: true} as git.WorktreeInfo;
            const result = newSessionReducer(state, {
                type: "SELECT_WORKTREE", worktree: wt, sessionName: "",
            });
            expect(result.selectedWorktree).toEqual(wt);
            expect(result.sessionName).toBe("keep-this");
        });

        it("preserves other fields", () => {
            const state: NewSessionState = {
                ...INITIAL_STATE,
                directory: "/proj",
                isGitRepo: true,
                useWorktree: true,
            };
            const wt = {path: "/wt", branch: "feat", isMain: false, isDetached: false} as git.WorktreeInfo;
            const result = newSessionReducer(state, {
                type: "SELECT_WORKTREE", worktree: wt, sessionName: "feat",
            });
            expect(result.directory).toBe("/proj");
            expect(result.isGitRepo).toBe(true);
            expect(result.useWorktree).toBe(true);
        });
    });

    describe("INITIAL_STATE", () => {
        it("covers all fields (field-count guard)", () => {
            expect(Object.keys(INITIAL_STATE)).toHaveLength(24);
        });

        it("has correct default values for all fields", () => {
            expect(INITIAL_STATE.directory).toBe("");
            expect(INITIAL_STATE.sessionName).toBe("");
            expect(INITIAL_STATE.isGitRepo).toBe(false);
            expect(INITIAL_STATE.currentBranch).toBe("");
            expect(INITIAL_STATE.branches).toEqual([]);
            expect(INITIAL_STATE.worktrees).toEqual([]);
            expect(INITIAL_STATE.useWorktree).toBe(false);
            expect(INITIAL_STATE.worktreeSource).toBe("new");
            expect(INITIAL_STATE.selectedWorktree).toBeNull();
            expect(INITIAL_STATE.worktreeConflict).toBe("");
            expect(INITIAL_STATE.directoryConflict).toBe("");
            expect(INITIAL_STATE.baseBranch).toBe("");
            expect(INITIAL_STATE.branchName).toBe("");
            expect(INITIAL_STATE.pullBefore).toBe(true);
            expect(INITIAL_STATE.enableAgentTeam).toBe(false);
            expect(INITIAL_STATE.useClaudeEnv).toBe(false);
            expect(INITIAL_STATE.usePaneEnv).toBe(false);
            expect(INITIAL_STATE.useSessionPaneScope).toBe(true);
            expect(INITIAL_STATE.shimAvailable).toBe(false);
            expect(INITIAL_STATE.loading).toBe(false);
            expect(INITIAL_STATE.gitCheckLoading).toBe(false);
            expect(INITIAL_STATE.worktreeDataLoading).toBe(false);
            expect(INITIAL_STATE.error).toBe("");
            expect(INITIAL_STATE.configLoadFailed).toBe(false);
        });
    });
});
