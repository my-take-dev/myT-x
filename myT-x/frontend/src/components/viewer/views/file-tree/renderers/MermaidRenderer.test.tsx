import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {MermaidRenderer} from "./MermaidRenderer";

const initializeMock = vi.fn();
const renderMock = vi.fn<(id: string, code: string) => Promise<{svg: string; bindFunctions?: (element: Element) => void}>>();

vi.mock("mermaid", () => ({
    default: {
        initialize: initializeMock,
        render: renderMock,
    },
}));

describe("MermaidRenderer", () => {
    let container: HTMLDivElement;
    let root: Root;
    let consoleWarnSpy: ReturnType<typeof vi.spyOn>;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
        initializeMock.mockReset();
        renderMock.mockReset();
        consoleWarnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        container.remove();
        consoleWarnSpy.mockRestore();
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = false;
    });

    async function flushMermaidRender(): Promise<void> {
        await act(async () => {
            await new Promise((resolve) => setTimeout(resolve, 0));
            await Promise.resolve();
        });
    }

    it("renders svg output returned by mermaid", async () => {
        renderMock.mockResolvedValue({
            svg: "<svg><text>diagram</text></svg>",
        });

        await act(async () => {
            root.render(<MermaidRenderer code="graph TD; A-->B;"/>);
        });
        await flushMermaidRender();

        expect(initializeMock).toHaveBeenCalledWith(expect.objectContaining({
            htmlLabels: true,
            markdownAutoWrap: true,
            startOnLoad: false,
        }));
        expect(renderMock).toHaveBeenCalledTimes(1);
        expect(renderMock.mock.calls[0]?.[1]).toBe("graph TD; A-->B;");
        expect(container.querySelector("svg")?.textContent).toContain("diagram");
    });

    it("shows an inline error when mermaid render fails", async () => {
        renderMock.mockRejectedValue(new Error("parse failed"));

        await act(async () => {
            root.render(<MermaidRenderer code="graph TD;"/>);
        });
        await flushMermaidRender();

        expect(container.textContent).toContain("parse failed");
        expect(consoleWarnSpy).toHaveBeenCalledWith("[mermaid] failed to render diagram", expect.any(Error));
    });
});
