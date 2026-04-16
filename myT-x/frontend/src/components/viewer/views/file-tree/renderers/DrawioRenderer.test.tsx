import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {DrawioRenderer} from "./DrawioRenderer";

const {devPanelReadBinaryMock} = vi.hoisted(() => ({
    devPanelReadBinaryMock: vi.fn(),
}));

vi.mock("../../../../../api", () => ({
    api: {
        DevPanelReadBinary: devPanelReadBinaryMock,
    },
}));

vi.mock("../../../../../hooks/useShikiHighlight", () => ({
    useShikiHighlight: () => ({
        tokens: null,
        skipInfo: null,
        isHighlightFailed: false,
    }),
}));

describe("DrawioRenderer", () => {
    let container: HTMLDivElement;
    let root: Root;
    let consoleWarnSpy: ReturnType<typeof vi.spyOn>;
    let originalCreateObjectURL: typeof URL.createObjectURL;
    let originalRevokeObjectURL: typeof URL.revokeObjectURL;
    let createObjectURLMock: ReturnType<typeof vi.fn>;
    let revokeObjectURLMock: ReturnType<typeof vi.fn>;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
        devPanelReadBinaryMock.mockReset();
        consoleWarnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});

        originalCreateObjectURL = URL.createObjectURL;
        originalRevokeObjectURL = URL.revokeObjectURL;
        createObjectURLMock = vi.fn(() => "blob:drawio-preview");
        revokeObjectURLMock = vi.fn();
        URL.createObjectURL = createObjectURLMock as unknown as typeof URL.createObjectURL;
        URL.revokeObjectURL = revokeObjectURLMock as unknown as typeof URL.revokeObjectURL;
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        container.remove();
        consoleWarnSpy.mockRestore();
        URL.createObjectURL = originalCreateObjectURL;
        URL.revokeObjectURL = originalRevokeObjectURL;
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = false;
    });

    async function flushRenderer(): Promise<void> {
        await act(async () => {
            await Promise.resolve();
            await new Promise((resolve) => setTimeout(resolve, 0));
        });
    }

    it("renders draw.io XML content in a highlighted code block", async () => {
        await act(async () => {
            root.render(
                <DrawioRenderer
                    kind="drawio-xml"
                    content={"<mxfile><diagram id=\"1\">hello</diagram></mxfile>"}
                    filePath="docs/arch.drawio.xml"
                />,
            );
        });

        const codeBlock = container.querySelector(".file-view-drawio-code code.language-xml");
        expect(codeBlock?.textContent).toContain("<mxfile><diagram id=\"1\">hello</diagram></mxfile>");
        expect(devPanelReadBinaryMock).not.toHaveBeenCalled();
    });

    it("loads a draw.io SVG preview via DevPanelReadBinary and renders the blob image", async () => {
        devPanelReadBinaryMock.mockResolvedValue({
            path: "docs/arch.drawio.svg",
            data: btoa("<svg xmlns=\"http://www.w3.org/2000/svg\"><text>diagram</text></svg>"),
            mime: "image/svg+xml",
        });

        await act(async () => {
            root.render(
                <DrawioRenderer
                    kind="drawio-svg"
                    content={"<svg xmlns=\"http://www.w3.org/2000/svg\"><text>diagram</text></svg>"}
                    filePath="docs/arch.drawio.svg"
                    sessionKey="session-1"
                    sessionName="test-session"
                />,
            );
        });
        await flushRenderer();

        expect(devPanelReadBinaryMock).toHaveBeenCalledWith("test-session", "docs/arch.drawio.svg");
        expect(createObjectURLMock).toHaveBeenCalledTimes(1);
        const image = container.querySelector(".file-view-drawio-image");
        expect(image?.getAttribute("src")).toBe("blob:drawio-preview");
        expect(image?.getAttribute("alt")).toBe("draw.io diagram preview");
    });

    it("shows an inline error when the draw.io SVG preview cannot be loaded", async () => {
        const loadError = new Error("read failed");
        devPanelReadBinaryMock.mockRejectedValue(loadError);

        await act(async () => {
            root.render(
                <DrawioRenderer
                    kind="drawio-svg"
                    content={"<svg xmlns=\"http://www.w3.org/2000/svg\"><text>diagram</text></svg>"}
                    filePath="docs/arch.drawio.svg"
                    sessionKey="session-1"
                    sessionName="test-session"
                />,
            );
        });
        await flushRenderer();

        expect(container.textContent).toContain("read failed");
        expect(consoleWarnSpy).toHaveBeenCalledWith("[drawio] failed to load svg preview", expect.objectContaining({
            path: "docs/arch.drawio.svg",
            session: "test-session",
            err: loadError,
        }));
    });
});
