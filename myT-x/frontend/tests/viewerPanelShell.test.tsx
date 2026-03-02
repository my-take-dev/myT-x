import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {ViewerPanelShell} from "../src/components/viewer/views/shared/ViewerPanelShell";

describe("ViewerPanelShell", () => {
    let container: HTMLDivElement;
    let root: Root;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        container.remove();
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = false;
    });

    it("renders children when message is not provided", () => {
        act(() => {
            root.render(
                <ViewerPanelShell
                    className="viewer-shell-test"
                    title="Test"
                    onClose={vi.fn()}
                >
                    <div data-testid="body">body-content</div>
                </ViewerPanelShell>,
            );
        });

        expect(container.querySelector('[data-testid="body"]')?.textContent).toBe("body-content");
        expect(container.querySelector(".viewer-message")).toBeNull();
    });

    it("renders message in .viewer-message when non-empty message is provided", () => {
        act(() => {
            root.render(
                <ViewerPanelShell
                    className="viewer-shell-test"
                    title="Test"
                    onClose={vi.fn()}
                    message="Some error occurred"
                />,
            );
        });

        const messageEl = container.querySelector(".viewer-message");
        expect(messageEl).not.toBeNull();
        expect(messageEl?.textContent).toBe("Some error occurred");
    });

    it("treats empty-string message as no-message (no empty .viewer-message div)", () => {
        act(() => {
            root.render(
                <ViewerPanelShell
                    className="viewer-shell-test"
                    title="Test"
                    onClose={vi.fn()}
                    message=""
                />,
            );
        });

        // Empty string is treated as "no message" to avoid rendering
        // an empty .viewer-message div that blocks content display.
        expect(container.querySelector(".viewer-message")).toBeNull();
    });
});
