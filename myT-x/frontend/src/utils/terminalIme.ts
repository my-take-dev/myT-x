import {isImeTransitionalEvent} from "./ime";

/**
 * Returns true if the event is IME-related and should be passed through to
 * xterm.js without interception. The caller (attachCustomKeyEventHandler callback)
 * should return this value directly when true, skipping all custom key handling.
 */
export function shouldLetXtermHandleImeEvent(event: KeyboardEvent, isComposing: boolean): boolean {
    return isComposing || isImeTransitionalEvent(event);
}

/**
 * IME Input Gate — prevents uncommitted IME text from reaching the backend
 * while ensuring committed text is forwarded exactly once.
 *
 * ## Race conditions handled
 *
 * - **RC-1** (compositionend → compositionstart rapid cycle):
 *   Auto-confirm scenarios where the user confirms one character and immediately
 *   starts typing the next. compositionstart fires before the deferred onData
 *   from the previous compositionend arrives.
 *
 * - **RC-2** (deferred onData arriving after new compositionstart):
 *   xterm.js CompositionHelper defers committed text via `setTimeout(fn, 0)`
 *   at compositionend. The onData callback fires while composing=true for
 *   the new composition. The pendingCommit mechanism identifies and accepts
 *   this deferred payload.
 *
 * ## Valid state transitions
 *
 * ```
 * (F=false, T=true, S=non-empty string, null/S=null or string)
 * Idle:           composing=F, commitWindowActive=F, pending=null/S, recentAccepted=0..N
 * Composing:      composing=T, commitWindowActive=F, pending=null/S, recentAccepted=0..N
 * CommitPending:  composing=F, commitWindowActive=T, pending=null/S, recentAccepted=0..N
 * ```
 *
 * Accepted committed payloads are tracked independently in a short-lived
 * `recentAccepted` set. This decouples duplicate suppression from the current
 * composition cycle, so a later `compositionend` cannot erase the dedupe state
 * of a previously accepted committed payload.
 *
 * ## xterm.js version dependency
 *
 * This gate is designed around @xterm/xterm v6.0's CompositionHelper which defers
 * the committed text via `setTimeout(fn, 0)` at compositionend. If a future
 * version changes this timing, the recentAccepted dedupe window and pendingCommit mechanism
 * may need adjustment. When upgrading, check CompositionHelper._finalizeComposition.
 */
export interface TerminalImeInputGate {
    /**
     * Returns the payload that should be forwarded to the backend, or null when
     * the current raw onData payload must be suppressed.
     */
    filterInput(input: string): string | null;
    /** Marks the start of an IME composition session and suppresses interim text. */
    markCompositionStart(): void;
    /**
     * Arms the post-composition dedupe window for the upcoming committed payload.
     * @param eventData - compositionend event.data (expected committed text).
     *   Used to pre-register the expected committed text so that it can be
     *   accepted even if a new compositionstart fires before the deferred
     *   onData payload arrives.
     *   **Required for RC-1/RC-2 to function.** Pass `""` if compositionend.data
     *   is unavailable — the gate falls back to first-payload dedup.
     */
    markCompositionEnd(eventData: string): void;
    /** Cancels an interrupted composition session without arming dedupe. */
    cancelComposition(): void;
    /**
     * Returns true when the current onData payload should be forwarded.
     * Mutates internal state to track the narrow post-composition dedupe window.
     */
    shouldSend(input: string): boolean;
    /** Clears timers and resets all internal state. */
    dispose(): void;
}

/**
 * Post-composition dedupe window duration in milliseconds.
 * Exported for test synchronization.
 *
 * 150ms covers:
 * - setTimeout(0) delay (~4ms typical, up to ~50ms under WebView2 GC/microtask load)
 * - WebView2 IPC overhead
 * - Timer coalescing on Windows (setTimeout can fire earlier than requested)
 * - Safety margin
 *
 * 150ms is well below the minimum human IME conversion interval (~200ms+),
 * so legitimate repeated input of the same character is not affected.
 *
 * This value is shared by both the recentAccepted dedup timer and the
 * schedulePendingWindowReset timeout. Both represent the same "how long
 * to wait after compositionend" concept, so a single constant is used.
 */
export const COMMIT_DEDUPE_WINDOW_MS = 150;

function isEmptyImePayload(input: string): boolean {
    return input.length === 0;
}

// Note: charCodeAt returns surrogate halves for astral characters,
// but they fall outside the control range so this is safe.
function isControlSequence(input: string): boolean {
    if (input.length === 0) {
        return false;
    }
    for (let index = 0; index < input.length; index++) {
        const code = input.charCodeAt(index);
        const isControl = code <= 0x1F || code === 0x7F;
        if (!isControl) {
            return false;
        }
    }
    return true;
}

/**
 * Prevents uncommitted IME text from reaching the backend while preserving the
 * xterm.js CompositionHelper flow that emits the final committed payload.
 *
 * Windows/WebView2 can also emit the committed payload more than once around
 * compositionend. Keep a short post-composition dedupe window so identical
 * committed text is suppressed even when empty/control payloads arrive between
 * the first accepted payload and the duplicate.
 *
 * The dedupe window is intentionally extended on each duplicate detection or
 * control/empty payload, so rapid bursts of duplicates are fully suppressed.
 *
 * See TerminalImeInputGate interface JSDoc for race conditions, state
 * transitions, and xterm.js version dependency.
 */
export function createTerminalImeInputGate(): TerminalImeInputGate {
    /** Entry in the recentAccepted dedup map (implementation detail). */
    interface RecentEntry {
        timer: ReturnType<typeof window.setTimeout>;
        /** performance.now() when the payload was accepted. Used as fallback when
         *  WebView2 timer coalescing causes the setTimeout to fire earlier than requested. */
        acceptedAt: number;
    }
    let composing = false;
    let commitWindowActive = false;
    // Pre-registered committed text from compositionend event.data.
    // Used to accept the deferred onData payload that arrives during composing.
    let pendingCommit: string | null = null;
    let pendingWindowTimer: ReturnType<typeof window.setTimeout> | null = null;
    const recentAccepted = new Map<string, RecentEntry>();
    let lastAcceptedInput: string | null = null;

    const clearPendingWindowTimer = (): void => {
        if (pendingWindowTimer !== null) {
            window.clearTimeout(pendingWindowTimer);
            pendingWindowTimer = null;
        }
    };

    const clearRecentAccepted = (): void => {
        for (const entry of recentAccepted.values()) {
            window.clearTimeout(entry.timer);
        }
        recentAccepted.clear();
        lastAcceptedInput = null;
    };

    /**
     * Registers `input` in the dedup map with a self-cleaning timer.
     * @param input - The accepted payload text.
     * @param preserveAcceptedAt - When rescheduling due to early timer fire,
     *   pass the original acceptedAt so elapsed time always converges and the
     *   timer is guaranteed to terminate. Omit for normal calls (new timestamp).
     */
    const rememberRecentAccepted = (input: string, preserveAcceptedAt?: number): void => {
        const existing = recentAccepted.get(input);
        if (existing !== undefined) {
            window.clearTimeout(existing.timer);
        }
        const acceptedAt = preserveAcceptedAt ?? performance.now();
        const nextTimer = window.setTimeout(() => {
            const entry = recentAccepted.get(input);
            if (entry?.timer !== nextTimer) return;
            // Guard against WebView2 timer coalescing: if the timer fires
            // earlier than requested, reschedule with the ORIGINAL acceptedAt
            // so elapsed time always converges and the timer terminates.
            const elapsed = performance.now() - entry.acceptedAt;
            if (elapsed < COMMIT_DEDUPE_WINDOW_MS) {
                rememberRecentAccepted(input, entry.acceptedAt);
                return;
            }
            recentAccepted.delete(input);
            if (lastAcceptedInput === input) {
                lastAcceptedInput = null;
            }
        }, COMMIT_DEDUPE_WINDOW_MS);
        recentAccepted.set(input, {timer: nextTimer, acceptedAt});
        lastAcceptedInput = input;
    };

    const isRecentAccepted = (input: string): boolean => {
        const entry = recentAccepted.get(input);
        if (entry === undefined) return false;
        // Defense-in-depth: verify the entry is still within the dedupe window
        // via wall-clock timestamp, guarding against WebView2 timer misbehavior.
        if (performance.now() - entry.acceptedAt >= COMMIT_DEDUPE_WINDOW_MS) {
            // Proactive cleanup: remove expired entry immediately rather than
            // waiting for the timer callback, keeping map and logic in sync.
            window.clearTimeout(entry.timer);
            recentAccepted.delete(input);
            if (lastAcceptedInput === input) lastAcceptedInput = null;
            return false;
        }
        return true;
    };

    const refreshRecentAccepted = (): void => {
        for (const input of Array.from(recentAccepted.keys())) {
            // Only refresh entries still within the dedupe window.
            // isRecentAccepted proactively cleans up expired entries,
            // so this avoids reviving entries that have already expired.
            if (isRecentAccepted(input)) {
                rememberRecentAccepted(input);
            }
        }
    };

    const rewriteAcceptedPrefix = (input: string): string | null => {
        if (lastAcceptedInput === null || !isRecentAccepted(lastAcceptedInput)) {
            return null;
        }
        if (input.length <= lastAcceptedInput.length || !input.startsWith(lastAcceptedInput)) {
            return null;
        }
        rememberRecentAccepted(lastAcceptedInput);
        return input.slice(lastAcceptedInput.length);
    };

    const schedulePendingWindowReset = (): void => {
        clearPendingWindowTimer();
        pendingWindowTimer = window.setTimeout(() => {
            if (import.meta.env.DEV && pendingCommit !== null) {
                console.warn("[DEBUG-ime] pendingCommit expired without match", pendingCommit);
            }
            commitWindowActive = false;
            pendingCommit = null;
            pendingWindowTimer = null;
        }, COMMIT_DEDUPE_WINDOW_MS);
    };

    const clearCurrentCompositionState = (): void => {
        composing = false;
        commitWindowActive = false;
        pendingCommit = null;
        clearPendingWindowTimer();
    };

    const closeCommitWindow = (): void => {
        commitWindowActive = false;
        pendingCommit = null;
        clearPendingWindowTimer();
    };

    /** Clears all state including recent dedupe entries. */
    const reset = (): void => {
        clearCurrentCompositionState();
        clearRecentAccepted();
    };

    const filterCommittedPayload = (input: string): string | null => {
        const rewritten = rewriteAcceptedPrefix(input);
        closeCommitWindow();
        rememberRecentAccepted(input);
        return rewritten === null ? input : rewritten;
    };

    const suppressRecentDuplicate = (input: string): boolean => {
        rememberRecentAccepted(input);
        return false;
    };

    // Layer 4: Diagnostic logging for IME gate decisions. DEV-only to avoid
    // production overhead. Captures gate state + decision for post-mortem debugging.
    let lastFilterTimestamp = 0;
    const logFilterDecision = (decision: string, input: string): void => {
        if (!import.meta.env.DEV) return;
        const now = performance.now();
        const elapsed = lastFilterTimestamp > 0 ? Math.round(now - lastFilterTimestamp) : 0;
        lastFilterTimestamp = now;
        const state = composing ? "Composing" : commitWindowActive ? "CommitPending" : "Idle";
        const preview = input.length > 20 ? input.slice(0, 20) + "..." : input;
        console.debug(`[DEBUG-ime-gate] filterInput state=${state} decision=${decision} input="${preview}" elapsed=${elapsed}ms recentN=${recentAccepted.size}`);
    };

    return {
        filterInput(input: string): string | null {
            // State: composing — suppress all onData payloads because they still
            // represent uncommitted IME text.
            if (composing) {
                // RC-2: The deferred onData from the previous compositionend
                // arrives while composing is already true for the new session.
                // Accept it via pendingCommit match.
                if (pendingCommit !== null) {
                    if (input === pendingCommit) {
                        logFilterDecision("accepted(RC-2:pendingCommit)", input);
                        return filterCommittedPayload(input);
                    }
                    if (input.length > pendingCommit.length && input.startsWith(pendingCommit)) {
                        logFilterDecision("accepted(RC-2:pendingPrefix)", input);
                        return filterCommittedPayload(pendingCommit);
                    }
                }
                if (isRecentAccepted(input)) {
                    logFilterDecision("suppressed(composing:recentDup)", input);
                    suppressRecentDuplicate(input);
                    return null;
                }
                // Side effect: rewriteAcceptedPrefix calls rememberRecentAccepted
                // internally, extending the dedupe window for lastAcceptedInput.
                // This is intentional — keeping dedup alive during composing prevents
                // stale duplicates from slipping through between composition cycles.
                if (rewriteAcceptedPrefix(input) !== null) {
                    logFilterDecision("suppressed(composing:prefixRewrite)", input);
                    return null;
                }
                logFilterDecision("suppressed(composing)", input);
                return null;
            }
            if (commitWindowActive) {
                // Ignore empty transitional payloads inside the post-composition window
                // so they do not become the remembered commit candidate.
                if (isEmptyImePayload(input)) {
                    schedulePendingWindowReset();
                    return null;
                }
                // Control sequences such as Enter should still be forwarded, but they
                // must not close the commit window because the committed text can
                // still arrive immediately after them.
                if (isControlSequence(input)) {
                    schedulePendingWindowReset();
                    refreshRecentAccepted();
                    return input;
                }
                // A matching pendingCommit belongs to the current composition cycle
                // and must be accepted even if the same text was committed recently.
                if (pendingCommit !== null && input === pendingCommit) {
                    logFilterDecision("accepted(commitWindow:pendingMatch)", input);
                    return filterCommittedPayload(input);
                }
                // If the payload does not match the current cycle but was already
                // accepted recently, treat it as a stale duplicate and keep waiting
                // for the current cycle's committed text.
                if (isRecentAccepted(input)) {
                    logFilterDecision("suppressed(commitWindow:recentDup)", input);
                    schedulePendingWindowReset();
                    suppressRecentDuplicate(input);
                    return null;
                }
                // Accept the first printable payload after compositionend and keep
                // duplicate suppression separate from future composition cycles.
                logFilterDecision("accepted(commitWindow:firstPrintable)", input);
                return filterCommittedPayload(input);
            }
            // State: idle — no IME transition is active, so pass the payload through.
            if (isRecentAccepted(input)) {
                logFilterDecision("suppressed(idle:recentDup)", input);
                suppressRecentDuplicate(input);
                return null;
            }
            const rewritten = rewriteAcceptedPrefix(input);
            if (rewritten !== null) {
                rememberRecentAccepted(input);
                return rewritten;
            }
            if (isControlSequence(input) && recentAccepted.size > 0) {
                refreshRecentAccepted();
            }
            return input;
        },
        markCompositionStart(): void {
            // RC-1: keep pendingCommit and recentAccepted alive across the next
            // composition so a late committed payload from the previous cycle
            // can still be accepted or deduped correctly.
            clearPendingWindowTimer();
            commitWindowActive = false;
            composing = true;
        },
        markCompositionEnd(eventData: string): void {
            composing = false;
            commitWindowActive = true;
            pendingCommit = eventData.length > 0 ? eventData : null;
            schedulePendingWindowReset();
        },
        cancelComposition(): void {
            clearCurrentCompositionState();
        },
        shouldSend(input: string): boolean {
            return this.filterInput(input) !== null;
        },
        dispose(): void {
            reset();
        },
    };
}
