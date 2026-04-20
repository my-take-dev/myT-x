import {act, createRef} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {MenuBar} from "../src/components/MenuBar";
import {QUICK_SEARCH_DIALOG_ID} from "../src/components/quickSearchShared";
import {useTmuxStore} from "../src/stores/tmuxStore";

describe("MenuBar", () => {
    let container: HTMLDivElement;
    let root: Root;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        useTmuxStore.setState({activeSession: null});
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        container.remove();
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = false;
    });

    it("keeps the IME reset button enabled without an active session", () => {
        act(() => {
            root.render(
                <MenuBar
                    onOpenSettings={() => {
                    }}
                    onOpenQuickSearch={() => {
                    }}
                    isQuickSearchOpen={false}
                />,
            );
        });

        const button = container.querySelector<HTMLButtonElement>(".menu-bar-ime-reset");
        expect(button).not.toBeNull();
        expect(button?.disabled).toBe(false);
    });

    it("opens quick search from the centered trigger", () => {
        const onOpenQuickSearch = vi.fn();

        act(() => {
            root.render(
                <MenuBar
                    onOpenSettings={() => {
                    }}
                    onOpenQuickSearch={onOpenQuickSearch}
                    isQuickSearchOpen={false}
                />,
            );
        });

        const button = container.querySelector<HTMLButtonElement>(".menu-bar-search-trigger");
        if (button === null) {
            throw new Error("expected quick search trigger");
        }

        act(() => {
            button.click();
        });

        expect(onOpenQuickSearch).toHaveBeenCalledTimes(1);
    });

    it("forwards the quick search trigger ref for anchored dropdown positioning", () => {
        const quickSearchTriggerRef = createRef<HTMLButtonElement>();

        act(() => {
            root.render(
                <MenuBar
                    onOpenSettings={() => {
                    }}
                    onOpenQuickSearch={() => {
                    }}
                    isQuickSearchOpen={false}
                    quickSearchTriggerRef={quickSearchTriggerRef}
                />,
            );
        });

        const button = container.querySelector<HTMLButtonElement>(".menu-bar-search-trigger");
        expect(quickSearchTriggerRef.current).toBe(button);
    });

    it("announces quick search trigger state and dialog semantics", () => {
        act(() => {
            root.render(
                <MenuBar
                    onOpenSettings={() => {
                    }}
                    onOpenQuickSearch={() => {
                    }}
                    isQuickSearchOpen
                />,
            );
        });

        const button = container.querySelector<HTMLButtonElement>(".menu-bar-search-trigger");
        const placeholder = container.querySelector<HTMLElement>(".menu-bar-search-placeholder");

        expect(button?.getAttribute("aria-haspopup")).toBe("dialog");
        expect(button?.getAttribute("aria-expanded")).toBe("true");
        expect(button?.getAttribute("aria-controls")).toBe(QUICK_SEARCH_DIALOG_ID);
        expect(placeholder?.getAttribute("aria-hidden")).toBe("true");
    });
});
