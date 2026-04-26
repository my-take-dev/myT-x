import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {setLanguage} from "../i18n";
import {TerminalToolbar} from "./TerminalToolbar";

function renderToolbar(root: Root): void {
    root.render(
        <TerminalToolbar
            paneId="%1"
            titleDraft="Pane"
            renameBusy={false}
            autoRunning={false}
            onTitleEditStart={vi.fn()}
            onTitleChange={vi.fn()}
            onTitleCommit={vi.fn()}
            onTitleCancel={vi.fn()}
            onAutoClick={vi.fn()}
            onAutoStartClick={vi.fn()}
            autoStartDisabled={false}
            onSplitVertical={vi.fn()}
            onSplitHorizontal={vi.fn()}
            onAddMember={vi.fn()}
            onClose={vi.fn()}
            preventTerminalFocusSteal={vi.fn()}
        />,
    );
}

describe("TerminalToolbar", () => {
    let container: HTMLDivElement;
    let root: Root;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
        setLanguage("en");
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        container.remove();
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = false;
    });

    it("hides the root toggle outside canvas mode", () => {
        act(() => {
            renderToolbar(root);
        });

        expect(container.querySelector('[aria-label="Set pane %1 as tree root"]')).toBeNull();
    });

    it("renders the root toggle and forwards clicks", () => {
        const onRootToggle = vi.fn();

        act(() => {
            root.render(
                <TerminalToolbar
                    paneId="%1"
                    titleDraft="Pane"
                    renameBusy={false}
                    autoRunning={false}
                    isRootToggleVisible
                    isRootPane
                    onTitleEditStart={vi.fn()}
                    onTitleChange={vi.fn()}
                    onTitleCommit={vi.fn()}
                    onTitleCancel={vi.fn()}
                    onAutoClick={vi.fn()}
                    onAutoStartClick={vi.fn()}
                    autoStartDisabled={false}
                    onRootToggle={onRootToggle}
                    onSplitVertical={vi.fn()}
                    onSplitHorizontal={vi.fn()}
                    onAddMember={vi.fn()}
                    onClose={vi.fn()}
                    preventTerminalFocusSteal={vi.fn()}
                />,
            );
        });

        const button = container.querySelector('[aria-label="Unset pane %1 as tree root"]');
        expect(button).not.toBeNull();
        expect(button?.getAttribute("aria-pressed")).toBe("true");
        expect(button?.className).toContain("terminal-toolbar-btn-root-active");

        act(() => {
            (button as HTMLButtonElement).click();
        });

        expect(onRootToggle).toHaveBeenCalledTimes(1);
    });

    it("localizes the root toggle in Japanese mode", () => {
        setLanguage("ja");

        act(() => {
            root.render(
                <TerminalToolbar
                    paneId="%1"
                    titleDraft="Pane"
                    renameBusy={false}
                    autoRunning={false}
                    isRootToggleVisible
                    onTitleEditStart={vi.fn()}
                    onTitleChange={vi.fn()}
                    onTitleCommit={vi.fn()}
                    onTitleCancel={vi.fn()}
                    onAutoClick={vi.fn()}
                    onAutoStartClick={vi.fn()}
                    autoStartDisabled={false}
                    onRootToggle={vi.fn()}
                    onSplitVertical={vi.fn()}
                    onSplitHorizontal={vi.fn()}
                    onAddMember={vi.fn()}
                    onClose={vi.fn()}
                    preventTerminalFocusSteal={vi.fn()}
                />,
            );
        });

        const button = container.querySelector('[aria-label="ペイン %1 をツリールートに設定"]');
        expect(button).not.toBeNull();
        expect(button?.getAttribute("title")).toBe("ツリールートに設定");
    });

    it("places AutoStart between Add Member and Close", () => {
        const onAutoStartClick = vi.fn();

        act(() => {
            root.render(
                <TerminalToolbar
                    paneId="%1"
                    titleDraft="Pane"
                    renameBusy={false}
                    autoRunning={false}
                    onTitleEditStart={vi.fn()}
                    onTitleChange={vi.fn()}
                    onTitleCommit={vi.fn()}
                    onTitleCancel={vi.fn()}
                    onAutoClick={vi.fn()}
                    onAutoStartClick={onAutoStartClick}
                    autoStartDisabled={false}
                    onSplitVertical={vi.fn()}
                    onSplitHorizontal={vi.fn()}
                    onAddMember={vi.fn()}
                    onClose={vi.fn()}
                    preventTerminalFocusSteal={vi.fn()}
                />,
            );
        });

        const buttons = Array.from(container.querySelectorAll(".terminal-toolbar-actions button"));
        const labels = buttons.map((button) => button.getAttribute("aria-label"));
        const addMemberIndex = labels.indexOf("Add member to pane %1");
        const autoStartIndex = labels.indexOf("Open AutoStart commands for pane %1");
        const closeIndex = labels.indexOf("Close pane %1");
        expect(addMemberIndex).toBeGreaterThanOrEqual(0);
        expect(autoStartIndex).toBe(addMemberIndex + 1);
        expect(closeIndex).toBe(autoStartIndex + 1);

        act(() => {
            (buttons[autoStartIndex] as HTMLButtonElement).click();
        });
        expect(onAutoStartClick).toHaveBeenCalledTimes(1);
    });

    it("keeps AutoStart visible but disabled without entries", () => {
        const onAutoStartClick = vi.fn();

        act(() => {
            root.render(
                <TerminalToolbar
                    paneId="%1"
                    titleDraft="Pane"
                    renameBusy={false}
                    autoRunning={false}
                    onTitleEditStart={vi.fn()}
                    onTitleChange={vi.fn()}
                    onTitleCommit={vi.fn()}
                    onTitleCancel={vi.fn()}
                    onAutoClick={vi.fn()}
                    onAutoStartClick={onAutoStartClick}
                    autoStartDisabled
                    onSplitVertical={vi.fn()}
                    onSplitHorizontal={vi.fn()}
                    onAddMember={vi.fn()}
                    onClose={vi.fn()}
                    preventTerminalFocusSteal={vi.fn()}
                />,
            );
        });

        const button = container.querySelector('[aria-label="Open AutoStart commands for pane %1"]') as HTMLButtonElement;
        expect(button).not.toBeNull();
        expect(button.disabled).toBe(true);
        act(() => {
            button.click();
        });
        expect(onAutoStartClick).not.toHaveBeenCalled();
    });
});
