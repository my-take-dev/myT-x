import {act} from "react";
import type {MouseEvent as ReactMouseEvent} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {getLanguage, setLanguage} from "../i18n";
import {AutoStartPopover} from "./AutoStartPopover";

describe("AutoStartPopover", () => {
    let container: HTMLDivElement;
    let root: Root;
    let previousLanguage: ReturnType<typeof getLanguage>;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
        previousLanguage = getLanguage();
        setLanguage("en");
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        container.remove();
        setLanguage(previousLanguage);
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = false;
    });

    it("renders command previews and starts selected entry", () => {
        const onStart = vi.fn();

        act(() => {
            root.render(
                <AutoStartPopover
                    entries={[
                        {name: "Mini Codex", command: "codex", args: "--model gpt-5.4-mini"},
                        {name: "Blank", command: "   ", args: ""},
                    ]}
                    onStart={onStart}
                    onClose={vi.fn()}
                    startDisabled={false}
                    preventTerminalFocusSteal={vi.fn()}
                />,
            );
        });

        const button = container.querySelector(".auto-start-command-btn") as HTMLButtonElement;
        expect(button).not.toBeNull();
        expect(button.textContent).toContain("Mini Codex");
        expect(button.textContent).toContain("[codex --model gpt-5.4-mini]");

        act(() => {
            button.click();
        });

        expect(onStart).toHaveBeenCalledWith({
            name: "Mini Codex",
            command: "codex",
            args: "--model gpt-5.4-mini",
        });
        expect(container.querySelectorAll(".auto-start-command-btn")).toHaveLength(1);
    });

    it("closes on Escape", () => {
        const onClose = vi.fn();

        act(() => {
            root.render(
                <AutoStartPopover
                    entries={[{name: "", command: "pwsh.exe", args: ""}]}
                    onStart={vi.fn()}
                    onClose={onClose}
                    startDisabled={false}
                    preventTerminalFocusSteal={vi.fn()}
                />,
            );
        });

        act(() => {
            document.dispatchEvent(new KeyboardEvent("keydown", {key: "Escape"}));
        });

        expect(onClose).toHaveBeenCalledTimes(1);
    });

    it("disables command buttons while start is in flight", () => {
        const onStart = vi.fn();

        act(() => {
            root.render(
                <AutoStartPopover
                    entries={[{name: "", command: "pwsh.exe", args: ""}]}
                    onStart={onStart}
                    onClose={vi.fn()}
                    startDisabled={true}
                    preventTerminalFocusSteal={vi.fn()}
                />,
            );
        });

        const button = container.querySelector(".auto-start-command-btn") as HTMLButtonElement;
        expect(button.disabled).toBe(true);

        act(() => {
            button.click();
        });

        expect(onStart).not.toHaveBeenCalled();
    });

    it("prevents command button mousedown from refocusing the terminal", () => {
        const preventTerminalFocusSteal = vi.fn((event: ReactMouseEvent<HTMLElement>) => {
            event.preventDefault();
            event.stopPropagation();
        });

        act(() => {
            root.render(
                <AutoStartPopover
                    entries={[{name: "", command: "pwsh.exe", args: ""}]}
                    onStart={vi.fn()}
                    onClose={vi.fn()}
                    startDisabled={false}
                    preventTerminalFocusSteal={preventTerminalFocusSteal}
                />,
            );
        });

        const button = container.querySelector(".auto-start-command-btn") as HTMLButtonElement;
        expect(button).not.toBeNull();

        const mouseDown = new MouseEvent("mousedown", {bubbles: true, cancelable: true});
        act(() => {
            button.dispatchEvent(mouseDown);
        });

        expect(preventTerminalFocusSteal).toHaveBeenCalledTimes(1);
        expect(mouseDown.defaultPrevented).toBe(true);
    });
});
