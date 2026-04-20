import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {GraphvizRenderer} from "./GraphvizRenderer";

const dotMock = vi.fn<(source: string) => string>();
const loadMock = vi.fn(() => Promise.resolve({dot: dotMock}));

vi.mock("@hpcc-js/wasm/graphviz", () => ({
    Graphviz: {
        load: loadMock,
    },
}));

async function flushRender(): Promise<void> {
    await act(async () => {
        await new Promise((resolve) => setTimeout(resolve, 0));
        await Promise.resolve();
    });
}

describe("GraphvizRenderer", () => {
    let container: HTMLDivElement;
    let root: Root;
    let consoleWarnSpy: ReturnType<typeof vi.spyOn>;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
        dotMock.mockReset();
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

    it("renders the svg output returned by graphviz.dot", async () => {
        dotMock.mockReturnValue("<svg xmlns=\"http://www.w3.org/2000/svg\"><g>ok</g></svg>");

        await act(async () => {
            root.render(<GraphvizRenderer code="digraph { a -> b }"/>);
        });
        await flushRender();

        expect(dotMock).toHaveBeenCalledWith("digraph { a -> b }");
        expect(container.querySelector("svg")).not.toBeNull();
    });

    it("shows an inline error when graphviz throws", async () => {
        dotMock.mockImplementation(() => {
            throw new Error("parse failure");
        });

        await act(async () => {
            root.render(<GraphvizRenderer code="bad source"/>);
        });
        await flushRender();

        expect(container.textContent).toContain("parse failure");
        expect(consoleWarnSpy).toHaveBeenCalledWith("[graphviz] failed to render diagram", expect.any(Error));
    });

    it("reports invalid svg output instead of injecting parse error markup", async () => {
        dotMock.mockReturnValue("<not-svg>oops</not-svg>");

        await act(async () => {
            root.render(<GraphvizRenderer code="digraph {}"/>);
        });
        await flushRender();

        expect(container.querySelector("svg")).toBeNull();
        expect(container.textContent).toContain("invalid SVG");
    });
});
