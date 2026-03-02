import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it} from "vitest";
import {MarkdownPreview} from "../src/components/viewer/views/file-tree/MarkdownPreview";

describe("MarkdownPreview image security", () => {
    let container: HTMLDivElement;
    let root: Root;

    const renderPreview = (content: string) => {
        act(() => {
            root.render(<MarkdownPreview content={content}/>);
        });
    };

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

    it("blocks external images with explicit schemes", () => {
        renderPreview("![remote](https://example.com/image.png)");

        expect(container.querySelector("img")).toBeNull();
        const blocked = container.querySelector(".md-preview-unsafe-img");
        expect(blocked).toBeTruthy();
        expect(blocked?.textContent).toContain("[image blocked]");
    });

    it("allows relative image paths", () => {
        renderPreview("![local](./local.png)");

        const image = container.querySelector("img");
        expect(image).toBeTruthy();
        expect(image?.getAttribute("src")).toBe("./local.png");
        expect(container.querySelector(".md-preview-unsafe-img")).toBeNull();
    });
});
