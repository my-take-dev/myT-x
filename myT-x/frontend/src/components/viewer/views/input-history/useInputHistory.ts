import {useCallback, useEffect, useRef} from "react";
import {ClipboardSetText} from "../../../../../wailsjs/runtime/runtime";
import {useInputHistoryStore} from "../../../../stores/inputHistoryStore";
import type {InputHistoryEntry} from "../../../../stores/inputHistoryStore";

function formatTimestamp(ts: string): string {
    if (ts.length !== 14) return ts;
    return `${ts.slice(0, 4)}-${ts.slice(4, 6)}-${ts.slice(6, 8)} ${ts.slice(8, 10)}:${ts.slice(10, 12)}:${ts.slice(12, 14)}`;
}

/**
 * formatInputForDisplay converts residual control characters to human-readable
 * form for display in the input history panel.
 *
 * CSI/OSC escape sequences are stripped by the backend (processInputString)
 * before storage, so only a minimal set of control characters can appear here.
 */
function formatInputForDisplay(input: string): string {
    return input.replace(/[\x00-\x1f\x7f]/g, (ch) => {
        const code = ch.charCodeAt(0);
        if (code === 0x7f) {
            return "^?";
        }
        return "^" + String.fromCharCode(code + 64);
    });
}

function formatEntryForCopy(entry: InputHistoryEntry): string {
    return `${formatTimestamp(entry.ts)} [${entry.pane_id}] ${formatInputForDisplay(entry.input)} (${entry.source})`;
}

export function useInputHistory() {
    const entries = useInputHistoryStore((s) => s.entries);
    const unreadCount = useInputHistoryStore((s) => s.unreadCount);
    const markAllRead = useInputHistoryStore((s) => s.markAllRead);
    const bodyRef = useRef<HTMLDivElement | null>(null);
    const registerBodyElement = useCallback((el: HTMLDivElement | null) => {
        bodyRef.current = el;
    }, []);

    // Auto-scroll to bottom when new entries arrive or on initial mount.
    // registerBodyElement is omitted from deps: it is a stable useCallback(fn,[]) reference
    // and its inclusion would mask the real dependency (entries.length).
    // eslint-disable-next-line react-hooks/exhaustive-deps
    useEffect(() => {
        const el = bodyRef.current;
        if (!el) return;
        const isNearBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 60;
        if (isNearBottom) {
            el.scrollTop = el.scrollHeight;
        }
    }, [entries.length]);

    const copyAll = useCallback(() => {
        if (entries.length === 0) return;
        const text = entries.map(formatEntryForCopy).join("\n");
        void ClipboardSetText(text).catch((err: unknown) => {
            if (import.meta.env.DEV) {
                console.warn("[input-history] clipboard write failed", err);
            }
        });
    }, [entries]);

    const copyEntry = useCallback((entry: InputHistoryEntry) => {
        void ClipboardSetText(formatEntryForCopy(entry)).catch((err: unknown) => {
            if (import.meta.env.DEV) {
                console.warn("[input-history] clipboard write failed", err);
            }
        });
    }, []);

    return {
        entries,
        unreadCount,
        markAllRead,
        copyAll,
        copyEntry,
        registerBodyElement,
        formatTimestamp,
        formatInputForDisplay,
    };
}
