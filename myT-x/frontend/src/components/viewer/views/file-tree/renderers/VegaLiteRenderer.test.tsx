import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {VegaLiteRenderer} from "./VegaLiteRenderer";

const finalizeMock = vi.fn();
const embedMock = vi.fn<(el: Element, spec: object, opts: object) => Promise<{finalize: () => void}>>();

vi.mock("vega-embed", () => ({
    default: embedMock,
}));

async function flushRender(): Promise<void> {
    for (let i = 0; i < 10; i++) {
        await act(async () => {
            await new Promise((resolve) => setTimeout(resolve, 0));
        });
    }
}

describe("VegaLiteRenderer", () => {
    let container: HTMLDivElement;
    let root: Root;
    let consoleWarnSpy: ReturnType<typeof vi.spyOn>;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
        embedMock.mockReset();
        finalizeMock.mockReset();
        embedMock.mockResolvedValue({finalize: finalizeMock});
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

    it("invokes vega-embed with vega-lite mode", async () => {
        await act(async () => {
            root.render(<VegaLiteRenderer code='{"mark":"bar"}' kind="vega-lite"/>);
        });
        await flushRender();

        expect(embedMock).toHaveBeenCalledTimes(1);
        const optsArg = embedMock.mock.calls[0]?.[2] as {
            mode?: string;
            actions?: {
                export?: {png?: boolean; svg?: boolean};
                source?: boolean;
                compiled?: boolean;
                editor?: boolean;
            };
        };
        expect(optsArg.mode).toBe("vega-lite");
        expect(optsArg.actions).toEqual({
            export: {
                png: true,
                svg: true,
            },
            source: true,
            compiled: true,
            editor: true,
        });
        expect((optsArg as {i18n?: Record<string, string>}).i18n).toEqual(expect.objectContaining({
            CLICK_TO_VIEW_ACTIONS: "操作",
            PNG_ACTION: "PNG を書き出し",
            SVG_ACTION: "SVG を書き出し",
            SOURCE_ACTION: "ソースを表示",
            COMPILED_ACTION: "コンパイル結果を表示",
            EDITOR_ACTION: "Vega Editor で開く",
        }));
    });

    it("invokes vega-embed with vega mode", async () => {
        await act(async () => {
            root.render(<VegaLiteRenderer code='{"signals":[]}' kind="vega"/>);
        });
        await flushRender();

        const optsArg = embedMock.mock.calls[0]?.[2] as {mode?: string};
        expect(optsArg.mode).toBe("vega");
    });

    it("shows an inline error when the spec is invalid JSON", async () => {
        await act(async () => {
            root.render(<VegaLiteRenderer code="not json" kind="vega-lite"/>);
        });
        await flushRender();

        expect(consoleWarnSpy).toHaveBeenCalledWith("[vega] failed to render chart", expect.any(Error));
        expect(container.querySelector(".file-content-empty")?.textContent).not.toBe("");
    });

    it("finalizes the embed on unmount", async () => {
        await act(async () => {
            root.render(<VegaLiteRenderer code='{"mark":"bar"}' kind="vega-lite"/>);
        });
        await flushRender();

        act(() => {
            root.unmount();
        });

        expect(finalizeMock).toHaveBeenCalled();
    });
});
