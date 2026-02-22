import {useCallback, useEffect, useRef} from "react";
import {ClipboardSetText} from "../../../../../wailsjs/runtime/runtime";
import {useErrorLogStore} from "../../../../stores/errorLogStore";
import type {ErrorLogEntry} from "../../../../stores/errorLogStore";

function formatTimestamp(ts: string): string {
    if (ts.length !== 14) return ts;
    // "20260221120000" -> "2026-02-21 12:00:00"
    return `${ts.slice(0, 4)}-${ts.slice(4, 6)}-${ts.slice(6, 8)} ${ts.slice(8, 10)}:${ts.slice(10, 12)}:${ts.slice(12, 14)}`;
}

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

    // Auto-scroll to bottom when new entries arrive.
    useEffect(() => {
        const el = bodyRef.current;
        if (!el) return;
        // Only auto-scroll if already near the bottom.
        const isNearBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 60;
        if (isNearBottom) {
            el.scrollTop = el.scrollHeight;
        }
    }, [entries.length, registerBodyElement]);

    const copyAll = useCallback(() => {
        if (entries.length === 0) return;
        const text = entries.map(formatEntryForCopy).join("\n");
        void ClipboardSetText(text).catch((err: unknown) => {
            if (import.meta.env.DEV) {
                console.warn("[error-log] clipboard write failed", err);
            }
        });
    }, [entries]);

    const copyEntry = useCallback((entry: ErrorLogEntry) => {
        void ClipboardSetText(formatEntryForCopy(entry)).catch((err: unknown) => {
            if (import.meta.env.DEV) {
                console.warn("[error-log] clipboard write failed", err);
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
    };
}
