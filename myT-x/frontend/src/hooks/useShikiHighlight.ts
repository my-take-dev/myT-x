import {useEffect, useState} from "react";
import type {ThemedToken} from "shiki/core";
import {getHighlightSkipInfo, highlightCode, pathToShikiLang, type HighlightSkipInfo} from "../utils/shikiHighlighter";
import {notifyHighlightFailure} from "../utils/notifyUtils";

interface UseShikiIdleState {
    tokens: null;
    skipInfo: null;
    isHighlightFailed: false;
}

interface UseShikiSkippedState {
    tokens: null;
    skipInfo: HighlightSkipInfo;
    isHighlightFailed: false;
}

interface UseShikiSuccessState {
    tokens: ThemedToken[][];
    skipInfo: null;
    isHighlightFailed: false;
}

interface UseShikiFailedState {
    tokens: null;
    skipInfo: null;
    isHighlightFailed: true;
}

export type UseShikiResult =
    | UseShikiIdleState
    | UseShikiSkippedState
    | UseShikiSuccessState
    | UseShikiFailedState;

const SHIKI_IDLE_STATE: UseShikiIdleState = {
    tokens: null,
    skipInfo: null,
    isHighlightFailed: false,
};

const SHIKI_FAILED_STATE: UseShikiFailedState = {
    tokens: null,
    skipInfo: null,
    isHighlightFailed: true,
};

// Dedup set: populated only in DEV builds to suppress repeated skip-warnings
// for the same lang+filePath pair. Cleared on HMR to re-emit after module reload.
const loggedSkippedHighlights = new Set<string>();
if (import.meta.hot) {
    import.meta.hot.dispose(() => loggedSkippedHighlights.clear());
}

/** @internal Test-only reset. Annotated @__PURE__ so bundlers can tree-shake when unused. */
export const _resetLoggedSkippedHighlights = /* @__PURE__ */ (() =>
    () => loggedSkippedHighlights.clear())();

/**
 * React hook that asynchronously highlights code using Shiki.
 *
 * Returns plain-text-compatible null tokens while loading,
 * then re-renders with colored tokens when ready.
 * Uses a `cancelled` flag (set in the cleanup function) to discard stale
 * results when inputs change or the component unmounts before the async
 * highlight completes.
 *
 * @param code     - Source code to highlight. Pass undefined to skip.
 * @param filePath - File path used to derive the language. Automatically ignored when `lang`
 *                   is provided (effectiveFilePath is set to undefined internally).
 * @param lang     - Explicit Shiki language ID. Takes precedence over `filePath` when both are provided.
 */
export function useShikiHighlight(
    code: string | undefined,
    filePath: string | undefined,
    lang?: string,
): UseShikiResult {
    const [state, setState] = useState<UseShikiResult>(SHIKI_IDLE_STATE);

    const effectiveFilePath = lang ? undefined : filePath;

    useEffect(() => {
        // Standard cancelled-flag pattern: the cleanup function sets `cancelled = true`
        // so that any in-flight async result is discarded when deps change or the
        // component unmounts.
        let cancelled = false;

        if (!code) {
            setState(SHIKI_IDLE_STATE);
            return;
        }

        const resolvedLang = lang ?? (effectiveFilePath ? pathToShikiLang(effectiveFilePath) : null);

        if (!resolvedLang) {
            setState(SHIKI_IDLE_STATE);
            return;
        }

        const guard = getHighlightSkipInfo(code);
        if (guard) {
            setState({
                tokens: null,
                skipInfo: guard,
                isHighlightFailed: false,
            });
            return;
        }

        // Reset tokens when inputs change (show plain text while loading).
        setState(SHIKI_IDLE_STATE);

        void highlightCode(code, resolvedLang)
            .then((result) => {
                if (cancelled) return;
                if (result === null) {
                    // Language not available (cooldown or unknown lang) — revert to idle, not failed.
                    setState(SHIKI_IDLE_STATE);
                    if (import.meta.env.DEV) {
                        const skipKey = `${resolvedLang}:${effectiveFilePath ?? "<unknown>"}`;
                        if (!loggedSkippedHighlights.has(skipKey)) {
                            loggedSkippedHighlights.add(skipKey);
                            console.warn("[DEBUG-shiki] highlight skipped after attempt", {
                                lang: resolvedLang,
                                filePath: effectiveFilePath ?? null,
                            });
                        }
                    }
                    return;
                }
                setState({
                    tokens: result,
                    skipInfo: null,
                    isHighlightFailed: false,
                });
            })
            .catch((err: unknown) => {
                if (cancelled) return;
                setState(SHIKI_FAILED_STATE);
                notifyHighlightFailure();
                if (import.meta.env.DEV) {
                    // DEV: log full context for diagnosis (single log, no duplication).
                    console.error("[DEBUG-shiki] highlight failed", {
                        lang: resolvedLang,
                        filePath: effectiveFilePath ?? null,
                        err,
                    });
                } else {
                    // Production: log the full error object so stack traces, cause
                    // chains, and custom properties are retained for diagnosis.
                    console.warn("[shiki] highlight failed", err);
                }
            });
        return () => {
            cancelled = true;
        };
    }, [code, effectiveFilePath, lang]);

    return state;
}
