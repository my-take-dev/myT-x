import {create} from "zustand";

export interface InputHistoryEntry {
    /** Backend sequence number (required for stable identity and unread tracking). */
    seq: number;
    ts: string;
    pane_id: string;
    input: string;
    source: string;
    session: string;
}

export interface InputHistorySnapshot {
    scope_key: string;
    entries: InputHistoryEntry[];
}

// MAX_ENTRIES caps in-memory history to prevent unbounded growth.
const MAX_ENTRIES = 10000;

function isValidSeq(seq: unknown): seq is number {
    // seq > 0: Backend assigns a positive sequence per history scope.
    // Therefore the Go zero value (0) can never appear as a valid seq.
    return typeof seq === "number" && Number.isFinite(seq) && seq > 0;
}

function isValidEntryShape(entry: InputHistoryEntry): boolean {
    return (
        isValidSeq(entry.seq) &&
        typeof entry.ts === "string" &&
        typeof entry.pane_id === "string" &&
        typeof entry.input === "string" &&
        typeof entry.source === "string" &&
        typeof entry.session === "string"
    );
}

// NOTE: When eviction occurs (entries exceed MAX_ENTRIES), the oldest entries
// are discarded. Unread count is recomputed from the current entries array
// on every snapshot update, so evicted unread entries are silently aged out.
function normalizeEntries(incoming: InputHistoryEntry[]): InputHistoryEntry[] {
    return incoming
        .filter((entry) => isValidEntryShape(entry))
        .sort((a, b) => a.seq - b.seq)
        .slice(-MAX_ENTRIES);
}

interface InputHistoryState {
    scopeKey: string;
    entries: InputHistoryEntry[];
    unreadCount: number;
    lastReadSeq: number;
    readSeqByScope: Record<string, number>;
    setSnapshot: (snapshot: InputHistorySnapshot) => void;
    markAllRead: () => void;
}

function applyEntriesForScope(
    state: InputHistoryState,
    incoming: InputHistoryEntry[],
    scopeKey: string,
): Pick<InputHistoryState, "scopeKey" | "entries" | "unreadCount" | "lastReadSeq" | "readSeqByScope"> {
    const entries = normalizeEntries(incoming);
    if (import.meta.env.DEV && incoming.length > 0 && entries.length === 0) {
        console.warn("[input-history] dropped all incoming entries due to invalid shape", incoming);
    }
    const latestEntry = entries.length > 0 ? entries[entries.length - 1] : undefined;
    const maxNewSeq = latestEntry?.seq ?? 0;
    const scopeChanged = state.scopeKey !== scopeKey;
    const storedReadSeq = state.readSeqByScope[scopeKey];

    let lastReadSeq = state.lastReadSeq;
    if (scopeChanged) {
        lastReadSeq = storedReadSeq ?? maxNewSeq;
    } else if (maxNewSeq > 0 && maxNewSeq < lastReadSeq) {
        // Backend restart resets seq to 1. When seq regresses, align the marker
        // to the latest entry so pre-existing entries are treated as already read.
        lastReadSeq = maxNewSeq;
    } else if (state.entries.length === 0 && state.lastReadSeq === 0 && maxNewSeq > 0) {
        // On first load, treat pre-existing history as already read.
        lastReadSeq = maxNewSeq;
    }

    // Entries are sorted by seq (ascending). Scan from end for unread count.
    let unreadCount = 0;
    for (let i = entries.length - 1; i >= 0; i--) {
        if (entries[i].seq <= lastReadSeq) break;
        unreadCount++;
    }
    return {
        scopeKey,
        entries,
        unreadCount,
        lastReadSeq,
        readSeqByScope: {
            ...state.readSeqByScope,
            [scopeKey]: lastReadSeq,
        },
    };
}

export const useInputHistoryStore = create<InputHistoryState>((set) => ({
    scopeKey: "",
    entries: [],
    unreadCount: 0,
    lastReadSeq: 0,
    readSeqByScope: {},
    setSnapshot: (snapshot) =>
        set((state) => {
            const scopeKey = typeof snapshot.scope_key === "string" ? snapshot.scope_key : "";
            const incoming = Array.isArray(snapshot.entries) ? snapshot.entries : [];
            return applyEntriesForScope(state, incoming, scopeKey);
        }),
    markAllRead: () =>
        set((state) => {
            const latestEntry = state.entries.length > 0 ? state.entries[state.entries.length - 1] : undefined;
            const lastReadSeq = latestEntry?.seq ?? state.lastReadSeq;
            return {
                unreadCount: 0,
                lastReadSeq,
                readSeqByScope: {
                    ...state.readSeqByScope,
                    [state.scopeKey]: lastReadSeq,
                },
            };
        }),
}));
