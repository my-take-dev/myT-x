import {useCallback, useEffect, useLayoutEffect, useRef, useState} from "react";
import {ClipboardSetText} from "../../../../../wailsjs/runtime/runtime";
import {useViewerStore} from "../../viewerStore";
import {useErrorLog} from "./useErrorLog";

export function ErrorLogView() {
    const closeView = useViewerStore((s) => s.closeView);
    const {entries, markAllRead, copyAll, copyEntry, registerBodyElement, formatTimestamp} = useErrorLog();
    const [allCopied, setAllCopied] = useState(false);
    const copyTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

    // Intentional: while this view is open, newly appended entries are immediately marked as read.
    useLayoutEffect(() => {
        markAllRead();
    }, [entries, markAllRead]);

    // Callback ref: attach keyboard and selection listeners when the DOM element mounts.
    // Using a callback ref avoids the silent failure risk of useRef + useEffect([]),
    // where bodyRef.current could be null if the effect runs before commit.
    const cleanupRef = useRef<(() => void) | null>(null);

    const bodyRefCallback = useCallback((el: HTMLDivElement | null) => {
        // Cleanup previous listeners if element is being unmounted or swapped.
        if (cleanupRef.current) {
            cleanupRef.current();
            cleanupRef.current = null;
        }

        registerBodyElement(el);

        if (!el) return;

        // --- Ctrl+C: copy selected text in error log body ---
        const handleKeyDown = (e: KeyboardEvent) => {
            if (e.ctrlKey && (e.key === "c" || e.key === "C")) {
                const selection = window.getSelection();
                if (selection && selection.toString()) {
                    e.preventDefault();
                    void ClipboardSetText(selection.toString()).catch((err: unknown) => {
                        if (import.meta.env.DEV) {
                            console.warn("[error-log] clipboard write failed", err);
                        }
                    });
                }
            }
        };
        el.addEventListener("keydown", handleKeyDown);

        // --- Copy-on-select with 100ms debounce (same as terminal / FileContentViewer) ---
        let copyOnSelectTimer: ReturnType<typeof setTimeout> | null = null;

        const handleSelectionChange = () => {
            if (copyOnSelectTimer !== null) clearTimeout(copyOnSelectTimer);
            copyOnSelectTimer = setTimeout(() => {
                copyOnSelectTimer = null;
                const selection = window.getSelection();
                if (!selection || selection.isCollapsed) return;
                if (!el.contains(selection.anchorNode)) return;
                const text = selection.toString();
                if (text) {
                    void ClipboardSetText(text).catch((err: unknown) => {
                        if (import.meta.env.DEV) {
                            console.warn("[error-log] clipboard write failed", err);
                        }
                    });
                }
            }, 100);
        };
        document.addEventListener("selectionchange", handleSelectionChange);

        // Store cleanup function for unmount or element swap.
        cleanupRef.current = () => {
            el.removeEventListener("keydown", handleKeyDown);
            document.removeEventListener("selectionchange", handleSelectionChange);
            if (copyOnSelectTimer !== null) clearTimeout(copyOnSelectTimer);
        };
    }, [registerBodyElement]);

    // Cleanup copy timer on unmount.
    useEffect(() => {
        return () => {
            if (copyTimerRef.current !== null) {
                clearTimeout(copyTimerRef.current);
            }
        };
    }, []);

    useEffect(() => {
        return () => {
            if (cleanupRef.current) {
                cleanupRef.current();
                cleanupRef.current = null;
            }
        };
    }, []);

    const handleCopyAll = useCallback(() => {
        copyAll();
        if (copyTimerRef.current !== null) {
            clearTimeout(copyTimerRef.current);
            copyTimerRef.current = null;
        }
        setAllCopied(true);
        copyTimerRef.current = setTimeout(() => {
            copyTimerRef.current = null;
            setAllCopied(false);
        }, 1500);
    }, [copyAll]);

    return (
        <div className="error-log-view">
            <div className="viewer-header">
                <h3 className="viewer-header-title">Error Log</h3>
                <span className="viewer-header-spacer"/>
                <button
                    className="viewer-header-btn"
                    onClick={handleCopyAll}
                    title={allCopied ? "Copied!" : "Copy all"}
                    disabled={entries.length === 0}
                >
                    {allCopied ? (
                        <svg width="14" height="14" viewBox="0 0 16 16" fill="none">
                            <path
                                d="M3 8.5L6.5 12L13 4"
                                stroke="currentColor"
                                strokeWidth="2"
                                strokeLinecap="round"
                                strokeLinejoin="round"
                            />
                        </svg>
                    ) : (
                        <svg width="14" height="14" viewBox="0 0 16 16" fill="none">
                            <rect x="5" y="3" width="8" height="10" rx="1" stroke="currentColor" strokeWidth="1.5"/>
                            <path
                                d="M3 5v8a1 1 0 001 1h6"
                                stroke="currentColor"
                                strokeWidth="1.5"
                                strokeLinecap="round"
                                strokeLinejoin="round"
                            />
                        </svg>
                    )}
                </button>
                <button
                    className="viewer-header-btn"
                    onClick={closeView}
                    title="Close (Escape)"
                >
                    <svg width="14" height="14" viewBox="0 0 16 16" fill="none">
                        <path
                            d="M4 4L12 12M12 4L4 12"
                            stroke="currentColor"
                            strokeWidth="1.5"
                            strokeLinecap="round"
                        />
                    </svg>
                </button>
            </div>
            <div className="error-log-body" ref={bodyRefCallback} tabIndex={0}>
                {entries.length === 0 ? (
                    <div className="error-log-empty">No errors logged</div>
                ) : (
                    entries.map((entry) => (
                        <div
                            key={entry.seq}
                            className="error-log-entry"
                            onClick={() => copyEntry(entry)}
                            title="Click to copy"
                        >
                            <span className="error-log-ts">{formatTimestamp(entry.ts)}</span>
                            <span className={`error-log-level ${entry.level}`}>{entry.level}</span>
                            <span className="error-log-msg">{entry.msg}</span>
                            {entry.source && (
                                <span className="error-log-source">[{entry.source}]</span>
                            )}
                        </div>
                    ))
                )}
            </div>
        </div>
    );
}
