import {useCallback, useEffect, useRef} from "react";
import {writeClipboardText} from "../../../../utils/clipboardUtils";
import {notifyClipboardFailure} from "../../../../utils/notifyUtils";

const DEFAULT_COPY_ON_SELECT_DEBOUNCE_MS = 100;

interface UseSelectableCopyBodyOptions {
    registerBodyElement: (el: HTMLDivElement | null) => void;
    logPrefix: string;
    debounceMs?: number;
}

function logClipboardFailure(logPrefix: string, err: unknown): void {
    notifyClipboardFailure();
    console.warn(`${logPrefix} clipboard write failed`, err);
}

/**
 * Wire Ctrl+C and copy-on-select behavior to a scrollable log-like body element.
 * Returns a callback ref that handles listener setup/cleanup.
 */
export function useSelectableCopyBody({
                                          registerBodyElement,
                                          logPrefix,
                                          debounceMs = DEFAULT_COPY_ON_SELECT_DEBOUNCE_MS,
                                      }: UseSelectableCopyBodyOptions): (el: HTMLDivElement | null) => void {
    const cleanupRef = useRef<(() => void) | null>(null);

    const bodyRefCallback = useCallback((el: HTMLDivElement | null) => {
        if (cleanupRef.current) {
            cleanupRef.current();
            cleanupRef.current = null;
        }

        registerBodyElement(el);
        if (!el) return;

        const handleKeyDown = (event: KeyboardEvent) => {
            if (!(event.ctrlKey && (event.key === "c" || event.key === "C"))) {
                return;
            }
            const selection = window.getSelection();
            const text = selection?.toString() ?? "";
            if (!text) return;

            event.preventDefault();
            void writeClipboardText(text).catch((err: unknown) => {
                logClipboardFailure(logPrefix, err);
            });
        };
        el.addEventListener("keydown", handleKeyDown);

        let copyOnSelectTimer: ReturnType<typeof setTimeout> | null = null;

        const handleSelectionChange = () => {
            if (copyOnSelectTimer !== null) {
                clearTimeout(copyOnSelectTimer);
            }
            copyOnSelectTimer = setTimeout(() => {
                copyOnSelectTimer = null;
                const selection = window.getSelection();
                if (!selection || selection.isCollapsed) return;
                if (!el.contains(selection.anchorNode)) return;
                const text = selection.toString();
                if (!text) return;

                void writeClipboardText(text).catch((err: unknown) => {
                    logClipboardFailure(logPrefix, err);
                });
            }, debounceMs);
        };
        // selectionchange only fires on document per the DOM spec (not on individual elements),
        // so we must listen at the document level and filter by checking the selection anchor.
        document.addEventListener("selectionchange", handleSelectionChange);

        cleanupRef.current = () => {
            el.removeEventListener("keydown", handleKeyDown);
            document.removeEventListener("selectionchange", handleSelectionChange);
            if (copyOnSelectTimer !== null) {
                clearTimeout(copyOnSelectTimer);
            }
        };
    }, [debounceMs, logPrefix, registerBodyElement]);

    useEffect(() => {
        return () => {
            if (cleanupRef.current) {
                cleanupRef.current();
                cleanupRef.current = null;
            }
        };
    }, []);

    return bodyRefCallback;
}
