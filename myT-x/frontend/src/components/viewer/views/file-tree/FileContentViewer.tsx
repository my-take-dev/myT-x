import {useCallback, useEffect, useMemo, useRef, useState} from "react";
import {ClipboardSetText} from "../../../../../wailsjs/runtime/runtime";
import type {FileContentResult} from "./fileTreeTypes";
import {formatFileSize} from "./treeUtils";

interface FileContentViewerProps {
    content: FileContentResult | null;
    isLoading: boolean;
}

export function FileContentViewer({content, isLoading}: FileContentViewerProps) {
    const [pathCopied, setPathCopied] = useState(false);
    const bodyRef = useRef<HTMLDivElement>(null);
    const copyPathTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

    // Reset copied state when file changes.
    useEffect(() => {
        setPathCopied(false);
    }, [content?.path]);

    // Cleanup copy path timer on unmount.
    useEffect(() => {
        return () => {
            if (copyPathTimerRef.current !== null) {
                clearTimeout(copyPathTimerRef.current);
            }
        };
    }, []);

    // Feature 1: Copy file path to clipboard.
    const handleCopyPath = useCallback(() => {
        if (!content) return;
        // Clear any existing timer before setting a new one.
        if (copyPathTimerRef.current !== null) {
            clearTimeout(copyPathTimerRef.current);
            copyPathTimerRef.current = null;
        }
        void ClipboardSetText(content.path)
            .then(() => {
                setPathCopied(true);
                copyPathTimerRef.current = setTimeout(() => {
                    copyPathTimerRef.current = null;
                    setPathCopied(false);
                }, 1500);
            })
            .catch((err: unknown) => {
                if (import.meta.env.DEV) {
                    console.warn("[DEBUG-copy] clipboard write failed", err);
                }
            });
    }, [content]);

    // Feature 2: Ctrl+C copies selected text in file content body.
    useEffect(() => {
        const el = bodyRef.current;
        if (!el) return;

        const handleKeyDown = (e: KeyboardEvent) => {
            if (e.ctrlKey && (e.key === "c" || e.key === "C")) {
                const selection = window.getSelection();
                if (selection && selection.toString()) {
                    e.preventDefault();
                    void ClipboardSetText(selection.toString()).catch((err: unknown) => {
                        if (import.meta.env.DEV) {
                            console.warn("[DEBUG-copy] clipboard write failed", err);
                        }
                    });
                }
            }
        };

        el.addEventListener("keydown", handleKeyDown);
        return () => el.removeEventListener("keydown", handleKeyDown);
    }, []);

    // Feature 3: Copy-on-select with 100ms debounce (same as terminal).
    useEffect(() => {
        const el = bodyRef.current;
        if (!el) return;

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
                            console.warn("[DEBUG-copy] clipboard write failed", err);
                        }
                    });
                }
            }, 100);
        };

        document.addEventListener("selectionchange", handleSelectionChange);
        return () => {
            document.removeEventListener("selectionchange", handleSelectionChange);
            if (copyOnSelectTimer !== null) clearTimeout(copyOnSelectTimer);
        };
    }, []);

    // Memoize lines to avoid re-splitting on every render.
    const lines = useMemo(() => {
        if (!content?.content) return [];
        return content.content.split("\n");
    }, [content?.content]);

    if (isLoading) {
        return <div className="file-content-empty">Loading...</div>;
    }

    if (!content) {
        return <div className="file-content-empty">Select a file to preview</div>;
    }

    if (content.binary) {
        return <div className="file-content-binary">Binary file ({formatFileSize(content.size)})</div>;
    }

    return (
        <div className="file-content-viewer">
            <div className="file-content-header">
                <span className="file-content-path">{content.path}</span>
                <button
                    className="file-content-copy-path"
                    onClick={handleCopyPath}
                    title={pathCopied ? "Copied!" : "Copy path"}
                >
                    {pathCopied ? (
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
                <span className="file-content-size">
          {formatFileSize(content.size)}
                    {content.truncated ? " (truncated)" : ""}
        </span>
            </div>
            <div className="file-content-body" ref={bodyRef} tabIndex={0}>
                {lines.map((line, i) => (
                    <div key={`line-${i}`} className="file-content-line">
                        <span className="file-content-line-number">{i + 1}</span>
                        <span className="file-content-line-text">{line}</span>
                    </div>
                ))}
            </div>
        </div>
    );
}
