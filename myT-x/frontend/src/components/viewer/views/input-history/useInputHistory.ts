import {useCallback, useEffect, useRef} from "react";
import {writeClipboardText} from "../../../../utils/clipboardUtils";
import {notifyClipboardFailure} from "../../../../utils/notifyUtils";
import {formatTimestamp} from "../../../../utils/timestampUtils";
import {useInputHistoryStore} from "../../../../stores/inputHistoryStore";
import type {InputHistoryEntry} from "../../../../stores/inputHistoryStore";

/**
 * formatInputForDisplay converts residual control characters to human-readable
 * form for display in the input history panel.
 *
 * CSI/OSC escape sequences are stripped by the backend (processInputString)
 * before storage, so only a minimal set of control characters can appear here.
 */
export function formatInputForDisplay(input: string): string {
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
    // Derive a scalar dependency so the auto-scroll effect doesn't re-fire when
    // entry objects change but no new entries are appended.
    // When entries is cleared (e.g., session switch), latestEntrySeq transitions
    // to undefined, which does trigger React's dependency check.
    const latestEntrySeq = entries.length > 0 ? entries[entries.length - 1].seq : undefined;

    // Auto-scroll to bottom when new entries arrive or on initial mount.
    useEffect(() => {
        const el = bodyRef.current;
        if (!el) return;
        const isNearBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 60;
        if (isNearBottom) {
            el.scrollTop = el.scrollHeight;
        }
    }, [latestEntrySeq]);

    const copyAll = useCallback(async (): Promise<boolean> => {
        if (entries.length === 0) return false;
        const text = entries.map(formatEntryForCopy).join("\n");
        try {
            await writeClipboardText(text);
            return true;
        } catch (err: unknown) {
            notifyClipboardFailure();
            console.warn("[input-history] clipboard write failed", err);
            return false;
        }
    }, [entries]);

    const copyEntry = useCallback(async (entry: InputHistoryEntry): Promise<boolean> => {
        try {
            await writeClipboardText(formatEntryForCopy(entry));
            return true;
        } catch (err: unknown) {
            notifyClipboardFailure();
            console.warn("[input-history] clipboard write failed", err);
            return false;
        }
    }, []);

    return {
        entries,
        unreadCount,
        markAllRead,
        copyAll,
        copyEntry,
        registerBodyElement,
        formatInputForDisplay,
    };
}
