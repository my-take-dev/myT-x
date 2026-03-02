import {act, useEffect} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";

const mocked = vi.hoisted(() => ({
    writeClipboardText: vi.fn(),
    notifyClipboardFailure: vi.fn(),
}));

vi.mock("../src/utils/clipboardUtils", () => ({
    writeClipboardText: (...args: [string]) => mocked.writeClipboardText(...args),
}));

vi.mock("../src/utils/notifyUtils", () => ({
    notifyClipboardFailure: () => mocked.notifyClipboardFailure(),
}));

import {useSelectableCopyBody} from "../src/components/viewer/views/shared/useSelectableCopyBody";

interface HookValue {
    bodyRefCallback: (el: HTMLDivElement | null) => void;
}

function HookProbe({
    registerBodyElement,
    logPrefix,
    debounceMs,
    onValue,
}: {
    registerBodyElement: (el: HTMLDivElement | null) => void;
    logPrefix: string;
    debounceMs?: number;
    onValue: (value: HookValue) => void;
}) {
    const bodyRefCallback = useSelectableCopyBody({
        registerBodyElement,
        logPrefix,
        debounceMs,
    });
    useEffect(() => {
        onValue({bodyRefCallback});
    }, [onValue, bodyRefCallback]);
    return null;
}

describe("useSelectableCopyBody", () => {
    let container: HTMLDivElement;
    let root: Root;
    let latest: HookValue | null;
    let registerBodyElement: ReturnType<typeof vi.fn>;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        latest = null;
        registerBodyElement = vi.fn();
        mocked.writeClipboardText.mockReset();
        mocked.notifyClipboardFailure.mockReset();
        mocked.writeClipboardText.mockResolvedValue(undefined);
        vi.spyOn(console, "warn").mockImplementation(() => undefined);
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        container.remove();
        vi.restoreAllMocks();
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = false;
    });

    function renderProbe(debounceMs?: number): void {
        act(() => {
            root.render(
                <HookProbe
                    registerBodyElement={registerBodyElement}
                    logPrefix="[test]"
                    debounceMs={debounceMs}
                    onValue={(value) => {
                        latest = value;
                    }}
                />,
            );
        });
    }

    function mockSelection(text: string, anchorNode: Node | null = document.body, isCollapsed = false): void {
        vi.spyOn(window, "getSelection").mockReturnValue({
            toString: () => text,
            anchorNode,
            isCollapsed,
            rangeCount: text ? 1 : 0,
        } as unknown as Selection);
    }

    it("bodyRefCallback calls registerBodyElement with the provided element", () => {
        renderProbe();

        const el = document.createElement("div");
        act(() => {
            latest!.bodyRefCallback(el);
        });

        expect(registerBodyElement).toHaveBeenCalledWith(el);
    });

    it("bodyRefCallback calls registerBodyElement with null", () => {
        renderProbe();

        act(() => {
            latest!.bodyRefCallback(null);
        });

        expect(registerBodyElement).toHaveBeenCalledWith(null);
    });

    it("Ctrl+C handler writes selected text to clipboard", async () => {
        renderProbe();

        const el = document.createElement("div");
        document.body.appendChild(el);
        // Make element focusable for keyboard events
        el.setAttribute("tabindex", "0");

        act(() => {
            latest!.bodyRefCallback(el);
        });

        mockSelection("selected text");

        const event = new KeyboardEvent("keydown", {
            key: "c",
            ctrlKey: true,
            bubbles: true,
            cancelable: true,
        });
        act(() => {
            el.dispatchEvent(event);
        });

        expect(event.defaultPrevented).toBe(true);
        expect(mocked.writeClipboardText).toHaveBeenCalledWith("selected text");

        el.remove();
    });

    it("Ctrl+C does nothing when selection is empty", () => {
        renderProbe();

        const el = document.createElement("div");
        document.body.appendChild(el);

        act(() => {
            latest!.bodyRefCallback(el);
        });

        mockSelection("");

        const event = new KeyboardEvent("keydown", {
            key: "c",
            ctrlKey: true,
            bubbles: true,
            cancelable: true,
        });
        act(() => {
            el.dispatchEvent(event);
        });

        expect(event.defaultPrevented).toBe(false);
        expect(mocked.writeClipboardText).not.toHaveBeenCalled();

        el.remove();
    });

    it("cleanup on unmount removes event listeners without errors", () => {
        renderProbe();

        const el = document.createElement("div");
        document.body.appendChild(el);

        act(() => {
            latest!.bodyRefCallback(el);
        });

        act(() => {
            root.unmount();
        });

        // Dispatching events after unmount should not throw
        mockSelection("text after unmount");
        expect(() => {
            el.dispatchEvent(new KeyboardEvent("keydown", {
                key: "c",
                ctrlKey: true,
                bubbles: true,
            }));
        }).not.toThrow();

        // Clipboard should NOT have been called since listeners were cleaned up
        expect(mocked.writeClipboardText).not.toHaveBeenCalled();

        el.remove();
    });

    // --- selectionchange (copy-on-select) tests ---

    it("selectionchange with text selected inside el writes to clipboard after debounce", async () => {
        vi.useFakeTimers();
        renderProbe(50);

        const el = document.createElement("div");
        const textNode = document.createTextNode("inside text");
        el.appendChild(textNode);
        document.body.appendChild(el);

        act(() => {
            latest!.bodyRefCallback(el);
        });

        mockSelection("inside text", textNode, false);

        act(() => {
            document.dispatchEvent(new Event("selectionchange"));
        });

        // Before debounce fires, clipboard should not be called yet
        expect(mocked.writeClipboardText).not.toHaveBeenCalled();

        // Advance past the debounce
        await act(async () => {
            vi.advanceTimersByTime(50);
            await Promise.resolve();
        });

        expect(mocked.writeClipboardText).toHaveBeenCalledWith("inside text");

        el.remove();
        vi.useRealTimers();
    });

    it("selectionchange with anchorNode outside el does NOT write to clipboard", async () => {
        vi.useFakeTimers();
        renderProbe(50);

        const el = document.createElement("div");
        document.body.appendChild(el);

        const outsideNode = document.createTextNode("outside");
        document.body.appendChild(outsideNode);

        act(() => {
            latest!.bodyRefCallback(el);
        });

        // anchorNode is outside el
        mockSelection("outside text", outsideNode, false);

        act(() => {
            document.dispatchEvent(new Event("selectionchange"));
        });

        await act(async () => {
            vi.advanceTimersByTime(50);
            await Promise.resolve();
        });

        expect(mocked.writeClipboardText).not.toHaveBeenCalled();

        outsideNode.remove();
        el.remove();
        vi.useRealTimers();
    });

    it("selectionchange with isCollapsed true does NOT write to clipboard", async () => {
        vi.useFakeTimers();
        renderProbe(50);

        const el = document.createElement("div");
        const textNode = document.createTextNode("collapsed");
        el.appendChild(textNode);
        document.body.appendChild(el);

        act(() => {
            latest!.bodyRefCallback(el);
        });

        // isCollapsed = true means no actual selection range
        mockSelection("collapsed", textNode, true);

        act(() => {
            document.dispatchEvent(new Event("selectionchange"));
        });

        await act(async () => {
            vi.advanceTimersByTime(50);
            await Promise.resolve();
        });

        expect(mocked.writeClipboardText).not.toHaveBeenCalled();

        el.remove();
        vi.useRealTimers();
    });

    it("two rapid selectionchange events within debounce window result in only one clipboard write", async () => {
        vi.useFakeTimers();
        renderProbe(100);

        const el = document.createElement("div");
        const textNode = document.createTextNode("debounce text");
        el.appendChild(textNode);
        document.body.appendChild(el);

        act(() => {
            latest!.bodyRefCallback(el);
        });

        // First selectionchange
        mockSelection("first selection", textNode, false);
        act(() => {
            document.dispatchEvent(new Event("selectionchange"));
        });

        // Advance partway (less than debounce)
        act(() => {
            vi.advanceTimersByTime(50);
        });
        expect(mocked.writeClipboardText).not.toHaveBeenCalled();

        // Second selectionchange within debounce window (resets the timer)
        mockSelection("second selection", textNode, false);
        act(() => {
            document.dispatchEvent(new Event("selectionchange"));
        });

        // Advance past the debounce from second event
        await act(async () => {
            vi.advanceTimersByTime(100);
            await Promise.resolve();
        });

        // Only the second selection text should have been written, exactly once
        expect(mocked.writeClipboardText).toHaveBeenCalledTimes(1);
        expect(mocked.writeClipboardText).toHaveBeenCalledWith("second selection");

        el.remove();
        vi.useRealTimers();
    });

    it("selectionchange clipboard write failure calls notifyClipboardFailure", async () => {
        vi.useFakeTimers();
        renderProbe(50);

        const el = document.createElement("div");
        const textNode = document.createTextNode("fail text");
        el.appendChild(textNode);
        document.body.appendChild(el);

        act(() => {
            latest!.bodyRefCallback(el);
        });

        mocked.writeClipboardText.mockRejectedValueOnce(new Error("clipboard denied"));

        mockSelection("fail text", textNode, false);

        act(() => {
            document.dispatchEvent(new Event("selectionchange"));
        });

        await act(async () => {
            vi.advanceTimersByTime(50);
            // Flush the promise rejection chain
            await Promise.resolve();
            await Promise.resolve();
            await Promise.resolve();
        });

        expect(mocked.writeClipboardText).toHaveBeenCalledWith("fail text");
        expect(mocked.notifyClipboardFailure).toHaveBeenCalledTimes(1);

        el.remove();
        vi.useRealTimers();
    });

    it("cleanup on element change: calling bodyRefCallback(null) removes old listeners", () => {
        renderProbe();

        const el = document.createElement("div");
        document.body.appendChild(el);

        act(() => {
            latest!.bodyRefCallback(el);
        });

        // Detach by passing null
        act(() => {
            latest!.bodyRefCallback(null);
        });

        expect(registerBodyElement).toHaveBeenCalledWith(null);

        // Old listeners should be removed
        mockSelection("text after detach");
        el.dispatchEvent(new KeyboardEvent("keydown", {
            key: "c",
            ctrlKey: true,
            bubbles: true,
        }));
        expect(mocked.writeClipboardText).not.toHaveBeenCalled();

        el.remove();
    });
});
