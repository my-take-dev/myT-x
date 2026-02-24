import {create} from "zustand";

export interface ErrorLogEntry {
    ts: string;
    level: string;
    msg: string;
    source: string;
    /** Backend sequence number (required for stable identity and unread tracking). */
    seq: number;
}

// MAX_ENTRIES caps in-memory history to prevent unbounded growth.
const MAX_ENTRIES = 10000;

function isValidSeq(seq: unknown): seq is number {
    // seq > 0: Backend assigns seq via a.sessionLogSeq++ before copying to entry.
    // Therefore the Go zero value (0) can never appear as a valid seq.
    return typeof seq === "number" && Number.isFinite(seq) && seq > 0;
}

function isValidEntryShape(entry: ErrorLogEntry): boolean {
    return (
        isValidSeq(entry.seq) &&
        typeof entry.ts === "string" &&
        typeof entry.msg === "string" &&
        typeof entry.level === "string" &&
        typeof entry.source === "string"
    );
}

// NOTE: When eviction occurs (entries exceed MAX_ENTRIES), the oldest entries
// are discarded. If evicted entries were unread, the unread count decreases
// accordingly because unreadCount is recomputed from the current entries array
// on every setEntries call. This is the intended behavior: very old unread
// entries are silently aged out.
function normalizeEntries(incoming: ErrorLogEntry[]): ErrorLogEntry[] {
    return incoming
        .filter((entry) => isValidEntryShape(entry))
        .sort((a, b) => a.seq - b.seq)
        .slice(-MAX_ENTRIES);
}

interface ErrorLogState {
    entries: ErrorLogEntry[];
    unreadCount: number;
    lastReadSeq: number;
    setEntries: (entries: ErrorLogEntry[]) => void;
    markAllRead: () => void;
}

export const useErrorLogStore = create<ErrorLogState>((set) => ({
    entries: [],
    unreadCount: 0,
    lastReadSeq: 0,
    setEntries: (incoming) =>
        set((state) => {
            const entries = normalizeEntries(incoming);
            if (import.meta.env.DEV && incoming.length > 0 && entries.length === 0) {
                console.warn("[error-log] dropped all incoming entries due to invalid shape", incoming);
            }
            const latestEntry = entries.length > 0 ? entries[entries.length - 1] : undefined;
            const maxNewSeq = latestEntry?.seq ?? 0;

            let lastReadSeq = state.lastReadSeq;
            // Backend restart resets seq to 1. If the latest seq regresses, align read marker to new max.
            if (maxNewSeq > 0 && maxNewSeq < lastReadSeq) {
                lastReadSeq = maxNewSeq;
            }
            // On first load, treat pre-existing history as already read.
            // The badge should represent only entries that arrive after the view model starts.
            if (state.entries.length === 0 && state.lastReadSeq === 0 && maxNewSeq > 0) {
                lastReadSeq = maxNewSeq;
            }

            // Entries are sorted by seq (ascending). Scan from end for unread count.
            let unreadCount = 0;
            for (let i = entries.length - 1; i >= 0; i--) {
                if (entries[i].seq <= lastReadSeq) break;
                unreadCount++;
            }
            return {entries, unreadCount, lastReadSeq};
        }),
    markAllRead: () =>
        set((state) => {
            const latestEntry = state.entries.length > 0 ? state.entries[state.entries.length - 1] : undefined;
            const lastReadSeq = latestEntry?.seq ?? state.lastReadSeq;
            return {unreadCount: 0, lastReadSeq};
        }),
}));
