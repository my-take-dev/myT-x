import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {getLanguage, setLanguage} from "../i18n";
import {setFieldValue} from "../testUtils/setFieldValue";
import {AutoEnterPopover} from "./AutoEnterPopover";

describe("AutoEnterPopover", () => {
    let container: HTMLDivElement;
    let root: Root;
    let previousLanguage: ReturnType<typeof getLanguage>;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        (globalThis as {IS_REACT_ACT_ENVIRONMENT?: boolean}).IS_REACT_ACT_ENVIRONMENT = true;
        previousLanguage = getLanguage();
        setLanguage("en");
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        container.remove();
        setLanguage(previousLanguage);
        (globalThis as {IS_REACT_ACT_ENVIRONMENT?: boolean}).IS_REACT_ACT_ENVIRONMENT = false;
    });

    function renderPopover(
        onStart = vi.fn(),
        parentClick = vi.fn(),
        parentMouseDown = vi.fn(),
    ): void {
        act(() => {
            root.render(
                <div onClick={parentClick} onMouseDown={parentMouseDown}>
                    <AutoEnterPopover
                        onStart={onStart}
                        onClose={vi.fn()}
                        preventTerminalFocusSteal={(event) => {
                            event.preventDefault();
                            event.stopPropagation();
                        }}
                    />
                </div>,
            );
        });
    }

    it("does not prevent the interval input mousedown default action", () => {
        renderPopover();
        const input = container.querySelector<HTMLInputElement>(".auto-enter-popover-input");
        expect(input).not.toBeNull();

        const mouseDown = new MouseEvent("mousedown", {bubbles: true, cancelable: true});
        act(() => {
            input?.dispatchEvent(mouseDown);
        });

        expect(mouseDown.defaultPrevented).toBe(false);
    });

    it("keeps interval input mousedown from bubbling to the parent pane", () => {
        const parentMouseDown = vi.fn();
        renderPopover(vi.fn(), vi.fn(), parentMouseDown);
        const input = container.querySelector<HTMLInputElement>(".auto-enter-popover-input");
        expect(input).not.toBeNull();

        const mouseDown = new MouseEvent("mousedown", {bubbles: true, cancelable: true});
        act(() => {
            input?.dispatchEvent(mouseDown);
        });

        expect(parentMouseDown).not.toHaveBeenCalled();
    });

    it("keeps interval input clicks from bubbling to the parent pane", () => {
        const parentClick = vi.fn();
        renderPopover(vi.fn(), parentClick);
        const input = container.querySelector<HTMLInputElement>(".auto-enter-popover-input");
        expect(input).not.toBeNull();

        act(() => {
            input?.click();
        });

        expect(parentClick).not.toHaveBeenCalled();
    });

    it("keeps start button clicks from bubbling to the parent pane", () => {
        const parentClick = vi.fn();
        renderPopover(vi.fn(), parentClick);
        const startButton = container.querySelector<HTMLButtonElement>(".auto-enter-popover-start-btn");
        expect(startButton).not.toBeNull();

        act(() => {
            startButton?.click();
        });

        expect(parentClick).not.toHaveBeenCalled();
    });

    it("prevents non-input mousedown from refocusing the terminal", () => {
        renderPopover();
        const startButton = container.querySelector<HTMLButtonElement>(".auto-enter-popover-start-btn");
        expect(startButton).not.toBeNull();

        const mouseDown = new MouseEvent("mousedown", {bubbles: true, cancelable: true});
        act(() => {
            startButton?.dispatchEvent(mouseDown);
        });

        expect(mouseDown.defaultPrevented).toBe(true);
    });

    it("starts with the edited interval seconds", () => {
        const onStart = vi.fn();
        renderPopover(onStart);
        const input = container.querySelector<HTMLInputElement>(".auto-enter-popover-input");
        const startButton = container.querySelector<HTMLButtonElement>(".auto-enter-popover-start-btn");
        expect(input).not.toBeNull();
        expect(startButton).not.toBeNull();

        setFieldValue(input!, "45");
        act(() => {
            startButton?.click();
        });

        expect(onStart).toHaveBeenCalledWith(45);
    });
});
