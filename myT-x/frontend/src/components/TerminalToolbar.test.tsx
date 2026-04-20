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
});
