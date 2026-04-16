import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {PaneChatBar} from "../src/components/PaneChatBar";
import {useChatStore} from "../src/stores/chatStore";

describe("PaneChatBar", () => {
    let container: HTMLDivElement;
    let root: Root;
    const preventTerminalFocusSteal = vi.fn();

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        useChatStore.setState({requestedPaneId: null});
        preventTerminalFocusSteal.mockReset();
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        container.remove();
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = false;
    });

    it("renders paneId in the bar", () => {
        act(() => {
            root.render(
                <PaneChatBar paneId="%1" preventTerminalFocusSteal={preventTerminalFocusSteal}/>,
            );
        });

        const paneLabel = container.querySelector(".pane-chat-bar-pane");
        expect(paneLabel).not.toBeNull();
        expect(paneLabel!.textContent).toBe("%1");
    });

    it("renders placeholder text", () => {
        act(() => {
            root.render(
                <PaneChatBar paneId="%0" preventTerminalFocusSteal={preventTerminalFocusSteal}/>,
            );
        });

        const placeholder = container.querySelector(".pane-chat-bar-placeholder");
        expect(placeholder).not.toBeNull();
        expect(placeholder!.textContent).toBeTruthy();
    });

    it("calls requestOpen on click", () => {
        act(() => {
            root.render(
                <PaneChatBar paneId="%3" preventTerminalFocusSteal={preventTerminalFocusSteal}/>,
            );
        });

        const bar = container.querySelector(".pane-chat-bar")!;
        act(() => {
            bar.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });

        expect(useChatStore.getState().requestedPaneId).toBe("%3");
    });

    it("stops click propagation so parent pane focus handlers do not run", () => {
        const parentClick = vi.fn();

        act(() => {
            root.render(
                <div onClick={parentClick}>
                    <PaneChatBar paneId="%4" preventTerminalFocusSteal={preventTerminalFocusSteal}/>
                </div>,
            );
        });

        const bar = container.querySelector(".pane-chat-bar")!;
        act(() => {
            bar.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });

        expect(useChatStore.getState().requestedPaneId).toBe("%4");
        expect(parentClick).not.toHaveBeenCalled();
    });

    it("calls requestOpen on Enter key", () => {
        act(() => {
            root.render(
                <PaneChatBar paneId="%5" preventTerminalFocusSteal={preventTerminalFocusSteal}/>,
            );
        });

        const bar = container.querySelector(".pane-chat-bar")!;
        act(() => {
            bar.dispatchEvent(new KeyboardEvent("keydown", {key: "Enter", bubbles: true}));
        });

        expect(useChatStore.getState().requestedPaneId).toBe("%5");
    });

    it("calls requestOpen on Space key", () => {
        act(() => {
            root.render(
                <PaneChatBar paneId="%6" preventTerminalFocusSteal={preventTerminalFocusSteal}/>,
            );
        });

        const bar = container.querySelector(".pane-chat-bar")!;
        act(() => {
            bar.dispatchEvent(new KeyboardEvent("keydown", {key: " ", bubbles: true}));
        });

        expect(useChatStore.getState().requestedPaneId).toBe("%6");
    });

    it("does not call requestOpen on unrelated key", () => {
        act(() => {
            root.render(
                <PaneChatBar paneId="%7" preventTerminalFocusSteal={preventTerminalFocusSteal}/>,
            );
        });

        const bar = container.querySelector(".pane-chat-bar")!;
        act(() => {
            bar.dispatchEvent(new KeyboardEvent("keydown", {key: "Tab", bubbles: true}));
        });

        expect(useChatStore.getState().requestedPaneId).toBeNull();
    });

    it("calls preventTerminalFocusSteal on mousedown", () => {
        act(() => {
            root.render(
                <PaneChatBar paneId="%8" preventTerminalFocusSteal={preventTerminalFocusSteal}/>,
            );
        });

        const bar = container.querySelector(".pane-chat-bar")!;
        act(() => {
            bar.dispatchEvent(new MouseEvent("mousedown", {bubbles: true}));
        });

        expect(preventTerminalFocusSteal).toHaveBeenCalledOnce();
    });

    it("has role=button and tabIndex=0 for accessibility", () => {
        act(() => {
            root.render(
                <PaneChatBar paneId="%0" preventTerminalFocusSteal={preventTerminalFocusSteal}/>,
            );
        });

        const bar = container.querySelector(".pane-chat-bar")!;
        expect(bar.getAttribute("role")).toBe("button");
        expect(bar.getAttribute("tabindex")).toBe("0");
    });
});
