// Reuses error-log-* CSS classes for visual consistency with ErrorLogView.
import {useCallback, useEffect, useLayoutEffect, useRef, useState} from "react";
import {ClipboardSetText} from "../../../../../wailsjs/runtime/runtime";
import {useViewerStore} from "../../viewerStore";
import {useInputHistory} from "./useInputHistory";

export function InputHistoryView() {
    const closeView = useViewerStore((s) => s.closeView);
    const {
        entries,
        markAllRead,
        copyAll,
        copyEntry,
        registerBodyElement,
        formatTimestamp,
        formatInputForDisplay
    } = useInputHistory();
    const [allCopied, setAllCopied] = useState(false);
    const copyTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

    // While this view is open, newly appended entries are immediately marked as read.
    const latestEntrySeq = entries.length > 0 ? entries[entries.length - 1]?.seq : undefined;
    useLayoutEffect(() => {
        markAllRead();
    }, [latestEntrySeq, markAllRead]);

    // Callback ref: attach keyboard and selection listeners when the DOM element mounts.
    const cleanupRef = useRef<(() => void) | null>(null);

    const bodyRefCallback = useCallback((el: HTMLDivElement | null) => {
        if (cleanupRef.current) {
            cleanupRef.current();
            cleanupRef.current = null;
        }

        registerBodyElement(el);

        if (!el) return;

        const handleKeyDown = (e: KeyboardEvent) => {
            if (e.ctrlKey && (e.key === "c" || e.key === "C")) {
                const selection = window.getSelection();
                if (selection && selection.toString()) {
                    e.preventDefault();
                    void ClipboardSetText(selection.toString()).catch((err: unknown) => {
                        if (import.meta.env.DEV) {
                            console.warn("[input-history] clipboard write failed", err);
                        }
                    });
                }
            }
        };
        el.addEventListener("keydown", handleKeyDown);

        // Copy-on-select with 100ms debounce (same as terminal / ErrorLogView).
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
                            console.warn("[input-history] clipboard write failed", err);
                        }
                    });
                }
            }, 100);
        };
        document.addEventListener("selectionchange", handleSelectionChange);

        cleanupRef.current = () => {
            el.removeEventListener("keydown", handleKeyDown);
            document.removeEventListener("selectionchange", handleSelectionChange);
            if (copyOnSelectTimer !== null) clearTimeout(copyOnSelectTimer);
        };
    }, [registerBodyElement]);

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
                <h3 className="viewer-header-title">Input History</h3>
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
                    <div className="error-log-empty">No input history</div>
                ) : (
                    entries.map((entry) => (
                        <div
                            key={entry.seq}
                            className="error-log-entry"
                            onClick={() => copyEntry(entry)}
                            title="Click to copy"
                        >
                            <span className="error-log-ts">{formatTimestamp(entry.ts)}</span>
                            <span className="error-log-level info">{entry.pane_id}</span>
                            <span className="error-log-msg">{formatInputForDisplay(entry.input)}</span>
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
