import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {WavedromRenderer} from "./WavedromRenderer";

const renderWaveElementMock = vi.fn<
    (index: number, source: object, outputElement: Element, waveSkin: Record<string, unknown>) => void
>();
const waveSkinValue: Record<string, unknown> = {};

vi.mock("wavedrom", () => ({
    default: {
        renderWaveElement: renderWaveElementMock,
        waveSkin: waveSkinValue,
    },
}));

vi.mock("json5", () => ({
    default: {
        parse: (s: string) => JSON.parse(s) as unknown,
    },
}));

async function flushRender(): Promise<void> {
    for (let i = 0; i < 10; i++) {
        await act(async () => {
            await new Promise((resolve) => setTimeout(resolve, 0));
        });
    }
}

describe("WavedromRenderer", () => {
    let container: HTMLDivElement;
    let root: Root;
    let consoleWarnSpy: ReturnType<typeof vi.spyOn>;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
        renderWaveElementMock.mockReset();
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

    it("parses JSON source and renders through renderWaveElement", async () => {
        renderWaveElementMock.mockImplementation((_index, _source, outputElement) => {
            const svgNs = "http://www.w3.org/2000/svg";
            const svgEl = document.createElementNS(svgNs, "svg");
            outputElement.replaceChildren(svgEl);
        });

        await act(async () => {
            root.render(<WavedromRenderer code='{"signal":[{"name":"clk","wave":"p."}]}'/>);
        });
        await flushRender();

        expect(renderWaveElementMock).toHaveBeenCalledTimes(1);
        expect(container.querySelector("svg")).not.toBeNull();
        const indexArg = renderWaveElementMock.mock.calls[0]?.[0];
        expect(indexArg).toEqual(expect.any(Number));
        expect(indexArg).toBeGreaterThan(0);
        const sourceArg = renderWaveElementMock.mock.calls[0]?.[1] as {signal?: unknown[]};
        expect(Array.isArray(sourceArg.signal)).toBe(true);
    });

    it("shows an inline error when the source is not JSON", async () => {
        await act(async () => {
            root.render(<WavedromRenderer code="not-json-source"/>);
        });
        await flushRender();

        expect(consoleWarnSpy).toHaveBeenCalledWith("[wavedrom] failed to render diagram", expect.any(Error));
        expect(container.querySelector(".file-content-empty")?.textContent).not.toBe("");
    });

    it("shows an inline error when renderWaveElement throws", async () => {
        renderWaveElementMock.mockImplementation(() => {
            throw new Error("bad spec");
        });

        await act(async () => {
            root.render(<WavedromRenderer code='{"signal":[]}'/>);
        });
        await flushRender();

        expect(container.textContent).toContain("bad spec");
    });
});
