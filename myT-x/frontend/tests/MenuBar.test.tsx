import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it} from "vitest";
import {MenuBar} from "../src/components/MenuBar";
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
            root.render(<MenuBar onOpenSettings={() => {
            }}/>);
        });

        const button = container.querySelector<HTMLButtonElement>(".menu-bar-ime-reset");
        expect(button).not.toBeNull();
        expect(button?.disabled).toBe(false);
    });
});
