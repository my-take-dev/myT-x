import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {ChatDivider} from "../src/components/ChatDivider";
import type {AnchorPosition} from "../src/components/useChatResize";

function createRect(left: number, top: number, width: number, height: number): DOMRect {
    return DOMRect.fromRect({x: left, y: top, width, height});
}

describe("ChatDivider", () => {
    let container: HTMLDivElement;
    let root: Root;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        document.body.style.userSelect = "";
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        container.remove();
        document.body.style.userSelect = "";
        vi.restoreAllMocks();
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = false;
    });

    it.each<{
        anchor: AnchorPosition;
        event: MouseEventInit;
        expected: number;
        cssClass: string;
    }>([
        {anchor: "top", event: {clientY: 350}, expected: 0.625, cssClass: "chat-divider--vertical"},
        {anchor: "bottom", event: {clientY: 350}, expected: 0.375, cssClass: "chat-divider--vertical"},
        {anchor: "left", event: {clientX: 260}, expected: 0.4, cssClass: "chat-divider--horizontal"},
        {anchor: "right", event: {clientX: 260}, expected: 0.6, cssClass: "chat-divider--horizontal"},
    ])("computes ratio correctly for anchor=$anchor", ({anchor, event, expected, cssClass}) => {
        const onRatioChange = vi.fn<(ratio: number) => void>();

        act(() => {
            root.render(
                <div className="chat-layout">
                    <ChatDivider
                        anchor={anchor}
                        onRatioChange={onRatioChange}
                        onReset={() => undefined}
                    />
                </div>,
            );
        });

        const layout = container.querySelector<HTMLDivElement>(".chat-layout");
        const divider = container.querySelector<HTMLDivElement>(".chat-divider");
        expect(layout).not.toBeNull();
        expect(divider).not.toBeNull();
        expect(divider?.classList.contains(cssClass)).toBe(true);

        vi.spyOn(layout!, "getBoundingClientRect").mockReturnValue(createRect(100, 100, 400, 400));

        act(() => {
            divider?.dispatchEvent(new MouseEvent("mousedown", {bubbles: true}));
        });

        expect(layout?.classList.contains("chat-layout--dragging")).toBe(true);
        expect(document.body.style.userSelect).toBe("none");

        act(() => {
            window.dispatchEvent(new MouseEvent("mousemove", event));
        });
        expect(onRatioChange).toHaveBeenCalledWith(expected);

        act(() => {
            window.dispatchEvent(new Event("blur"));
        });

        expect(layout?.classList.contains("chat-layout--dragging")).toBe(false);
        expect(document.body.style.userSelect).toBe("");
    });

    it("cleans up drag state on mouseup", () => {
        const onRatioChange = vi.fn<(ratio: number) => void>();

        act(() => {
            root.render(
                <div className="chat-layout">
                    <ChatDivider
                        anchor="bottom"
                        onRatioChange={onRatioChange}
                        onReset={() => undefined}
                    />
                </div>,
            );
        });

        const layout = container.querySelector<HTMLDivElement>(".chat-layout");
        const divider = container.querySelector<HTMLDivElement>(".chat-divider");
        vi.spyOn(layout!, "getBoundingClientRect").mockReturnValue(createRect(100, 100, 400, 400));

        act(() => {
            divider?.dispatchEvent(new MouseEvent("mousedown", {bubbles: true}));
        });
        expect(layout?.classList.contains("chat-layout--dragging")).toBe(true);

        act(() => {
            window.dispatchEvent(new MouseEvent("mouseup"));
        });

        expect(layout?.classList.contains("chat-layout--dragging")).toBe(false);
        expect(document.body.style.userSelect).toBe("");

        // Verify mousemove no longer triggers ratio changes after mouseup
        onRatioChange.mockClear();
        act(() => {
            window.dispatchEvent(new MouseEvent("mousemove", {clientY: 200}));
        });
        expect(onRatioChange).not.toHaveBeenCalled();
    });

    it("resets the ratio on double click", () => {
        const onReset = vi.fn();

        act(() => {
            root.render(
                <div className="chat-layout">
                    <ChatDivider
                        anchor="bottom"
                        onRatioChange={() => undefined}
                        onReset={onReset}
                    />
                </div>,
            );
        });

        const divider = container.querySelector<HTMLDivElement>(".chat-divider");
        expect(divider).not.toBeNull();

        act(() => {
            divider?.dispatchEvent(new MouseEvent("dblclick", {bubbles: true}));
        });

        expect(onReset).toHaveBeenCalledTimes(1);
    });
});
