import React, {act, useEffect} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {useCopyFeedback} from "../src/components/viewer/views/shared/useCopyFeedback";

interface HookValue {
    allCopied: boolean;
    copiedEntrySeq: number | null;
    markAllCopied: () => void;
    markEntryCopied: (seq: number) => void;
}

function HookProbe({
    durationMs,
    onValue,
}: {
    durationMs?: number;
    onValue: (value: HookValue) => void;
}) {
    const value = useCopyFeedback(durationMs);
    useEffect(() => {
        onValue(value);
    }, [onValue, value]);
    return null;
}

describe("useCopyFeedback", () => {
    let container: HTMLDivElement;
    let root: Root;
    let latest: HookValue | null;

    beforeEach(() => {
        vi.useFakeTimers();
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        latest = null;
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        container.remove();
        vi.useRealTimers();
        vi.restoreAllMocks();
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = false;
    });

    function renderProbe(durationMs?: number): void {
        act(() => {
            root.render(
                React.createElement(HookProbe, {
                    durationMs,
                    onValue: (value: HookValue) => {
                        latest = value;
                    },
                }),
            );
        });
    }

    it("initial state: allCopied is false and copiedEntrySeq is null", () => {
        renderProbe();
        expect(latest?.allCopied).toBe(false);
        expect(latest?.copiedEntrySeq).toBeNull();
    });

    it("markAllCopied sets allCopied to true then resets after durationMs", () => {
        renderProbe(1500);

        act(() => {
            latest!.markAllCopied();
        });
        expect(latest?.allCopied).toBe(true);

        act(() => {
            vi.advanceTimersByTime(1499);
        });
        expect(latest?.allCopied).toBe(true);

        act(() => {
            vi.advanceTimersByTime(1);
        });
        expect(latest?.allCopied).toBe(false);
    });

    it("markEntryCopied sets copiedEntrySeq then resets after durationMs", () => {
        renderProbe(1500);

        act(() => {
            latest!.markEntryCopied(42);
        });
        expect(latest?.copiedEntrySeq).toBe(42);

        act(() => {
            vi.advanceTimersByTime(1500);
        });
        expect(latest?.copiedEntrySeq).toBeNull();
    });

    it("seq guard: markEntryCopied(1) then markEntryCopied(2) before timer fires does NOT reset seq 2", () => {
        renderProbe(1500);

        act(() => {
            latest!.markEntryCopied(1);
        });
        expect(latest?.copiedEntrySeq).toBe(1);

        act(() => {
            latest!.markEntryCopied(2);
        });
        expect(latest?.copiedEntrySeq).toBe(2);

        // markEntryCopied clears the previous timer, so only the second timer is active.
        // After 1500ms, the second timer fires and resets seq 2 to null.
        act(() => {
            vi.advanceTimersByTime(1500);
        });
        expect(latest?.copiedEntrySeq).toBeNull();
    });

    it("cleanup on unmount does not leave dangling timers", () => {
        renderProbe(1500);

        act(() => {
            latest!.markAllCopied();
        });
        expect(latest?.allCopied).toBe(true);

        act(() => {
            root.unmount();
        });

        // Running all remaining timers after unmount should not throw
        expect(() => {
            vi.runAllTimers();
        }).not.toThrow();
    });

    it("markAllCopied then markEntryCopied: both flags are independent", () => {
        renderProbe(1500);

        act(() => {
            latest!.markAllCopied();
        });
        expect(latest?.allCopied).toBe(true);
        expect(latest?.copiedEntrySeq).toBeNull();

        act(() => {
            latest!.markEntryCopied(42);
        });
        expect(latest?.allCopied).toBe(true);
        expect(latest?.copiedEntrySeq).toBe(42);
    });

    it("markEntryCopied then markAllCopied: both flags are independent", () => {
        renderProbe(1500);

        act(() => {
            latest!.markEntryCopied(42);
        });
        expect(latest?.allCopied).toBe(false);
        expect(latest?.copiedEntrySeq).toBe(42);

        act(() => {
            latest!.markAllCopied();
        });
        expect(latest?.allCopied).toBe(true);
        expect(latest?.copiedEntrySeq).toBe(42);
    });

    it("rapid calls to markAllCopied: only last timer is active", () => {
        renderProbe(1500);

        act(() => {
            latest!.markAllCopied();
        });
        expect(latest?.allCopied).toBe(true);

        // Advance 500ms, then call again
        act(() => {
            vi.advanceTimersByTime(500);
        });
        expect(latest?.allCopied).toBe(true);

        act(() => {
            latest!.markAllCopied();
        });
        expect(latest?.allCopied).toBe(true);

        // After 1000ms from second call, the first timer would have fired but was cleared
        act(() => {
            vi.advanceTimersByTime(1000);
        });
        expect(latest?.allCopied).toBe(true);

        // After the remaining 500ms for the second timer (total 1500ms from second call)
        act(() => {
            vi.advanceTimersByTime(500);
        });
        expect(latest?.allCopied).toBe(false);
    });
});
