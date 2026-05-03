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

    function delayRenderTick(): Promise<void> {
        return new Promise((resolve) => setTimeout(resolve, 0));
    }

    async function waitForRenderer(expectation: () => void): Promise<void> {
        const deadline = Date.now() + 1000;
        let lastError: unknown = null;
        while (Date.now() < deadline) {
            try {
                expectation();
                return;
            } catch (err: unknown) {
                lastError = err;
                await act(async () => {
                    await delayRenderTick();
                });
            }
        }
        throw lastError instanceof Error ? lastError : new Error("Timed out waiting for draw.io renderer update.");
    }

    it("renders uncompressed draw.io XML content as an inline diagram preview", async () => {
        await act(async () => {
            root.render(
                <DrawioRenderer
                    kind="drawio-xml"
                    content={`
                        <mxfile>
                            <diagram id="1">
                                <mxGraphModel>
                                    <root>
                                        <mxCell id="0"/>
                                        <mxCell id="1" parent="0"/>
                                        <mxCell id="2" value="Start" style="rounded=1;whiteSpace=wrap;html=1;" vertex="1" parent="1">
                                            <mxGeometry x="40" y="60" width="120" height="60" as="geometry"/>
                                        </mxCell>
                                        <mxCell id="3" value="End" style="ellipse;whiteSpace=wrap;html=1;" vertex="1" parent="1">
                                            <mxGeometry x="240" y="60" width="120" height="60" as="geometry"/>
                                        </mxCell>
                                        <mxCell id="4" edge="1" source="2" target="3" parent="1"/>
                                    </root>
                                </mxGraphModel>
                            </diagram>
                        </mxfile>
                    `}
                    filePath="docs/arch.drawio.xml"
                />,
            );
        });
        await waitForRenderer(() => {
            expect(container.querySelector(".file-view-drawio-svg")).not.toBeNull();
        });

        const svg = container.querySelector(".file-view-drawio-svg");
        expect(svg).not.toBeNull();
        expect(svg?.textContent).toContain("Start");
        expect(svg?.textContent).toContain("End");
        expect(container.querySelector(".file-view-drawio-code")).toBeNull();
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
        await waitForRenderer(() => {
            expect(container.querySelector(".file-view-drawio-image")).not.toBeNull();
        });

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
        await waitForRenderer(() => {
            expect(container.textContent).toContain("read failed");
        });

        expect(container.textContent).toContain("read failed");
        expect(consoleWarnSpy).toHaveBeenCalledWith("[drawio] failed to load svg preview", expect.objectContaining({
            path: "docs/arch.drawio.svg",
            session: "test-session",
            err: loadError,
        }));
    });
});
