import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";

const mocked = vi.hoisted(() => ({
    resetDockRatio: vi.fn(),
    setDockRatio: vi.fn<(ratio: number) => void>(),
}));

vi.mock("../src/components/viewer/viewerStore", () => ({
    useViewerStore: (selector: (state: {
        resetDockRatio: () => void;
        setDockRatio: (ratio: number) => void;
    }) => unknown) => selector({
        resetDockRatio: mocked.resetDockRatio,
        setDockRatio: mocked.setDockRatio,
    }),
}));

import {DockedDivider} from "../src/components/viewer/DockedDivider";

function createRect(left: number, width: number): DOMRect {
    return DOMRect.fromRect({x: left, y: 0, width, height: 32});
}

describe("DockedDivider", () => {
    let container: HTMLDivElement;
    let root: Root;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        mocked.resetDockRatio.mockReset();
        mocked.setDockRatio.mockReset();
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

    it("cleans up drag state on window blur", () => {
        act(() => {
            root.render(
                <div className="app-body">
                    <DockedDivider/>
                </div>,
            );
        });

        const appBody = container.querySelector<HTMLDivElement>(".app-body");
        const divider = container.querySelector<HTMLDivElement>(".docked-divider");
        expect(appBody).not.toBeNull();
        expect(divider).not.toBeNull();

        vi.spyOn(appBody!, "getBoundingClientRect").mockReturnValue(createRect(100, 400));
        document.body.style.userSelect = "text";

        act(() => {
            divider?.dispatchEvent(new MouseEvent("mousedown", {bubbles: true, clientX: 260}));
        });

        expect(appBody?.classList.contains("app-body--dragging")).toBe(true);
        expect(document.body.style.userSelect).toBe("none");

        act(() => {
            window.dispatchEvent(new MouseEvent("mousemove", {clientX: 300}));
        });

        expect(mocked.setDockRatio).toHaveBeenCalledWith(0.5);

        act(() => {
            window.dispatchEvent(new Event("blur"));
        });

        expect(appBody?.classList.contains("app-body--dragging")).toBe(false);
        expect(document.body.style.userSelect).toBe("text");

        act(() => {
            window.dispatchEvent(new MouseEvent("mousemove", {clientX: 340}));
        });

        expect(mocked.setDockRatio).toHaveBeenCalledTimes(1);
    });

    it("resets the dock ratio on double click", () => {
        act(() => {
            root.render(
                <div className="app-body">
                    <DockedDivider/>
                </div>,
            );
        });

        const divider = container.querySelector<HTMLDivElement>(".docked-divider");
        expect(divider).not.toBeNull();

        act(() => {
            divider?.dispatchEvent(new MouseEvent("dblclick", {bubbles: true}));
        });

        expect(mocked.resetDockRatio).toHaveBeenCalledTimes(1);
    });
});
