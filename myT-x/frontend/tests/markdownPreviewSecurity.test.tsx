import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it} from "vitest";
import {MarkdownPreview} from "../src/components/viewer/views/file-tree/MarkdownPreview";
import type {PluggableList} from "unified";

const MARK_PARAGRAPH_PLUGIN: PluggableList = [() => (tree: any) => {
    const visit = (node: any) => {
        if (!node || typeof node !== "object") {
            return false;
        }
        if (node.type === "element" && node.tagName === "p") {
            node.properties = {
                ...(node.properties ?? {}),
                "data-testid": "rehype-hit",
            };
            return true;
        }
        const children = Array.isArray(node.children) ? node.children : [];
        for (const child of children) {
            if (visit(child)) {
                return true;
            }
        }
        return false;
    };

    visit(tree);
}];

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

    it("accepts rehype plugins without changing existing callers", () => {
        act(() => {
            root.render(<MarkdownPreview content="paragraph" rehypePlugins={MARK_PARAGRAPH_PLUGIN}/>);
        });

        expect(container.querySelector("[data-testid='rehype-hit']")?.textContent).toBe("paragraph");
    });

    it("marks blocked links with blocked accessibility labels", () => {
        renderPreview("[relative](./notes.txt)\n\n[unsupported](foo/bar)");

        const links = container.querySelectorAll("a");
        expect(links[0]?.getAttribute("aria-label")).toBe("Blocked in-app link: ./notes.txt");
        expect(links[1]?.getAttribute("aria-label")).toBe("Blocked unsupported link: foo/bar");
    });
});
