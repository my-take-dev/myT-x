import {useEffect, useRef, useState} from "react";
import {writeClipboardText} from "../../../../utils/clipboardUtils";
import {notifyClipboardFailure} from "../../../../utils/notifyUtils";

/**
 * Manages clipboard copy with visual "Copied" feedback and safe unmount handling.
 *
 * @template K - Union type of valid copy-target keys (e.g. CliExampleID | "session").
 * @param logTag - Label written to console.warn on clipboard failure.
 */
export function useClipboardCopyFeedback<K extends string>(logTag: string) {
    const [copiedKey, setCopiedKey] = useState<K | null>(null);
    const isMountedRef = useRef(true);
    const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

    useEffect(() => {
        isMountedRef.current = true;
        return () => {
            isMountedRef.current = false;
            if (timerRef.current != null) {
                clearTimeout(timerRef.current);
                timerRef.current = null;
            }
        };
    }, []);

    function handleCopy(value: string, key: K) {
        void writeClipboardText(value).then(() => {
            if (!isMountedRef.current) {
                return;
            }
            setCopiedKey(key);
            if (timerRef.current != null) {
                clearTimeout(timerRef.current);
            }
            timerRef.current = setTimeout(() => setCopiedKey(null), 2000);
        }).catch((err: unknown) => {
            if (!isMountedRef.current) {
                return;
            }
            notifyClipboardFailure();
            console.warn(`[${logTag}] clipboard write failed`, err);
        });
    }

    return {copiedKey, handleCopy} as const;
}
