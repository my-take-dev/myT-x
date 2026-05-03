import {create} from "zustand";

interface SessionMemoDraft {
    readonly content: string;
    readonly savedContent: string;
    readonly loaded: boolean;
}

interface SessionMemoState {
    readonly drafts: Readonly<Record<string, SessionMemoDraft>>;
    initializeMemo: (sessionKey: string, content: string, force?: boolean) => void;
    setMemoContent: (sessionKey: string, content: string) => void;
    markSaved: (sessionKey: string, savedContent: string) => void;
    moveDraft: (fromSessionKey: string, toSessionKey: string) => void;
    removeDraft: (sessionKey: string) => void;
}

// UI drafts use session instance identity so rename migration preserves unsaved
// text; persisted memo identity is resolved by the backend from the root path.
export function buildSessionMemoDraftKey(sessionName: string, sessionID: number): string {
    return `${sessionName}:${sessionID}`;
}

function createDraft(content: string, loaded: boolean): SessionMemoDraft {
    return {
        content,
        savedContent: content,
        loaded,
    };
}

export const useSessionMemoStore = create<SessionMemoState>((set) => ({
    drafts: {},
    initializeMemo: (sessionKey, content, force = false) => set((state) => {
        const existing = state.drafts[sessionKey];
        if (existing?.loaded && !force) {
            return state;
        }
        const hasUnsavedContent = existing ? existing.content !== existing.savedContent : false;
        return {
            drafts: {
                ...state.drafts,
                [sessionKey]: existing
                    ? {
                        ...existing,
                        content: hasUnsavedContent ? existing.content : content,
                        savedContent: content,
                        loaded: true,
                    }
                    : createDraft(content, true),
            },
        };
    }),
    setMemoContent: (sessionKey, content) => set((state) => {
        const existing = state.drafts[sessionKey] ?? createDraft("", false);
        return {
            drafts: {
                ...state.drafts,
                [sessionKey]: {
                    ...existing,
                    content,
                },
            },
        };
    }),
    markSaved: (sessionKey, savedContent) => set((state) => {
        const existing = state.drafts[sessionKey] ?? createDraft(savedContent, true);
        return {
            drafts: {
                ...state.drafts,
                [sessionKey]: {
                    ...existing,
                    savedContent,
                    loaded: true,
                },
            },
        };
    }),
    moveDraft: (fromSessionKey, toSessionKey) => set((state) => {
        if (fromSessionKey === toSessionKey || !state.drafts[fromSessionKey] || state.drafts[toSessionKey]) {
            return state;
        }
        const {[fromSessionKey]: draft, ...remainingDrafts} = state.drafts;
        return {
            drafts: {
                ...remainingDrafts,
                [toSessionKey]: draft,
            },
        };
    }),
    removeDraft: (sessionKey) => set((state) => {
        if (!state.drafts[sessionKey]) {
            return state;
        }
        const {[sessionKey]: _removedDraft, ...remainingDrafts} = state.drafts;
        return {drafts: remainingDrafts};
    }),
}));
