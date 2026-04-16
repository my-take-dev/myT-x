import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {
    DOCKED_ACTIVITY_STRIP_WIDTH,
    DOCKED_DIVIDER_WIDTH,
    DOCKED_LAYOUT_BASE_WIDTH,
    DOCKED_WINDOW_MIN_WIDTH,
    DOCKED_SIDEBAR_WIDTH,
    DOCK_RATIO_MIN,
    DOCK_RATIO_MAX,
} from "../src/components/viewer/viewerDocking";

const mocked = vi.hoisted(() => ({
    dockRatio: 0.5,
    resetDockRatio: vi.fn(),
    setDockRatio: vi.fn<(ratio: number) => void>(),
}));

vi.mock("../src/components/viewer/viewerStore", () => {
    const state = {
        resetDockRatio: () => mocked.resetDockRatio(),
        setDockRatio: (ratio: number) => mocked.setDockRatio(ratio),
        get dockRatio() {
            return mocked.dockRatio;
        },
    };
    const useViewerStore = (selector: (s: typeof state) => unknown) => selector(state);
    useViewerStore.getState = () => state;
    return {useViewerStore};
});

import {DockedDivider} from "../src/components/viewer/DockedDivider";

function createRect(left: number, width: number): DOMRect {
    return DOMRect.fromRect({x: left, y: 0, width, height: 32});
}

describe("DockedDivider", () => {
    let container: HTMLDivElement;
    let root: Root;

    function renderDivider() {
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

        return {appBody: appBody!, divider: divider!};
    }

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        mocked.dockRatio = 0.5;
        mocked.resetDockRatio.mockReset();
        mocked.setDockRatio.mockReset();
        mocked.resetDockRatio.mockImplementation(() => {
            mocked.dockRatio = 0.5;
        });
        mocked.setDockRatio.mockImplementation((ratio) => {
            mocked.dockRatio = ratio;
        });
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

    it("uses the visible docked content span for wide layouts", () => {
        const {appBody, divider} = renderDivider();

        vi.spyOn(appBody, "getBoundingClientRect").mockReturnValue(createRect(100, 1600));
        document.body.style.userSelect = "text";

        act(() => {
            divider.dispatchEvent(new MouseEvent("mousedown", {bubbles: true, clientX: 260}));
        });

        expect(appBody.classList.contains("app-body--dragging")).toBe(true);
        expect(document.body.style.userSelect).toBe("none");

        act(() => {
            window.dispatchEvent(new MouseEvent("mousemove", {clientX: 388}));
        });

        expect(mocked.setDockRatio).toHaveBeenCalledTimes(1);
        expect(mocked.setDockRatio.mock.calls[0]?.[0]).toBeCloseTo(0.6);
    });

    it("uses the scaled docked content span and cleans up drag state on blur", () => {
        const {appBody, divider} = renderDivider();

        vi.spyOn(appBody, "getBoundingClientRect").mockReturnValue(createRect(100, 1200));
        document.body.style.userSelect = "text";

        act(() => {
            divider.dispatchEvent(new MouseEvent("mousedown", {bubbles: true, clientX: 260}));
        });

        act(() => {
            window.dispatchEvent(new MouseEvent("mousemove", {clientX: 360}));
        });

        expect(mocked.setDockRatio).toHaveBeenCalledTimes(1);
        // 1200px viewport: hand-calculated displayed content width is about 927.33px.
        expect(mocked.setDockRatio.mock.calls[0]?.[0]).toBeCloseTo(0.6078, 3);

        act(() => {
            window.dispatchEvent(new Event("blur"));
        });

        expect(appBody.classList.contains("app-body--dragging")).toBe(false);
        expect(document.body.style.userSelect).toBe("text");

        act(() => {
            window.dispatchEvent(new MouseEvent("mousemove", {clientX: 340}));
        });

        expect(mocked.setDockRatio).toHaveBeenCalledTimes(1);
    });

    it("normalizes transient widths below the runtime minimum before drag math", () => {
        const {appBody, divider} = renderDivider();

        vi.spyOn(appBody, "getBoundingClientRect").mockReturnValue(createRect(100, 600));

        act(() => {
            divider.dispatchEvent(new MouseEvent("mousedown", {bubbles: true, clientX: 260}));
        });

        act(() => {
            window.dispatchEvent(new MouseEvent("mousemove", {clientX: 360}));
        });

        const appScale = DOCKED_WINDOW_MIN_WIDTH / DOCKED_LAYOUT_BASE_WIDTH;
        const visibleContentWidth =
            DOCKED_WINDOW_MIN_WIDTH -
            (DOCKED_SIDEBAR_WIDTH * appScale) -
            (DOCKED_DIVIDER_WIDTH * appScale) -
            DOCKED_ACTIVITY_STRIP_WIDTH;
        const expectedRatio = 0.5 + (360 - 260) / visibleContentWidth;
        expect(mocked.setDockRatio).toHaveBeenCalledTimes(1);
        expect(mocked.setDockRatio.mock.calls[0]?.[0]).toBeCloseTo(expectedRatio);
    });

    it("does not update the dock ratio when the container width is zero", () => {
        const {appBody, divider} = renderDivider();
        const errorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
        vi.spyOn(appBody, "getBoundingClientRect").mockReturnValue(createRect(100, 0));

        act(() => {
            divider.dispatchEvent(new MouseEvent("mousedown", {bubbles: true, clientX: 260}));
        });

        act(() => {
            window.dispatchEvent(new MouseEvent("mousemove", {clientX: 300}));
        });

        expect(errorSpy).toHaveBeenCalledWith("[DockedDivider] BUG: appBody viewport width is", 0);
        expect(appBody.classList.contains("app-body--dragging")).toBe(false);
        expect(mocked.setDockRatio).not.toHaveBeenCalled();
    });

    it("calculates the delta from the original drag start across multiple mousemoves", () => {
        const {appBody, divider} = renderDivider();
        vi.spyOn(appBody, "getBoundingClientRect").mockReturnValue(createRect(100, 1600));

        act(() => {
            divider.dispatchEvent(new MouseEvent("mousedown", {bubbles: true, clientX: 260}));
        });

        act(() => {
            window.dispatchEvent(new MouseEvent("mousemove", {clientX: 388}));
        });

        expect(mocked.setDockRatio.mock.calls[0]?.[0]).toBeCloseTo(0.6);

        act(() => {
            window.dispatchEvent(new MouseEvent("mousemove", {clientX: 516}));
        });

        expect(mocked.setDockRatio.mock.calls[1]?.[0]).toBeCloseTo(0.7);
    });

    it("starts a new drag from the latest dock ratio", () => {
        const {appBody, divider} = renderDivider();
        vi.spyOn(appBody, "getBoundingClientRect").mockReturnValue(createRect(100, 1600));

        act(() => {
            divider.dispatchEvent(new MouseEvent("mousedown", {bubbles: true, clientX: 260}));
        });

        act(() => {
            window.dispatchEvent(new MouseEvent("mousemove", {clientX: 388}));
            window.dispatchEvent(new Event("blur"));
        });

        expect(mocked.dockRatio).toBe(0.6);

        act(() => {
            divider.dispatchEvent(new MouseEvent("mousedown", {bubbles: true, clientX: 300}));
        });

        act(() => {
            window.dispatchEvent(new MouseEvent("mousemove", {clientX: 428}));
        });

        expect(mocked.setDockRatio.mock.calls[1]?.[0]).toBeCloseTo(0.7);
        expect(mocked.dockRatio).toBe(0.7);
    });

    it("errors and aborts when the expected app-body ancestor is missing", () => {
        const errorSpy = vi.spyOn(console, "error").mockImplementation(() => {});

        act(() => {
            root.render(<DockedDivider/>);
        });

        const divider = container.querySelector<HTMLDivElement>(".docked-divider");
        expect(divider).not.toBeNull();

        act(() => {
            divider!.dispatchEvent(new MouseEvent("mousedown", {bubbles: true, clientX: 260}));
        });

        expect(errorSpy).toHaveBeenCalledWith("[DockedDivider] BUG: .app-body not found in ancestor chain");
        expect(mocked.setDockRatio).not.toHaveBeenCalled();
    });

    it("cleans up drag state on mouseup", () => {
        const {appBody, divider} = renderDivider();
        vi.spyOn(appBody, "getBoundingClientRect").mockReturnValue(createRect(100, 1600));
        document.body.style.userSelect = "text";

        act(() => {
            divider.dispatchEvent(new MouseEvent("mousedown", {bubbles: true, clientX: 260}));
        });

        expect(appBody.classList.contains("app-body--dragging")).toBe(true);

        act(() => {
            window.dispatchEvent(new MouseEvent("mouseup", {clientX: 320}));
        });

        expect(appBody.classList.contains("app-body--dragging")).toBe(false);
        expect(document.body.style.userSelect).toBe("text");

        act(() => {
            window.dispatchEvent(new MouseEvent("mousemove", {clientX: 388}));
        });

        expect(mocked.setDockRatio).not.toHaveBeenCalled();
    });

    it("cleans up global drag listeners when the component unmounts mid-drag", () => {
        const {appBody, divider} = renderDivider();
        vi.spyOn(appBody, "getBoundingClientRect").mockReturnValue(createRect(100, 1600));
        document.body.style.userSelect = "text";

        act(() => {
            divider.dispatchEvent(new MouseEvent("mousedown", {bubbles: true, clientX: 260}));
        });

        act(() => {
            root.unmount();
        });

        expect(appBody.classList.contains("app-body--dragging")).toBe(false);
        expect(document.body.style.userSelect).toBe("text");

        act(() => {
            window.dispatchEvent(new MouseEvent("mousemove", {clientX: 388}));
        });

        expect(mocked.setDockRatio).not.toHaveBeenCalled();
    });

    it("resets the dock ratio on double click", () => {
        const {divider} = renderDivider();

        act(() => {
            divider.dispatchEvent(new MouseEvent("dblclick", {bubbles: true}));
        });

        expect(mocked.resetDockRatio).toHaveBeenCalledTimes(1);
    });

    it("renders with separator role and keyboard accessibility attributes", () => {
        const {divider} = renderDivider();

        expect(divider.getAttribute("role")).toBe("separator");
        expect(divider.getAttribute("aria-orientation")).toBe("vertical");
        expect(divider.getAttribute("aria-label")).toBe("Resize panels");
        expect(divider.getAttribute("tabindex")).toBe("0");
        expect(divider.getAttribute("aria-valuemin")).toBe("0");
        expect(divider.getAttribute("aria-valuemax")).toBe("100");
        expect(divider.getAttribute("aria-valuenow")).toBe("50");
    });

    it("moves the dock ratio left on ArrowLeft keydown", () => {
        const {divider} = renderDivider();

        act(() => {
            divider.dispatchEvent(new KeyboardEvent("keydown", {key: "ArrowLeft", bubbles: true}));
        });

        expect(mocked.setDockRatio).toHaveBeenCalledTimes(1);
        expect(mocked.setDockRatio.mock.calls[0]?.[0]).toBeCloseTo(0.48);
    });

    it("moves the dock ratio right on ArrowRight keydown", () => {
        const {divider} = renderDivider();

        act(() => {
            divider.dispatchEvent(new KeyboardEvent("keydown", {key: "ArrowRight", bubbles: true}));
        });

        expect(mocked.setDockRatio).toHaveBeenCalledTimes(1);
        expect(mocked.setDockRatio.mock.calls[0]?.[0]).toBeCloseTo(0.52);
    });

    it("jumps to minimum ratio on Home keydown", () => {
        const {divider} = renderDivider();

        act(() => {
            divider.dispatchEvent(new KeyboardEvent("keydown", {key: "Home", bubbles: true}));
        });

        expect(mocked.setDockRatio).toHaveBeenCalledTimes(1);
        expect(mocked.setDockRatio.mock.calls[0]?.[0]).toBe(DOCK_RATIO_MIN);
    });

    it("jumps to maximum ratio on End keydown", () => {
        const {divider} = renderDivider();

        act(() => {
            divider.dispatchEvent(new KeyboardEvent("keydown", {key: "End", bubbles: true}));
        });

        expect(mocked.setDockRatio).toHaveBeenCalledTimes(1);
        expect(mocked.setDockRatio.mock.calls[0]?.[0]).toBe(DOCK_RATIO_MAX);
    });

    it("ignores unrelated keys without calling setDockRatio", () => {
        const {divider} = renderDivider();

        act(() => {
            divider.dispatchEvent(new KeyboardEvent("keydown", {key: "Tab", bubbles: true}));
        });

        expect(mocked.setDockRatio).not.toHaveBeenCalled();
    });
});
