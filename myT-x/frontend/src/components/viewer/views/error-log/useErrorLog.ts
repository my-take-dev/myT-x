import {useCallback, useEffect, useRef} from "react";
import {writeClipboardText} from "../../../../utils/clipboardUtils";
import {notifyClipboardFailure} from "../../../../utils/notifyUtils";
import {formatTimestamp} from "../../../../utils/timestampUtils";
import {useErrorLogStore} from "../../../../stores/errorLogStore";
import type {ErrorLogEntry} from "../../../../stores/errorLogStore";

function formatEntryForCopy(entry: ErrorLogEntry): string {
    return `${formatTimestamp(entry.ts)}: ${entry.level}, ${entry.msg}${entry.source ? ` [${entry.source}]` : ""}`;
}

export function useErrorLog() {
    const entries = useErrorLogStore((s) => s.entries);
    const unreadCount = useErrorLogStore((s) => s.unreadCount);
    const markAllRead = useErrorLogStore((s) => s.markAllRead);
    const bodyRef = useRef<HTMLDivElement | null>(null);
    const registerBodyElement = useCallback((el: HTMLDivElement | null) => {
        bodyRef.current = el;
    }, []);
    const latestEntrySeq = entries.length > 0 ? entries[entries.length - 1]?.seq : undefined;

    // Auto-scroll to bottom when new entries arrive.
    useEffect(() => {
        const el = bodyRef.current;
        if (!el) return;
        // Only auto-scroll if already near the bottom.
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
            console.warn("[error-log] clipboard write failed", err);
            return false;
        }
    }, [entries]);

    const copyEntry = useCallback(async (entry: ErrorLogEntry): Promise<boolean> => {
        try {
            await writeClipboardText(formatEntryForCopy(entry));
            return true;
        } catch (err: unknown) {
            notifyClipboardFailure();
            console.warn("[error-log] clipboard write failed", err);
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
    };
}
