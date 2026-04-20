import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {MarkmapRenderer} from "./MarkmapRenderer";

const transformMock = vi.fn<(source: string) => {root: unknown}>();
const destroyMock = vi.fn();
const setDataMock = vi.fn<(data: unknown) => Promise<void>>();
const fitMock = vi.fn(() => Promise.resolve());
const createMock = vi.fn<
    (svg: SVGElement, opts?: unknown) => {
        destroy: typeof destroyMock;
        fit: typeof fitMock;
        setData: typeof setDataMock;
    }
>();

class MockTransformer {
    transform = transformMock;
}

vi.mock("markmap-lib", () => ({
    Transformer: MockTransformer,
}));

vi.mock("markmap-view", () => ({
    Markmap: {
        create: createMock,
    },
}));

async function flushRender(): Promise<void> {
    for (let i = 0; i < 10; i++) {
        await act(async () => {
            await new Promise((resolve) => setTimeout(resolve, 0));
        });
    }
}

describe("MarkmapRenderer", () => {
    let container: HTMLDivElement;
    let root: Root;
    let consoleWarnSpy: ReturnType<typeof vi.spyOn>;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
        transformMock.mockReset();
        createMock.mockReset();
        destroyMock.mockReset();
        setDataMock.mockReset();
        fitMock.mockClear();
        setDataMock.mockResolvedValue();
        createMock.mockImplementation(() => ({fit: fitMock, destroy: destroyMock, setData: setDataMock}));
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

    it("creates a Markmap instance with transformed root", async () => {
        transformMock.mockReturnValue({root: {type: "node"}});

        await act(async () => {
            root.render(<MarkmapRenderer code="# root"/>);
        });
        await flushRender();

        expect(transformMock).toHaveBeenCalledWith("# root");
        expect(createMock).toHaveBeenCalledTimes(1);
        const svgArg = createMock.mock.calls[0]?.[0];
        expect(svgArg).toBeInstanceOf(SVGElement);
        expect(createMock.mock.calls[0]?.[1]).toMatchObject({
            autoFit: true,
            initialExpandLevel: 2,
            maxWidth: 240,
        });
        expect(setDataMock).toHaveBeenCalledWith({type: "node"});
        expect(fitMock).toHaveBeenCalled();
    });

    it("waits for setData before marking the diagram ready", async () => {
        let resolveSetData: (() => void) | null = null;
        transformMock.mockReturnValue({root: {type: "node"}});
        setDataMock.mockImplementation(() => new Promise<void>((resolve) => {
            resolveSetData = resolve;
        }));

        await act(async () => {
            root.render(<MarkmapRenderer code="# root"/>);
        });
        await flushRender();

        expect(container.textContent).toContain("Rendering Markmap diagram...");
        expect(fitMock).not.toHaveBeenCalled();

        await act(async () => {
            resolveSetData?.();
            await Promise.resolve();
        });
        await flushRender();

        expect(fitMock).toHaveBeenCalledTimes(1);
        expect(container.textContent).not.toContain("Rendering Markmap diagram...");
    });

    it("shows an inline error when transform throws", async () => {
        transformMock.mockImplementation(() => {
            throw new Error("bad markdown");
        });

        await act(async () => {
            root.render(<MarkmapRenderer code=""/>);
        });
        await flushRender();

        expect(container.textContent).toContain("bad markdown");
        expect(consoleWarnSpy).toHaveBeenCalledWith("[markmap] failed to render diagram", expect.any(Error));
    });

    it("destroys the Markmap instance on unmount after a successful render", async () => {
        transformMock.mockReturnValue({root: {type: "node"}});

        await act(async () => {
            root.render(<MarkmapRenderer code="# root"/>);
        });
        await flushRender();
        expect(destroyMock).not.toHaveBeenCalled();

        act(() => {
            root.unmount();
        });

        expect(destroyMock).toHaveBeenCalledTimes(1);
    });
});
