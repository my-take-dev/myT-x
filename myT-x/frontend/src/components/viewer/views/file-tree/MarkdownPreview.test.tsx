import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {MarkdownPreview} from "./MarkdownPreview";

describe("MarkdownPreview", () => {
    let container: HTMLDivElement;
    let root: Root;
    let originalScrollIntoView: typeof HTMLElement.prototype.scrollIntoView;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
        originalScrollIntoView = HTMLElement.prototype.scrollIntoView;
        HTMLElement.prototype.scrollIntoView = vi.fn();
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        container.remove();
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = false;
        HTMLElement.prototype.scrollIntoView = originalScrollIntoView;
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

    it("keeps single tildes as literal text instead of strikethrough", async () => {
        await act(async () => {
            root.render(<MarkdownPreview content={"alpha ~beta~ gamma"}/>);
        });

        expect(container.querySelector("del")).toBeNull();
        expect(container.textContent).toContain("alpha ~beta~ gamma");
    });

    it("renders a markdown outline with stable heading anchors", async () => {
        await act(async () => {
            root.render(<MarkdownPreview content={"# Intro\n\n## Details\n\n# Intro"}/>);
        });

        const outlineItems = container.querySelectorAll<HTMLButtonElement>(".md-preview-outline-item");
        const headings = container.querySelectorAll("h1, h2");

        expect(Array.from(outlineItems, (item) => item.textContent)).toEqual(["Intro", "Details", "Intro"]);
        expect(headings[0]?.id).toBe("intro");
        expect(headings[1]?.id).toBe("details");
        expect(headings[2]?.id).toBe("intro-1");
        expect(outlineItems).toHaveLength(3);

        await act(async () => {
            outlineItems[1]!.click();
        });

        expect(HTMLElement.prototype.scrollIntoView).toHaveBeenCalledTimes(1);
        expect(HTMLElement.prototype.scrollIntoView).toHaveBeenCalledWith({
            behavior: "smooth",
            block: "start",
        });
    });

    it("keeps heading IDs unique when a numeric heading suffix collides with duplicates", async () => {
        await act(async () => {
            root.render(<MarkdownPreview content={"## Intro-1\n\n## Intro\n\n## Intro"}/>);
        });

        const headings = container.querySelectorAll("h2");
        expect(Array.from(headings, (heading) => heading.id)).toEqual(["intro-1", "intro", "intro-2"]);
    });

    it("does not mark in-page anchor links as disabled", async () => {
        await act(async () => {
            root.render(<MarkdownPreview content={"# Details\n\n[Go to details](#details)"}/>);
        });

        const link = container.querySelector<HTMLAnchorElement>("a");
        expect(link).not.toBeNull();
        expect(link?.getAttribute("title")).toBeNull();
        expect(link?.classList.contains("md-preview-disabled-link")).toBe(false);
    });

    it("scrolls to the target heading when an in-page anchor link is clicked", async () => {
        await act(async () => {
            root.render(<MarkdownPreview content={"# Details\n\n[Go to details](#details)"}/>);
        });

        const link = container.querySelector<HTMLAnchorElement>("a");
        expect(link).not.toBeNull();

        await act(async () => {
            link!.click();
        });

        expect(HTMLElement.prototype.scrollIntoView).toHaveBeenCalledTimes(1);
        expect(HTMLElement.prototype.scrollIntoView).toHaveBeenCalledWith({
            behavior: "smooth",
            block: "start",
        });
    });

    it("does not throw when an in-page anchor link targets a non-existent heading", async () => {
        const consoleWarnSpy = vi.spyOn(console, "warn").mockImplementation(() => undefined);
        try {
            await act(async () => {
                root.render(<MarkdownPreview content={"[Missing](#non-existent)"}/>);
            });

            const link = container.querySelector<HTMLAnchorElement>("a");
            expect(link).not.toBeNull();

            await expect(act(async () => {
                link!.click();
            })).resolves.not.toThrow();

            expect(HTMLElement.prototype.scrollIntoView).not.toHaveBeenCalled();
            expect(consoleWarnSpy).toHaveBeenCalledWith("[markdown] heading target not found", "non-existent");
        } finally {
            consoleWarnSpy.mockRestore();
        }
    });

    it("does not render an outline when content has no headings", async () => {
        await act(async () => {
            root.render(<MarkdownPreview content={"Plain text with **no headings**."}/>);
        });

        expect(container.querySelector(".md-preview-outline")).toBeNull();
        expect(container.querySelector(".md-preview-layout")).not.toBeNull();
    });

    it("collapses the outline into a right-side toggle", async () => {
        await act(async () => {
            root.render(<MarkdownPreview content={"# Intro\n\n## Details"}/>);
        });

        const collapseButton = container.querySelector<HTMLButtonElement>(".md-preview-outline-toggle");
        expect(collapseButton).not.toBeNull();
        expect(container.querySelector(".md-preview-outline-panel")).not.toBeNull();

        await act(async () => {
            collapseButton!.click();
        });

        expect(container.querySelector(".md-preview-outline-panel")).toBeNull();
        const expandButton = container.querySelector<HTMLButtonElement>(".md-preview-outline-collapsed-toggle");
        expect(expandButton).not.toBeNull();

        await act(async () => {
            expandButton!.click();
        });

        expect(container.querySelector(".md-preview-outline-panel")).not.toBeNull();
        expect(container.querySelector(".md-preview-outline-collapsed-toggle")).toBeNull();
    });

    it("resets the outline collapse state when content changes", async () => {
        await act(async () => {
            root.render(<MarkdownPreview content={"# First File"}/>);
        });

        const collapseButton = container.querySelector<HTMLButtonElement>(".md-preview-outline-toggle");
        expect(collapseButton).not.toBeNull();

        await act(async () => {
            collapseButton!.click();
        });

        expect(container.querySelector(".md-preview-outline-panel")).toBeNull();

        await act(async () => {
            root.render(<MarkdownPreview content={"# Second File"}/>);
        });

        expect(container.querySelector(".md-preview-outline-panel")).not.toBeNull();
    });
});
