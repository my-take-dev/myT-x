import {act, useRef} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {useInputHistoryStore} from "../src/stores/inputHistoryStore";
import type {InputHistoryEntry} from "../src/stores/inputHistoryStore";
import {useInputHistory} from "../src/components/viewer/views/input-history/useInputHistory";

function makeEntry(seq: number): InputHistoryEntry {
    return {
        seq,
        ts: "20260301120000",
        pane_id: "%0",
        input: `cmd-${seq}`,
        source: "keyboard",
        session: "test-session",
    };
}

/**
 * Test harness that renders useInputHistory and exposes the registerBodyElement
 * callback so we can attach a mock scrollable element.
 */
function AutoScrollProbe({bodyEl}: { bodyEl: HTMLDivElement | null }) {
    const {registerBodyElement, entries} = useInputHistory();
    const registeredRef = useRef(false);
    if (!registeredRef.current && bodyEl) {
        registerBodyElement(bodyEl);
        registeredRef.current = true;
    }
    return <output data-testid="count">{entries.length}</output>;
}

describe("useInputHistory auto-scroll", () => {
    let container: HTMLDivElement;
    let root: Root;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

        // Reset store to initial state
        useInputHistoryStore.setState({
            entries: [],
            unreadCount: 0,
            lastReadSeq: 0,
        });
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        container.remove();
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = false;
    });

    it("scrolls to bottom when latestEntrySeq changes and user is near bottom", () => {
        const mockBody = document.createElement("div");
        // Simulate: user is near the bottom (scrollHeight - scrollTop - clientHeight < 60)
        Object.defineProperties(mockBody, {
            scrollHeight: {get: () => 500, configurable: true},
            scrollTop: {get: () => 420, set: vi.fn(), configurable: true},
            clientHeight: {get: () => 50, configurable: true},
        });

        act(() => {
            root.render(<AutoScrollProbe bodyEl={mockBody}/>);
        });

        // Set initial entries (seq=1)
        act(() => {
            useInputHistoryStore.getState().setEntries([makeEntry(1)]);
        });

        // scrollTop should be assigned scrollHeight (500)
        const scrollTopSetter = Object.getOwnPropertyDescriptor(mockBody, "scrollTop")!.set!;
        expect(scrollTopSetter).toHaveBeenCalledWith(500);
    });

    it("does not scroll when latestEntrySeq is unchanged", () => {
        const mockBody = document.createElement("div");
        let scrollTopValue = 420;
        const scrollTopSetter = vi.fn((v: number) => {
            scrollTopValue = v;
        });
        Object.defineProperties(mockBody, {
            scrollHeight: {get: () => 500, configurable: true},
            scrollTop: {get: () => scrollTopValue, set: scrollTopSetter, configurable: true},
            clientHeight: {get: () => 50, configurable: true},
        });

        act(() => {
            root.render(<AutoScrollProbe bodyEl={mockBody}/>);
        });

        // Set entries with seq=1
        act(() => {
            useInputHistoryStore.getState().setEntries([makeEntry(1)]);
        });
        const callCountAfterFirst = scrollTopSetter.mock.calls.length;

        // Set entries again with same seq=1 (no new entry)
        act(() => {
            useInputHistoryStore.getState().setEntries([makeEntry(1)]);
        });

        // No additional scroll calls since latestEntrySeq didn't change
        expect(scrollTopSetter.mock.calls.length).toBe(callCountAfterFirst);
    });

    it("does nothing when bodyRef is null", () => {
        // Render without providing a body element
        act(() => {
            root.render(<AutoScrollProbe bodyEl={null}/>);
        });

        // This should not throw even when entries arrive
        act(() => {
            useInputHistoryStore.getState().setEntries([makeEntry(1)]);
        });

        expect(container.querySelector('[data-testid="count"]')?.textContent).toBe("1");
    });

    it("does not scroll when user is scrolled far from bottom", () => {
        const mockBody = document.createElement("div");
        const scrollTopSetter = vi.fn();
        Object.defineProperties(mockBody, {
            scrollHeight: {get: () => 1000, configurable: true},
            scrollTop: {get: () => 200, set: scrollTopSetter, configurable: true},
            clientHeight: {get: () => 50, configurable: true},
        });
        // Distance from bottom: 1000 - 200 - 50 = 750 (>> 60 threshold)

        act(() => {
            root.render(<AutoScrollProbe bodyEl={mockBody}/>);
        });

        act(() => {
            useInputHistoryStore.getState().setEntries([makeEntry(1)]);
        });

        // scrollTop should NOT be reassigned because user is far from bottom
        expect(scrollTopSetter).not.toHaveBeenCalled();
    });
});
