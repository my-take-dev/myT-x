import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {isImeTransitionalEvent} from "../src/utils/ime";
import {COMMIT_DEDUPE_WINDOW_MS, createTerminalImeInputGate, shouldLetXtermHandleImeEvent} from "../src/utils/terminalIme";

/** 1ms past the dedupe window to advance timers beyond it. */
const TIMER_PAST_DEDUPE_WINDOW = COMMIT_DEDUPE_WINDOW_MS + 1;

function keyboardEvent(init: KeyboardEventInit & { keyCode?: number } = {}): KeyboardEvent {
    const event = new KeyboardEvent("keydown", init);
    if (typeof init.keyCode === "number") {
        Object.defineProperty(event, "keyCode", {value: init.keyCode});
    }
    return event;
}

describe("isImeTransitionalEvent", () => {
    it("returns true for composing events", () => {
        const event = keyboardEvent({isComposing: true});
        expect(isImeTransitionalEvent(event)).toBe(true);
    });

    it("returns true for Process key events", () => {
        const event = keyboardEvent({key: "Process"});
        expect(isImeTransitionalEvent(event)).toBe(true);
    });

    it("returns true for keyCode=229 fallback", () => {
        const event = keyboardEvent({key: "Unidentified", keyCode: 229});
        expect(isImeTransitionalEvent(event)).toBe(true);
    });

    it("returns false for regular keys", () => {
        const event = keyboardEvent({key: "a"});
        expect(isImeTransitionalEvent(event)).toBe(false);
    });
});

describe("shouldLetXtermHandleImeEvent", () => {
    it("returns true while composition is active", () => {
        const event = keyboardEvent({key: "a"});
        expect(shouldLetXtermHandleImeEvent(event, true)).toBe(true);
    });

    it("returns true for transitional IME key events", () => {
        const event = keyboardEvent({key: "Process"});
        expect(shouldLetXtermHandleImeEvent(event, false)).toBe(true);
    });

    it("returns true for keyCode=229 transitional IME events", () => {
        const event = keyboardEvent({key: "Unidentified", keyCode: 229});
        expect(shouldLetXtermHandleImeEvent(event, false)).toBe(true);
    });

    it("returns false for regular key events outside composition", () => {
        const event = keyboardEvent({key: "a"});
        expect(shouldLetXtermHandleImeEvent(event, false)).toBe(false);
    });
});

describe("createTerminalImeInputGate", () => {
    beforeEach(() => {
        vi.useFakeTimers();
    });

    afterEach(() => {
        vi.useRealTimers();
    });

    it("suppresses uncommitted payloads while composition is active", () => {
        const gate = createTerminalImeInputGate();

        gate.markCompositionStart();

        expect(gate.shouldSend("に")).toBe(false);
        expect(gate.shouldSend("にほ")).toBe(false);
    });

    it("suppresses one identical payload immediately after compositionend", () => {
        const gate = createTerminalImeInputGate();

        gate.markCompositionStart();
        gate.markCompositionEnd("");

        expect(gate.shouldSend("日本語")).toBe(true);
        expect(gate.shouldSend("日本語")).toBe(false);
    });

    it("does not suppress different payloads after compositionend", () => {
        const gate = createTerminalImeInputGate();

        gate.markCompositionStart();
        gate.markCompositionEnd("");

        expect(gate.shouldSend("日")).toBe(true);
        expect(gate.shouldSend("本")).toBe(true);
    });

    it("does not suppress repeated payloads outside the dedupe window", () => {
        const gate = createTerminalImeInputGate();

        gate.markCompositionStart();
        gate.markCompositionEnd("");

        expect(gate.shouldSend("かな")).toBe(true);
        vi.advanceTimersByTime(TIMER_PAST_DEDUPE_WINDOW);

        expect(gate.shouldSend("かな")).toBe(true);
    });

    it("keeps the previous committed payload deduped across a new compositionstart", () => {
        const gate = createTerminalImeInputGate();

        gate.markCompositionStart();
        gate.markCompositionEnd("");

        expect(gate.shouldSend("変換")).toBe(true);

        gate.markCompositionStart();
        gate.markCompositionEnd("");

        expect(gate.shouldSend("変換")).toBe(false);
        vi.advanceTimersByTime(TIMER_PAST_DEDUPE_WINDOW);
        expect(gate.shouldSend("変換")).toBe(true);
    });

    it("cancelComposition resets composing state without arming dedupe", () => {
        const gate = createTerminalImeInputGate();

        gate.markCompositionStart();
        gate.cancelComposition();

        expect(gate.shouldSend("a")).toBe(true);
        expect(gate.shouldSend("a")).toBe(true);
    });

    it("allows normal input when no IME composition is active", () => {
        const gate = createTerminalImeInputGate();

        expect(gate.shouldSend("a")).toBe(true);
        expect(gate.shouldSend("b")).toBe(true);
    });

    it("dispose cancels the pending timer and resets state", () => {
        const gate = createTerminalImeInputGate();

        gate.markCompositionStart();
        gate.markCompositionEnd("");

        expect(gate.shouldSend("テスト")).toBe(true);
        gate.dispose();
        vi.runAllTimers();

        gate.markCompositionStart();
        gate.markCompositionEnd("");

        expect(gate.shouldSend("テスト")).toBe(true);
        expect(gate.shouldSend("テスト")).toBe(false);
    });

    it("compositionEnd without compositionStart arms the gate safely", () => {
        const gate = createTerminalImeInputGate();

        gate.markCompositionEnd("");

        expect(gate.shouldSend("a")).toBe(true);
        expect(gate.shouldSend("a")).toBe(false);
        vi.advanceTimersByTime(TIMER_PAST_DEDUPE_WINDOW);

        expect(gate.shouldSend("a")).toBe(true);
    });

    it("ignores empty payloads while waiting for the committed text", () => {
        const gate = createTerminalImeInputGate();

        gate.markCompositionStart();
        gate.markCompositionEnd("");

        expect(gate.shouldSend("")).toBe(false);
        expect(gate.shouldSend("日本語")).toBe(true);
        expect(gate.shouldSend("日本語")).toBe(false);
    });

    it("keeps the committed payload dedupe active across control input", () => {
        const gate = createTerminalImeInputGate();

        gate.markCompositionStart();
        gate.markCompositionEnd("");

        expect(gate.shouldSend("日本語")).toBe(true);
        expect(gate.shouldSend("\r")).toBe(true);
        expect(gate.shouldSend("日本語")).toBe(false);
    });

    it("allows control input before the committed payload without arming the wrong candidate", () => {
        const gate = createTerminalImeInputGate();

        gate.markCompositionStart();
        gate.markCompositionEnd("");

        expect(gate.shouldSend("\r")).toBe(true);
        expect(gate.shouldSend("日本語")).toBe(true);
        expect(gate.shouldSend("日本語")).toBe(false);
    });

    it("handles three consecutive composition cycles correctly", () => {
        const gate = createTerminalImeInputGate();

        for (const word of ["東京", "大阪", "名古屋"]) {
            gate.markCompositionStart();
            expect(gate.shouldSend(word)).toBe(false);
            gate.markCompositionEnd("");
            expect(gate.shouldSend(word)).toBe(true);
            expect(gate.shouldSend(word)).toBe(false);
        }
    });

    // --- 自動確定シナリオ（RC-1/RC-2 修正） ---

    it("accepts committed text via pendingCommit during new composition (auto-confirm)", () => {
        const gate = createTerminalImeInputGate();

        // 1st composition: "二重確定"
        gate.markCompositionStart();
        gate.markCompositionEnd("二重確定");

        // Auto-confirm: new composition starts before onData arrives
        gate.markCompositionStart();

        // xterm.js setTimeout fires: committed text from previous composition
        expect(gate.shouldSend("二重確定")).toBe(true);
        // Duplicate from WebView2
        expect(gate.shouldSend("二重確定")).toBe(false);
    });

    it("suppresses uncommitted text during new composition even with pendingCommit", () => {
        const gate = createTerminalImeInputGate();

        gate.markCompositionStart();
        gate.markCompositionEnd("確定");
        gate.markCompositionStart();

        // Non-matching payload during composing (new uncommitted text)
        expect(gate.shouldSend("か")).toBe(false);
        // pendingCommit still accepted
        expect(gate.shouldSend("確定")).toBe(true);
    });

    it("handles compositionend→compositionstart→onData→duplicate correctly", () => {
        const gate = createTerminalImeInputGate();

        // Simulate: type "東京", convert, then start typing next word
        gate.markCompositionStart();
        expect(gate.shouldSend("とうきょう")).toBe(false);
        gate.markCompositionEnd("東京");

        // New composition starts before setTimeout fires
        gate.markCompositionStart();

        // setTimeout fires: committed text from "東京"
        expect(gate.shouldSend("東京")).toBe(true);
        // WebView2 duplicate
        expect(gate.shouldSend("東京")).toBe(false);
        // Uncommitted text in new composition
        expect(gate.shouldSend("おお")).toBe(false);

        // Complete new composition
        gate.markCompositionEnd("大阪");
        expect(gate.shouldSend("大阪")).toBe(true);
        expect(gate.shouldSend("大阪")).toBe(false);
    });

    it("falls back to first-payload dedup when compositionend.data is empty", () => {
        const gate = createTerminalImeInputGate();

        gate.markCompositionStart();
        // compositionend.data is empty/undefined (unreliable on some browsers)
        gate.markCompositionEnd("");

        // Auto-confirm: new composition starts
        gate.markCompositionStart();

        // Without pendingCommit, composing=true suppresses everything
        expect(gate.shouldSend("テスト")).toBe(false);
    });

    it("accepts pendingCommit then dedupes across compositionend boundary", () => {
        const gate = createTerminalImeInputGate();

        gate.markCompositionStart();
        gate.markCompositionEnd("入力");
        gate.markCompositionStart();

        // Accept via pendingCommit
        expect(gate.shouldSend("入力")).toBe(true);

        // Finish new composition
        gate.markCompositionEnd("テスト");

        // "入力" is now committedInput, should still be deduped
        // (even though we transitioned to new composition)
        expect(gate.shouldSend("テスト")).toBe(true);
        expect(gate.shouldSend("テスト")).toBe(false);
    });

    it("suppresses a delayed duplicate from the previous cycle after the next compositionend", () => {
        const gate = createTerminalImeInputGate();

        gate.markCompositionStart();
        gate.markCompositionEnd("東京");
        gate.markCompositionStart();

        // First committed payload from the previous cycle is accepted.
        expect(gate.shouldSend("東京")).toBe(true);

        // The next cycle finishes before the old duplicate arrives.
        gate.markCompositionEnd("大阪");

        // The delayed duplicate from the previous cycle must still be suppressed.
        expect(gate.shouldSend("東京")).toBe(false);

        // The current cycle's committed payload must still pass exactly once.
        expect(gate.shouldSend("大阪")).toBe(true);
        expect(gate.shouldSend("大阪")).toBe(false);
    });

    // --- 重複検出ウィンドウ延長（Fix 6） ---

    it("extends the dedupe window on duplicate detection (3+ duplicates)", () => {
        const gate = createTerminalImeInputGate();

        gate.markCompositionStart();
        gate.markCompositionEnd("");

        expect(gate.shouldSend("重複")).toBe(true);

        // 1st duplicate at 70ms
        vi.advanceTimersByTime(70);
        expect(gate.shouldSend("重複")).toBe(false);

        // 2nd duplicate at 140ms (would fail without window extension)
        vi.advanceTimersByTime(70);
        expect(gate.shouldSend("重複")).toBe(false);

        // Window expires
        vi.advanceTimersByTime(TIMER_PAST_DEDUPE_WINDOW);
        expect(gate.shouldSend("重複")).toBe(true);
    });

    // --- pendingCommit with eventData ---

    it("uses compositionend eventData to pre-register committed text", () => {
        const gate = createTerminalImeInputGate();

        gate.markCompositionStart();
        gate.markCompositionEnd("変換テスト");

        // First payload matches eventData
        expect(gate.shouldSend("変換テスト")).toBe(true);
        // Duplicate suppressed
        expect(gate.shouldSend("変換テスト")).toBe(false);
    });

    it("clears pendingCommit after first match in non-composing mode", () => {
        const gate = createTerminalImeInputGate();

        gate.markCompositionStart();
        gate.markCompositionEnd("ABC");

        expect(gate.shouldSend("ABC")).toBe(true);
        // pendingCommit cleared, committedInput="ABC", duplicate suppressed
        expect(gate.shouldSend("ABC")).toBe(false);

        vi.advanceTimersByTime(TIMER_PAST_DEDUPE_WINDOW);
        // After window expires, normal input resumes
        expect(gate.shouldSend("ABC")).toBe(true);
    });

    // --- dedupe timer does not affect composing state ---

    it("dedupe timer does not set composing to false during active composition", () => {
        const gate = createTerminalImeInputGate();

        gate.markCompositionStart();
        gate.markCompositionEnd("テスト");

        // Accept committed text
        expect(gate.shouldSend("テスト")).toBe(true);

        // Start new composition immediately
        gate.markCompositionStart();

        // Let the dedupe timer from markCompositionEnd expire
        vi.advanceTimersByTime(TIMER_PAST_DEDUPE_WINDOW);

        // Should still be composing - timer must NOT reset composing flag
        expect(gate.shouldSend("あ")).toBe(false);

        // Finish composition normally
        gate.markCompositionEnd("新しい");
        expect(gate.shouldSend("新しい")).toBe(true);
    });

    // --- 連続高速確定 ---

    it("handles rapid auto-confirm cycles without losing text", () => {
        const gate = createTerminalImeInputGate();

        // Cycle 1: "あ" → "亜"
        gate.markCompositionStart();
        gate.markCompositionEnd("亜");
        gate.markCompositionStart(); // auto-confirm
        expect(gate.shouldSend("亜")).toBe(true);
        expect(gate.shouldSend("亜")).toBe(false);

        // Cycle 2: "い" → "以"
        gate.markCompositionEnd("以");
        gate.markCompositionStart(); // auto-confirm
        expect(gate.shouldSend("以")).toBe(true);
        expect(gate.shouldSend("以")).toBe(false);

        // Cycle 3: "う" → "宇" (final, no auto-confirm)
        gate.markCompositionEnd("宇");
        expect(gate.shouldSend("宇")).toBe(true);
        expect(gate.shouldSend("宇")).toBe(false);
    });

    // --- C-3: isControlSequence / isEmptyImePayload edge cases ---

    it("forwards DEL character (0x7F) as control sequence during dedupe window", () => {
        const gate = createTerminalImeInputGate();
        gate.markCompositionStart();
        gate.markCompositionEnd("");
        // DEL (0x7F) is a control character — should be forwarded, not become committedInput
        expect(gate.shouldSend("\x7F")).toBe(true);
        // Committed printable text after DEL should still be accepted
        expect(gate.shouldSend("漢字")).toBe(true);
        expect(gate.shouldSend("漢字")).toBe(false);
    });

    it("treats surrogate pair emoji as printable (not control)", () => {
        const gate = createTerminalImeInputGate();
        gate.markCompositionStart();
        gate.markCompositionEnd("\uD83D\uDE00");
        // Emoji is printable — accepted as committed text
        expect(gate.shouldSend("\uD83D\uDE00")).toBe(true);
        // Duplicate suppressed
        expect(gate.shouldSend("\uD83D\uDE00")).toBe(false);
    });

    it("treats ANSI escape sequence as printable (mixed control + non-control)", () => {
        const gate = createTerminalImeInputGate();
        gate.markCompositionStart();
        gate.markCompositionEnd("");
        // "\x1B[A" starts with ESC (control) but contains non-control chars
        // isControlSequence returns false, so it is treated as printable
        expect(gate.shouldSend("\x1B[A")).toBe(true);
        expect(gate.shouldSend("\x1B[A")).toBe(false);
    });

    it("forwards single ESC (0x1B) as control sequence during dedupe window", () => {
        const gate = createTerminalImeInputGate();
        gate.markCompositionStart();
        gate.markCompositionEnd("");
        // Single ESC is purely control — forwarded without becoming committedInput
        expect(gate.shouldSend("\x1B")).toBe(true);
        expect(gate.shouldSend("日本")).toBe(true);
        expect(gate.shouldSend("日本")).toBe(false);
    });

    // --- C-4: markCompositionEnd("") maps empty string to null (pendingCommit) ---

    it("markCompositionEnd with empty string treats eventData as null (no pendingCommit)", () => {
        const gate = createTerminalImeInputGate();
        gate.markCompositionStart();
        // Empty string: eventData || null → null, so pendingCommit is not armed
        gate.markCompositionEnd("");

        // Auto-confirm scenario: new composition starts
        gate.markCompositionStart();
        // Without pendingCommit, composing=true suppresses all payloads
        expect(gate.shouldSend("テスト")).toBe(false);
    });

    // --- IM-5: non-composing CommitPending with eventData mismatch ---

    it("handles non-composing eventData mismatch: different payload resets dedupe", () => {
        const gate = createTerminalImeInputGate();
        gate.markCompositionStart();
        gate.markCompositionEnd("予測");
        // composing=false, commitWindowActive=true, pendingCommit="予測"
        expect(gate.shouldSend("実際")).toBe(true);   // committedInput="実際" (first printable accepted)
        // "予測" != committedInput "実際" → reset() called, dedupe cleared, returns true
        expect(gate.shouldSend("予測")).toBe(true);
        // After reset, no dedupe — repeated "予測" also passes (idle state)
        expect(gate.shouldSend("予測")).toBe(true);
    });

    // --- I-5: Timer boundary value tests ---

    it("dedupe is still active at exactly COMMIT_DEDUPE_WINDOW_MS - 1", () => {
        const gate = createTerminalImeInputGate();
        gate.markCompositionStart();
        gate.markCompositionEnd("");
        expect(gate.shouldSend("境界")).toBe(true);

        vi.advanceTimersByTime(COMMIT_DEDUPE_WINDOW_MS - 1); // 79ms
        // Window still active → duplicate suppressed
        expect(gate.shouldSend("境界")).toBe(false);
    });

    it("dedupe window expires at exactly COMMIT_DEDUPE_WINDOW_MS", () => {
        const gate = createTerminalImeInputGate();
        gate.markCompositionStart();
        gate.markCompositionEnd("");
        expect(gate.shouldSend("境界")).toBe(true);

        vi.advanceTimersByTime(COMMIT_DEDUPE_WINDOW_MS); // 80ms
        // Window expired → same text accepted as new input
        expect(gate.shouldSend("境界")).toBe(true);
    });

    // --- S-3: dispose() then shouldSend returns to idle state ---

    it("shouldSend returns true immediately after dispose (idle state)", () => {
        const gate = createTerminalImeInputGate();
        gate.markCompositionStart();
        gate.dispose();
        // After dispose, gate should be in idle state — all payloads pass through
        expect(gate.shouldSend("a")).toBe(true);
        expect(gate.shouldSend("b")).toBe(true);
    });

    it("shouldSend returns true after dispose during dedupe window", () => {
        const gate = createTerminalImeInputGate();
        gate.markCompositionStart();
        gate.markCompositionEnd("テスト");
        expect(gate.shouldSend("テスト")).toBe(true);
        gate.dispose();
        vi.runAllTimers();
        // Idle state — no dedup, no composing
        expect(gate.shouldSend("テスト")).toBe(true);
        expect(gate.shouldSend("テスト")).toBe(true);
    });

    // --- S-1: isControlSequence boundary conditions ---

    it("treats mixed control+printable (\\x01A) as printable", () => {
        const gate = createTerminalImeInputGate();
        gate.markCompositionStart();
        gate.markCompositionEnd("");
        // "\x01A" contains SOH (control) + 'A' (printable) → not pure control
        // Should be treated as printable and become committedInput
        expect(gate.shouldSend("\x01A")).toBe(true);
        expect(gate.shouldSend("\x01A")).toBe(false);
    });

    it("treats CRLF pair as control sequence", () => {
        const gate = createTerminalImeInputGate();
        gate.markCompositionStart();
        gate.markCompositionEnd("");
        // "\r\n" is purely control characters — forwarded without becoming committedInput
        expect(gate.shouldSend("\r\n")).toBe(true);
        // Committed printable text after CRLF should still be accepted
        expect(gate.shouldSend("漢字")).toBe(true);
        expect(gate.shouldSend("漢字")).toBe(false);
    });

    // --- S-2: dispose during composing with pendingCommit ---

    it("dispose during composing with pendingCommit clears all state", () => {
        const gate = createTerminalImeInputGate();
        gate.markCompositionStart();
        gate.markCompositionEnd("テスト");
        gate.markCompositionStart();
        gate.dispose();
        // After dispose, gate is in idle — all payloads pass, no dedupe
        expect(gate.shouldSend("テスト")).toBe(true);
        expect(gate.shouldSend("テスト")).toBe(true);
    });

    // --- S-4: eventData and actual onData mismatch ---

    it("handles eventData mismatch (eventData != actual onData)", () => {
        const gate = createTerminalImeInputGate();
        gate.markCompositionStart();
        gate.markCompositionEnd("予測");
        gate.markCompositionStart();
        // Non-matching payload during composing: not pendingCommit match, suppressed
        expect(gate.shouldSend("実際")).toBe(false);
        // pendingCommit "予測" still matches
        expect(gate.shouldSend("予測")).toBe(true);
        // Finish composition with the actual text
        gate.markCompositionEnd("実際");
        expect(gate.shouldSend("実際")).toBe(true);
        expect(gate.shouldSend("実際")).toBe(false);
    });

    // --- markCompositionStart clears commitWindowActive (I-2 fix) ---

    it("markCompositionStart clears commitWindowActive to prevent shadowed state", () => {
        const gate = createTerminalImeInputGate();
        gate.markCompositionStart();
        gate.markCompositionEnd("テスト");
        // commitWindowActive=true now
        // Start new composition — should clear commitWindowActive
        gate.markCompositionStart();
        // Accept pendingCommit (RC-2)
        expect(gate.shouldSend("テスト")).toBe(true);
        // Finish composition
        gate.markCompositionEnd("新規");
        // After markCompositionEnd, commitWindowActive=true again for "新規"
        expect(gate.shouldSend("新規")).toBe(true);
        expect(gate.shouldSend("新規")).toBe(false);
    });

    // --- IM-6: control sequence extends dedupe window (time-based verification) ---

    it("control sequence extends dedupe window", () => {
        const gate = createTerminalImeInputGate();
        gate.markCompositionStart();
        gate.markCompositionEnd("");
        expect(gate.shouldSend("日本語")).toBe(true);
        vi.advanceTimersByTime(70);
        expect(gate.shouldSend("\r")).toBe(true);      // control: extends window
        vi.advanceTimersByTime(70);                     // 140ms from start, but only 70ms from extension
        expect(gate.shouldSend("日本語")).toBe(false);  // still in extended window
        vi.advanceTimersByTime(TIMER_PAST_DEDUPE_WINDOW);
        expect(gate.shouldSend("日本語")).toBe(true);   // window expired
    });

    // --- IM-7: cancelComposition during CommitPending state ---

    it("cancelComposition during CommitPending clears timer and resets to idle", () => {
        const gate = createTerminalImeInputGate();
        gate.markCompositionStart();
        gate.markCompositionEnd("テスト"); // CommitPending state
        gate.cancelComposition();           // reset() → Idle
        // Idle state — no dedupe, no composing
        expect(gate.shouldSend("テスト")).toBe(true);
        expect(gate.shouldSend("テスト")).toBe(true); // no dedup
    });

    // --- SG-6: markCompositionEnd consecutive calls (idempotency) ---

    it("consecutive markCompositionEnd overwrites pendingCommit and reschedules timer", () => {
        const gate = createTerminalImeInputGate();
        gate.markCompositionStart();
        gate.markCompositionEnd("A");
        gate.markCompositionEnd("B"); // overwrites pendingCommit to "B"
        // "A" is no longer the pending commit
        expect(gate.shouldSend("B")).toBe(true);
        expect(gate.shouldSend("B")).toBe(false);
        // "A" treated as different payload → reset
        expect(gate.shouldSend("A")).toBe(true);
    });

    // --- SG-7: dispose() idempotency ---

    it("dispose() can be called multiple times without throwing", () => {
        const gate = createTerminalImeInputGate();
        gate.markCompositionStart();
        gate.markCompositionEnd("テスト");
        gate.dispose();
        gate.dispose(); // should not throw
        // Still in idle state
        expect(gate.shouldSend("a")).toBe(true);
    });

    // --- S-7: same character consecutive auto-confirm cycles ---

    it("accepts same character in consecutive auto-confirm cycles", () => {
        const gate = createTerminalImeInputGate();
        // Cycle 1: "亜"
        gate.markCompositionStart();
        gate.markCompositionEnd("亜");
        gate.markCompositionStart();
        expect(gate.shouldSend("亜")).toBe(true);  // pendingCommit match
        // Cycle 2: "亜" again (same character)
        gate.markCompositionEnd("亜");
        gate.markCompositionStart();
        expect(gate.shouldSend("亜")).toBe(true);  // pendingCommit match, not deduped
    });

    it("rewrites a later committed extension to only the newly confirmed suffix", () => {
        const gate = createTerminalImeInputGate();

        gate.markCompositionStart();
        gate.markCompositionEnd("東京");
        expect(gate.filterInput("東京")).toBe("東京");
        expect(gate.filterInput("東京")).toBeNull();

        gate.markCompositionStart();
        gate.markCompositionEnd("東京駅");
        expect(gate.filterInput("東京駅")).toBe("駅");
        expect(gate.filterInput("東京駅")).toBeNull();
    });

    it("rewrites chained committed extensions against the most recent accepted payload", () => {
        const gate = createTerminalImeInputGate();

        gate.markCompositionStart();
        gate.markCompositionEnd("変換");
        expect(gate.filterInput("変換")).toBe("変換");

        gate.markCompositionStart();
        gate.markCompositionEnd("変換し");
        expect(gate.filterInput("変換し")).toBe("し");

        gate.markCompositionStart();
        gate.markCompositionEnd("変換した");
        expect(gate.filterInput("変換した")).toBe("た");
    });

    it("forwards only the pending committed prefix when a cumulative payload arrives during the next composition", () => {
        const gate = createTerminalImeInputGate();

        gate.markCompositionStart();
        gate.markCompositionEnd("東京");
        gate.markCompositionStart();

        expect(gate.filterInput("東京さ")).toBe("東京");
        expect(gate.filterInput("東京さ")).toBeNull();
    });

    it("rewrites a cumulative payload that arrives after the committed base was already accepted", () => {
        const gate = createTerminalImeInputGate();

        gate.markCompositionStart();
        gate.markCompositionEnd("東京");
        expect(gate.filterInput("東京")).toBe("東京");

        expect(gate.filterInput("東京さ")).toBe("さ");
        expect(gate.filterInput("東京さ")).toBeNull();
    });
});
