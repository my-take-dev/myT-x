import {create} from "zustand";
import type {DiffLineType} from "../utils/diffParser";
import {buildScopedDiffReviewPrefix} from "../components/viewer/views/diff-view/diffReviewKeys";

export interface DiffReviewComment {
    readonly id: string;
    readonly sessionKey: string;
    readonly filePath: string;
    readonly oldFilePath?: string;
    readonly startLineNum: number;
    readonly startLineType: DiffLineType;
    readonly endLineNum: number;
    readonly endLineType: DiffLineType;
    readonly lineContent: string;
    readonly commentText: string;
}

interface DiffReviewState {
    comments: DiffReviewComment[];
    drafts: Record<string, string>;
    activeCommentLineKey: string | null;
    addComment: (comment: Omit<DiffReviewComment, "id">) => void;
    removeComments: (ids: readonly string[]) => void;
    removeCommentsForSession: (ids: readonly string[], sessionKey: string) => void;
    clearCommentsForSession: (sessionKey: string) => void;
    clearTransientStateForSession: (sessionKey: string) => void;
    clearSessionState: (sessionKey: string) => void;
    setActiveCommentLineKey: (key: string | null) => void;
    setDraft: (lineKey: string, text: string) => void;
    clearDraft: (lineKey: string) => void;
}

let nextFallbackID = 0;

function generateCommentID(): string {
    if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
        return crypto.randomUUID();
    }
    const fallbackID = `${Date.now()}-${nextFallbackID}`;
    nextFallbackID += 1;
    return fallbackID;
}

function stripSessionDrafts(
    drafts: Record<string, string>,
    sessionKey: string,
): {nextDrafts: Record<string, string>; changed: boolean} {
    const prefix = buildScopedDiffReviewPrefix(sessionKey);
    if (prefix === "") {
        return {nextDrafts: drafts, changed: false};
    }

    let changed = false;
    const nextDrafts: Record<string, string> = {};
    for (const [draftKey, draftText] of Object.entries(drafts)) {
        if (draftKey.startsWith(prefix)) {
            changed = true;
            continue;
        }
        nextDrafts[draftKey] = draftText;
    }
    return {nextDrafts, changed};
}

function clearSessionActiveCommentLineKey(activeCommentLineKey: string | null, sessionKey: string): string | null {
    const prefix = buildScopedDiffReviewPrefix(sessionKey);
    if (prefix === "" || activeCommentLineKey == null || !activeCommentLineKey.startsWith(prefix)) {
        return activeCommentLineKey;
    }
    return null;
}

function removeDraft(
    drafts: Record<string, string>,
    lineKey: string,
): {nextDrafts: Record<string, string>; changed: boolean} {
    if (!(lineKey in drafts)) {
        return {nextDrafts: drafts, changed: false};
    }
    const nextDrafts = {...drafts};
    delete nextDrafts[lineKey];
    return {nextDrafts, changed: true};
}

export const useDiffReviewStore = create<DiffReviewState>((set) => ({
    comments: [],
    drafts: {},
    activeCommentLineKey: null,
    addComment: (comment) => {
        const sessionKey = comment.sessionKey.trim();
        const filePath = comment.filePath.trim();
        const commentText = comment.commentText.trim();
        if (sessionKey === "" || filePath === "" || commentText === "") {
            return;
        }
        const id = generateCommentID();
        set((state) => ({
            comments: [...state.comments, {...comment, sessionKey, filePath, commentText, id}],
        }));
    },
    removeComments: (ids) =>
        set((state) => {
            if (ids.length === 0) return state;
            const sentIDs = new Set(ids);
            return {
                comments: state.comments.filter((comment) => !sentIDs.has(comment.id)),
            };
        }),
    removeCommentsForSession: (ids, sessionKey) =>
        set((state) => {
            if (ids.length === 0 || sessionKey === "") return state;
            const sentIDs = new Set(ids);
            return {
                comments: state.comments.filter(
                    (comment) => comment.sessionKey !== sessionKey || !sentIDs.has(comment.id),
                ),
            };
        }),
    clearCommentsForSession: (sessionKey) =>
        set((state) => {
            if (sessionKey === "") return state;
            const nextComments = state.comments.filter((comment) => comment.sessionKey !== sessionKey);
            if (nextComments.length === state.comments.length) return state;
            return {comments: nextComments};
        }),
    clearTransientStateForSession: (sessionKey) =>
        set((state) => {
            if (sessionKey === "") return state;
            const {nextDrafts, changed: draftsChanged} = stripSessionDrafts(state.drafts, sessionKey);
            const nextActiveCommentLineKey = clearSessionActiveCommentLineKey(state.activeCommentLineKey, sessionKey);
            if (!draftsChanged && nextActiveCommentLineKey === state.activeCommentLineKey) {
                return state;
            }

            return {
                drafts: draftsChanged ? nextDrafts : state.drafts,
                activeCommentLineKey: nextActiveCommentLineKey,
            };
        }),
    clearSessionState: (sessionKey) =>
        set((state) => {
            if (sessionKey === "") return state;
            const nextComments = state.comments.filter((comment) => comment.sessionKey !== sessionKey);
            const {nextDrafts, changed: draftsChanged} = stripSessionDrafts(state.drafts, sessionKey);
            const nextActiveCommentLineKey = clearSessionActiveCommentLineKey(state.activeCommentLineKey, sessionKey);
            if (
                nextComments.length === state.comments.length
                && !draftsChanged
                && nextActiveCommentLineKey === state.activeCommentLineKey
            ) {
                return state;
            }

            return {
                comments: nextComments,
                drafts: draftsChanged ? nextDrafts : state.drafts,
                activeCommentLineKey: nextActiveCommentLineKey,
            };
        }),
    setActiveCommentLineKey: (key) => set({activeCommentLineKey: key}),
    setDraft: (lineKey, text) =>
        set((state) => {
            return {
                drafts: {...state.drafts, [lineKey]: text},
            };
        }),
    clearDraft: (lineKey) =>
        set((state) => {
            const {nextDrafts, changed} = removeDraft(state.drafts, lineKey);
            if (!changed) return state;
            return {drafts: nextDrafts};
        }),
}));
