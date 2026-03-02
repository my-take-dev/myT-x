import {type ReactNode, useCallback, useLayoutEffect} from "react";
import {notifyClipboardFailure} from "../../../../utils/notifyUtils";
import {ViewerPanelShell} from "./ViewerPanelShell";
import {useCopyFeedback} from "./useCopyFeedback";
import {useSelectableCopyBody} from "./useSelectableCopyBody";

/** Minimum contract an entry must satisfy for LogEntryView to track identity. */
interface SeqEntry {
    readonly seq: number;
}

interface LogEntryViewProps<T extends SeqEntry> {
    /** Outer wrapper CSS class (e.g. "error-log-view"). */
    readonly className: string;
    /** Header title text. */
    readonly title: string;
    /** Sorted (ascending seq) array of log entries. */
    readonly entries: readonly T[];
    /**
     * Render a single entry row.
     * @param entry - the entry to render
     * @param isCopied - true while the "Copied!" feedback is active for this entry
     */
    readonly renderEntry: (entry: T, isCopied: boolean) => ReactNode;
    /** Copy all entries to clipboard. Returns true on success. */
    readonly copyAll: () => Promise<boolean>;
    /** Copy a single entry to clipboard. Returns true on success. */
    readonly copyEntry: (entry: T) => Promise<boolean>;
    /** Mark all entries as read (called on mount and when new entries arrive). */
    readonly markAllRead: () => void;
    /** Register the body element for selection-based copy behavior. */
    readonly registerBodyElement: (el: HTMLDivElement | null) => void;
    /** Close the viewer panel. */
    readonly onClose: () => void;
    /** Message shown when entries is empty. */
    readonly emptyMessage: string;
    /** CSS class for the body container (e.g. "error-log-body"). */
    readonly bodyClassName: string;
    /** CSS class for the empty-state container (e.g. "error-log-empty"). */
    readonly emptyClassName: string;
    /** CSS class for each entry row (e.g. "error-log-entry"). */
    readonly entryClassName: string;
    /** Log prefix for clipboard failure messages (e.g. "[error-log]"). */
    readonly logPrefix: string;
}

/**
 * Shared log-entry viewer used by ErrorLogView and InputHistoryView.
 *
 * Owns the common wiring: copy-feedback state, selectable-copy body,
 * auto-read on mount/new-entries, and the header/body/empty-state layout.
 */
export function LogEntryView<T extends SeqEntry>({
                                                     className,
                                                     title,
                                                     entries,
                                                     renderEntry,
                                                     copyAll,
                                                     copyEntry,
                                                     markAllRead,
                                                     registerBodyElement,
                                                     onClose,
                                                     emptyMessage,
                                                     bodyClassName,
                                                     emptyClassName,
                                                     entryClassName,
                                                     logPrefix,
                                                 }: LogEntryViewProps<T>) {
    const {allCopied, copiedEntrySeq, markAllCopied, markEntryCopied} = useCopyFeedback();
    const bodyRefCallback = useSelectableCopyBody({
        registerBodyElement,
        logPrefix,
    });

    // Derive a scalar dependency so useLayoutEffect doesn't re-fire when entry
    // objects change but no new entries are appended.
    // entries is sorted by seq, so the last entry's seq is a monotonic watermark.
    // When entries is empty, latestEntrySeq is undefined. Transitions from a number
    // to undefined (entries cleared) DO trigger React's dependency check, so markAllRead
    // fires correctly. undefined → undefined (empty stays empty) doesn't trigger,
    // but markAllRead on zero entries is a no-op.
    const latestEntrySeq = entries.length > 0 ? entries[entries.length - 1].seq : undefined;

    // While this view is open, newly appended entries are immediately marked as read.
    useLayoutEffect(() => {
        markAllRead();
    }, [latestEntrySeq, markAllRead]);

    const handleCopyAll = useCallback(() => {
        void copyAll()
            .then((copied) => {
                if (copied) markAllCopied();
            })
            .catch((err: unknown) => {
                // Catches unexpected exceptions from the .then() handler.
                // Expected clipboard failures are handled inside copyAll.
                console.error(logPrefix, "unexpected error in copy handler", err);
                notifyClipboardFailure();
            });
    }, [copyAll, logPrefix, markAllCopied]);

    const handleCopyEntry = useCallback((entry: T) => {
        void copyEntry(entry)
            .then((copied) => {
                if (copied) markEntryCopied(entry.seq);
            })
            .catch((err: unknown) => {
                // Catches unexpected exceptions from the .then() handler.
                // Expected clipboard failures are handled inside copyEntry.
                console.error(logPrefix, "unexpected error in copy handler", err);
                notifyClipboardFailure();
            });
    }, [copyEntry, logPrefix, markEntryCopied]);

    return (
        <ViewerPanelShell
            className={className}
            title={title}
            onClose={onClose}
            headerChildren={(
                <button
                    type="button"
                    className="viewer-header-btn"
                    onClick={handleCopyAll}
                    title={allCopied ? "Copied!" : "Copy all"}
                    aria-label={allCopied ? "Copied!" : "Copy all"}
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
            )}
        >
            <div className={bodyClassName} ref={bodyRefCallback} tabIndex={0}>
                {entries.length === 0 ? (
                    <div className={emptyClassName}>{emptyMessage}</div>
                ) : (
                    entries.map((entry) => (
                        <div
                            key={entry.seq}
                            className={entryClassName}
                            role="button"
                            tabIndex={0}
                            onClick={() => handleCopyEntry(entry)}
                            onKeyDown={(event) => {
                                if (event.key !== "Enter" && event.key !== " ") return;
                                event.preventDefault();
                                handleCopyEntry(entry);
                            }}
                            title={copiedEntrySeq === entry.seq ? "Copied!" : "Click to copy"}
                            aria-label={copiedEntrySeq === entry.seq ? "Copied!" : "Copy entry"}
                        >
                            {renderEntry(entry, copiedEntrySeq === entry.seq)}
                        </div>
                    ))
                )}
            </div>
        </ViewerPanelShell>
    );
}
