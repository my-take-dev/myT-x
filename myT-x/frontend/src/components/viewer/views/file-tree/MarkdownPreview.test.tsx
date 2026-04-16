import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {MarkdownPreview} from "./MarkdownPreview";

describe("MarkdownPreview", () => {
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

    it("routes mermaid fenced code blocks through the custom renderer", async () => {
        const codeBlockRenderer = vi.fn(({className, code}: {className?: string; code: string}) => {
            if (className !== "language-mermaid") {
                return null;
            }

            return <div data-testid="mermaid-block">{code}</div>;
        });

        await act(async () => {
            root.render(
                <MarkdownPreview
                    content={"```mermaid\ngraph TD;\n```"}
                    codeBlockRenderer={codeBlockRenderer}
                />,
            );
        });

        expect(codeBlockRenderer).toHaveBeenCalledWith({
            className: "language-mermaid",
            code: "graph TD;",
        });
        expect(container.querySelector("[data-testid='mermaid-block']")?.textContent).toBe("graph TD;");
    });
});
