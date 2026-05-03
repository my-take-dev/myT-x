import {deflateRawSync} from "node:zlib";
import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";

const apiMock = vi.hoisted(() => ({
    DevPanelReadBinary: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
}));

vi.mock("../src/api", () => ({
    api: {
        DevPanelReadBinary: (...args: unknown[]) => apiMock.DevPanelReadBinary(...args),
    },
}));

import {DrawioRenderer} from "../src/components/viewer/views/file-tree/renderers/DrawioRenderer";

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

function compressedDrawioContent(modelXml: string, pageCount = 1): string {
    const encoded = deflateRawSync(Buffer.from(encodeURIComponent(modelXml), "utf8")).toString("base64");
    // Fold to exercise whitespace stripping in compressed draw.io payloads.
    const foldedEncoded = `${encoded.slice(0, 18)}\n ${encoded.slice(18, 36)}\n${encoded.slice(36)}`;
    const diagrams = Array.from({length: pageCount}, (_, index) => (
        `<diagram id="page-${index + 1}">${foldedEncoded}</diagram>`
    )).join("");
    return `<mxfile>${diagrams}</mxfile>`;
}

function rawCompressedDrawioContent(modelXml: string): string {
    const encoded = deflateRawSync(Buffer.from(modelXml, "utf8")).toString("base64");
    return `<mxfile><diagram id="raw-page">${encoded}</diagram></mxfile>`;
}

function installInflatedTextStream(inflatedText: string, delayMs = 0): () => void {
    const target = globalThis as typeof globalThis & { DecompressionStream?: typeof DecompressionStream };
    const hadDecompressionStream = "DecompressionStream" in target;
    const originalDecompressionStream = target.DecompressionStream;

    class MockDecompressionStream {
        readonly readable: ReadableStream<Uint8Array>;
        readonly writable: WritableStream<Uint8Array>;

        constructor() {
            const encoder = new TextEncoder();
            let emitted = false;
            const transform = new TransformStream<Uint8Array, Uint8Array>({
                async transform(_chunk, controller) {
                    if (emitted) {
                        return;
                    }
                    emitted = true;
                    if (delayMs > 0) {
                        await new Promise((resolve) => setTimeout(resolve, delayMs));
                    }
                    controller.enqueue(encoder.encode(inflatedText));
                },
            });
            this.readable = transform.readable;
            this.writable = transform.writable;
        }
    }

    Object.defineProperty(target, "DecompressionStream", {
        configurable: true,
        writable: true,
        value: MockDecompressionStream as unknown as typeof DecompressionStream,
    });

    return () => {
        if (hadDecompressionStream) {
            Object.defineProperty(target, "DecompressionStream", {
                configurable: true,
                writable: true,
                value: originalDecompressionStream,
            });
            return;
        }
        Reflect.deleteProperty(target, "DecompressionStream");
    };
}

describe("DrawioRenderer", () => {
    let container: HTMLDivElement;
    let root: Root;
    let createObjectURLSpy: ReturnType<typeof vi.spyOn>;
    let revokeObjectURLSpy: ReturnType<typeof vi.spyOn>;
    let consoleWarnSpy: ReturnType<typeof vi.spyOn>;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        apiMock.DevPanelReadBinary.mockReset();
        createObjectURLSpy = vi.spyOn(URL, "createObjectURL")
            .mockReturnValueOnce("blob:drawio-preview-a")
            .mockReturnValueOnce("blob:drawio-preview-b");
        revokeObjectURLSpy = vi.spyOn(URL, "revokeObjectURL").mockImplementation(() => {});
        consoleWarnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        container.remove();
        createObjectURLSpy.mockRestore();
        revokeObjectURLSpy.mockRestore();
        consoleWarnSpy.mockRestore();
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = false;
    });

    it("reloads the same draw.io svg path when content changes", async () => {
        apiMock.DevPanelReadBinary
            .mockResolvedValueOnce({
                path: "docs/diagram.drawio.svg",
                data: "PHN2Zy8+",
                mime: "image/svg+xml",
            })
            .mockResolvedValueOnce({
                path: "docs/diagram.drawio.svg",
                data: "PHN2ZyBjbGFzcz0idXBkYXRlZCIvPg==",
                mime: "image/svg+xml",
            });

        act(() => {
            root.render(
                <DrawioRenderer
                    kind="drawio-svg"
                    content="<svg/>"
                    filePath="docs/diagram.drawio.svg"
                    sessionKey="session-a:1"
                    sessionName="session-a"
                />,
            );
        });
        await waitForRenderer(() => {
            expect(container.querySelector("img")?.getAttribute("src")).toBe("blob:drawio-preview-a");
        });

        act(() => {
            root.render(
                <DrawioRenderer
                    kind="drawio-svg"
                    content={'<svg class="updated"/>'}
                    filePath="docs/diagram.drawio.svg"
                    sessionKey="session-a:1"
                    sessionName="session-a"
                />,
            );
        });
        await waitForRenderer(() => {
            expect(container.querySelector("img")?.getAttribute("src")).toBe("blob:drawio-preview-b");
        });

        expect(apiMock.DevPanelReadBinary).toHaveBeenCalledTimes(2);
        expect(apiMock.DevPanelReadBinary).toHaveBeenNthCalledWith(1, "session-a", "docs/diagram.drawio.svg");
        expect(apiMock.DevPanelReadBinary).toHaveBeenNthCalledWith(2, "session-a", "docs/diagram.drawio.svg");
        expect(container.querySelector("img")?.getAttribute("src")).toBe("blob:drawio-preview-b");
        expect(revokeObjectURLSpy).toHaveBeenCalledWith("blob:drawio-preview-a");
    });

    it("clears stale load errors when same-path content changes", async () => {
        apiMock.DevPanelReadBinary
            .mockRejectedValueOnce(new Error("temporary read failure"))
            .mockResolvedValueOnce({
                path: "docs/diagram.drawio.svg",
                data: "PHN2Zy8+",
                mime: "image/svg+xml",
            });

        act(() => {
            root.render(
                <DrawioRenderer
                    kind="drawio-svg"
                    content="<svg/>"
                    filePath="docs/diagram.drawio.svg"
                    sessionKey="session-a:1"
                    sessionName="session-a"
                />,
            );
        });
        await waitForRenderer(() => {
            expect(container.textContent).toContain("temporary read failure");
        });

        act(() => {
            root.render(
                <DrawioRenderer
                    kind="drawio-svg"
                    content={'<svg class="retry"/>'}
                    filePath="docs/diagram.drawio.svg"
                    sessionKey="session-a:1"
                    sessionName="session-a"
                />,
            );
        });
        await waitForRenderer(() => {
            expect(container.querySelector("img")?.getAttribute("src")).toBe("blob:drawio-preview-a");
        });

        expect(apiMock.DevPanelReadBinary).toHaveBeenCalledTimes(2);
        expect(container.querySelector("img")?.getAttribute("src")).toBe("blob:drawio-preview-a");
        expect(container.textContent).not.toContain("temporary read failure");
    });

    it("renders draw.io xml files as inline svg previews", async () => {
        act(() => {
            root.render(
                <DrawioRenderer
                    kind="drawio-xml"
                    content={`
                        <mxGraphModel>
                            <root>
                                <mxCell id="0"/>
                                <mxCell id="1" parent="0"/>
                                <mxCell id="2" value="Decision" style="rhombus;whiteSpace=wrap;html=1;" vertex="1" parent="1">
                                    <mxGeometry x="120" y="80" width="120" height="80" as="geometry"/>
                                </mxCell>
                            </root>
                        </mxGraphModel>
                    `}
                    filePath="docs/diagram.drawio"
                />,
            );
        });
        await waitForRenderer(() => {
            expect(container.querySelector(".file-view-drawio-svg")).not.toBeNull();
        });

        expect(container.querySelector(".file-view-drawio-svg")).not.toBeNull();
        expect(container.querySelector("polygon")).not.toBeNull();
        expect(container.textContent).toContain("Decision");
        expect(container.querySelector(".file-view-drawio-code")).toBeNull();
    });

    it("renders compressed draw.io XML with folded base64 as an inline svg preview", async () => {
        const modelXml = `
            <mxGraphModel>
                <root>
                    <mxCell id="0"/>
                    <mxCell id="1" parent="0"/>
                    <mxCell id="2" value="Start&lt;br /&gt;Next" style="rounded=1;fillColor=default;strokeColor=default;" vertex="1" parent="1">
                        <mxGeometry x="120" y="80" width="120" height="60" as="geometry"/>
                    </mxCell>
                    <mxCell id="3" value="End" style="ellipse;fillColor=#dae8fc;strokeColor=#6c8ebf;" vertex="1" parent="1">
                        <mxGeometry x="300" y="80" width="60" height="60" as="geometry"/>
                    </mxCell>
                    <mxCell id="4" value="Flow" style="strokeColor=#ff0000;" edge="1" source="2" target="3" parent="1"/>
                </root>
            </mxGraphModel>
        `;

        act(() => {
            root.render(
                <DrawioRenderer
                    kind="drawio-xml"
                    content={compressedDrawioContent(modelXml, 2)}
                    filePath="docs/compressed.drawio"
                />,
            );
        });
        await waitForRenderer(() => {
            expect(container.querySelector(".file-view-drawio-svg")).not.toBeNull();
        });

        const svg = container.querySelector(".file-view-drawio-svg");
        expect(svg).not.toBeNull();
        expect(svg?.getAttribute("viewBox")).toBe("72 32 336 156");
        expect(svg?.getAttribute("aria-label")).toBe("draw.io diagram preview: docs/compressed.drawio");
        expect(container.textContent).toContain("Showing page 1 of 2.");
        expect(container.textContent).toContain("Start");
        expect(container.textContent).toContain("Next");
        expect(container.querySelector("rect")?.getAttribute("fill")).toBe("var(--bg-panel)");
        expect(container.querySelector("rect")?.getAttribute("stroke")).toBe("var(--line)");
        expect(container.querySelector("line")?.getAttribute("stroke")).toBe("#ff0000");
        expect(container.querySelector("line")?.getAttribute("marker-end")).toMatch(/^url\(#drawio-arrowhead-[A-Za-z0-9_-]+-0\)$/);
        expect(container.querySelector("marker path")?.getAttribute("fill")).toBe("#ff0000");
        expect(container.querySelector(".file-view-drawio-code")).toBeNull();
    });

    it("keeps html div labels as separate svg text lines", async () => {
        act(() => {
            root.render(
                <DrawioRenderer
                    kind="drawio-xml"
                    content={`
                        <mxGraphModel>
                            <root>
                                <mxCell id="0"/>
                                <mxCell id="1" parent="0"/>
                                <mxCell id="2" value="&lt;div&gt;Line 1&lt;/div&gt;&lt;div&gt;Line 2&lt;/div&gt;" style="whiteSpace=wrap;html=1;" vertex="1" parent="1">
                                    <mxGeometry x="10" y="20" width="120" height="60" as="geometry"/>
                                </mxCell>
                            </root>
                        </mxGraphModel>
                    `}
                    filePath="docs/div-label.drawio"
                />,
            );
        });
        await waitForRenderer(() => {
            expect(container.querySelector(".file-view-drawio-svg")).not.toBeNull();
        });

        const labelLines = Array.from(container.querySelectorAll(".file-view-drawio-label tspan"))
            .map((line) => line.textContent);
        expect(labelLines).toEqual(["Line 1", "Line 2"]);
    });

    it("uses unique svg marker ids for co-located draw.io previews", async () => {
        const content = `
            <mxGraphModel>
                <root>
                    <mxCell id="0"/>
                    <mxCell id="1" parent="0"/>
                    <mxCell id="2" value="A" vertex="1" parent="1">
                        <mxGeometry x="0" y="0" width="40" height="40" as="geometry"/>
                    </mxCell>
                    <mxCell id="3" value="B" vertex="1" parent="1">
                        <mxGeometry x="100" y="0" width="40" height="40" as="geometry"/>
                    </mxCell>
                    <mxCell id="4" edge="1" source="2" target="3" parent="1"/>
                </root>
            </mxGraphModel>
        `;

        act(() => {
            root.render(
                <div>
                    <DrawioRenderer kind="drawio-xml" content={content} filePath="docs/a.drawio"/>
                    <DrawioRenderer kind="drawio-xml" content={content} filePath="docs/b.drawio"/>
                </div>,
            );
        });
        await waitForRenderer(() => {
            expect(container.querySelectorAll(".file-view-drawio-svg")).toHaveLength(2);
        });

        const markerIds = Array.from(container.querySelectorAll("marker")).map((marker) => marker.id);
        const markerReferences = Array.from(container.querySelectorAll("line"))
            .map((line) => line.getAttribute("marker-end"));
        expect(new Set(markerIds).size).toBe(2);
        expect(markerReferences).toEqual(markerIds.map((id) => `url(#${id})`));
    });

    it("reports the displayed page index when earlier draw.io pages are empty", async () => {
        act(() => {
            root.render(
                <DrawioRenderer
                    kind="drawio-xml"
                    content={`
                        <mxfile>
                            <diagram id="empty"></diagram>
                            <diagram id="page-2">
                                <mxGraphModel>
                                    <root>
                                        <mxCell id="0"/>
                                        <mxCell id="1" parent="0"/>
                                        <mxCell id="2" value="Second page" vertex="1" parent="1">
                                            <mxGeometry x="10" y="20" width="120" height="60" as="geometry"/>
                                        </mxCell>
                                    </root>
                                </mxGraphModel>
                            </diagram>
                        </mxfile>
                    `}
                    filePath="docs/multipage.drawio"
                />,
            );
        });
        await waitForRenderer(() => {
            expect(container.querySelector(".file-view-drawio-svg")).not.toBeNull();
        });

        expect(container.textContent).toContain("Showing page 2 of 2.");
        expect(container.textContent).toContain("Second page");
    });

    it("renders fixed-point edges from compressed raw XML labels containing percent signs", async () => {
        const modelXml = `
            <mxGraphModel>
                <root>
                    <mxCell id="0"/>
                    <mxCell id="1" parent="0"/>
                    <mxCell id="2" value="99%" vertex="1" parent="1">
                        <mxGeometry x="0" y="0" width="20" height="20" as="geometry"/>
                    </mxCell>
                    <mxCell id="3" value="99%" edge="1" parent="1">
                        <mxGeometry relative="1" as="geometry">
                            <mxPoint x="40" y="20" as="sourcePoint"/>
                            <mxPoint x="180" y="90" as="targetPoint"/>
                        </mxGeometry>
                    </mxCell>
                </root>
            </mxGraphModel>
        `;

        act(() => {
            root.render(
                <DrawioRenderer
                    kind="drawio-xml"
                    content={rawCompressedDrawioContent(modelXml)}
                    filePath="docs/fixed-edge.drawio"
                />,
            );
        });
        await waitForRenderer(() => {
            expect(container.querySelector(".file-view-drawio-svg")).not.toBeNull();
        });

        const line = container.querySelector("line");
        expect(line?.getAttribute("x1")).toBe("40");
        expect(line?.getAttribute("y1")).toBe("20");
        expect(line?.getAttribute("x2")).toBe("180");
        expect(line?.getAttribute("y2")).toBe("90");
        expect(container.textContent).toContain("99%");
        expect(container.querySelector(".file-view-drawio-code")).toBeNull();
    });

    it("falls back to source XML when compressed preview support is unavailable", async () => {
        const target = globalThis as typeof globalThis & { DecompressionStream?: typeof DecompressionStream };
        const hadDecompressionStream = "DecompressionStream" in target;
        const originalDecompressionStream = target.DecompressionStream;
        Reflect.deleteProperty(target, "DecompressionStream");

        try {
            act(() => {
                root.render(
                    <DrawioRenderer
                        kind="drawio-xml"
                        content={compressedDrawioContent(`
                            <mxGraphModel>
                                <root>
                                    <mxCell id="0"/>
                                    <mxCell id="1" parent="0"/>
                                    <mxCell id="2" value="Start" vertex="1" parent="1">
                                        <mxGeometry x="0" y="0" width="120" height="60" as="geometry"/>
                                    </mxCell>
                                </root>
                            </mxGraphModel>
                        `)}
                        filePath="docs/unsupported.drawio"
                    />,
                );
            });
            await waitForRenderer(() => {
                expect(container.querySelector(".file-view-drawio-code")).not.toBeNull();
            });

            expect(container.querySelector(".file-view-drawio-svg")).toBeNull();
            expect(container.querySelector(".file-view-drawio-code")).not.toBeNull();
            expect(container.textContent).toContain("Unable to decode this compressed draw.io diagram");
            expect(consoleWarnSpy).toHaveBeenCalledWith("[drawio] failed to render xml preview", expect.objectContaining({
                phase: "decompress",
                reason: "compressed-preview-decode-failed",
                filePath: "docs/unsupported.drawio",
            }));
        } finally {
            if (hadDecompressionStream) {
                Object.defineProperty(target, "DecompressionStream", {
                    configurable: true,
                    writable: true,
                    value: originalDecompressionStream,
                });
            }
        }
    });

    it("falls back to source XML for malformed draw.io XML", async () => {
        act(() => {
            root.render(
                <DrawioRenderer
                    kind="drawio-xml"
                    content="<mxfile><diagram>"
                    filePath="docs/malformed.drawio"
                />,
            );
        });
        await waitForRenderer(() => {
            expect(container.querySelector(".file-view-drawio-code")).not.toBeNull();
        });

        expect(container.querySelector(".file-view-drawio-svg")).toBeNull();
        expect(container.querySelector(".file-view-drawio-code")).not.toBeNull();
        expect(container.textContent).toContain("Invalid draw.io XML");
        expect(consoleWarnSpy).toHaveBeenCalledWith("[drawio] failed to render xml preview", expect.objectContaining({
            phase: "extract",
            reason: "invalid-xml",
            filePath: "docs/malformed.drawio",
        }));
    });

    it("falls back to source XML when a draw.io model has no renderable vertices", async () => {
        act(() => {
            root.render(
                <DrawioRenderer
                    kind="drawio-xml"
                    content={`
                        <mxGraphModel>
                            <root>
                                <mxCell id="0"/>
                                <mxCell id="1" parent="0"/>
                                <mxCell id="2" edge="1" parent="1"/>
                            </root>
                        </mxGraphModel>
                    `}
                    filePath="docs/no-vertices.drawio"
                />,
            );
        });
        await waitForRenderer(() => {
            expect(container.querySelector(".file-view-drawio-code")).not.toBeNull();
        });

        expect(container.querySelector(".file-view-drawio-svg")).toBeNull();
        expect(container.querySelector(".file-view-drawio-code")).not.toBeNull();
        expect(container.textContent).toContain("No renderable draw.io shapes were found");
        expect(consoleWarnSpy).toHaveBeenCalledWith("[drawio] failed to render xml preview", expect.objectContaining({
            phase: "parse",
            reason: "no-renderable-nodes",
            filePath: "docs/no-vertices.drawio",
        }));
    });

    it("falls back to source XML for broken compressed draw.io payloads", async () => {
        act(() => {
            root.render(
                <DrawioRenderer
                    kind="drawio-xml"
                    content="<mxfile><diagram id='bad'>***</diagram></mxfile>"
                    filePath="docs/broken-compressed.drawio"
                />,
            );
        });
        await waitForRenderer(() => {
            expect(container.querySelector(".file-view-drawio-code")).not.toBeNull();
        });

        expect(container.querySelector(".file-view-drawio-svg")).toBeNull();
        expect(container.textContent).toContain("Unable to decode this compressed draw.io diagram");
        expect(consoleWarnSpy).toHaveBeenCalledWith("[drawio] failed to render xml preview", expect.objectContaining({
            phase: "decompress",
            reason: "compressed-preview-decode-failed",
            filePath: "docs/broken-compressed.drawio",
        }));
    });

    it("falls back to source XML when a compressed draw.io payload exceeds the preview limit", async () => {
        const restoreDecompressionStream = installInflatedTextStream("x".repeat(10 * 1024 * 1024 + 1));
        try {
            act(() => {
                root.render(
                    <DrawioRenderer
                        kind="drawio-xml"
                        content="<mxfile><diagram id='oversized'>AA==</diagram></mxfile>"
                        filePath="docs/oversized.drawio"
                    />,
                );
            });
            await waitForRenderer(() => {
                expect(container.querySelector(".file-view-drawio-code")).not.toBeNull();
            });

            expect(container.querySelector(".file-view-drawio-svg")).toBeNull();
            expect(container.textContent).toContain("Decompressed draw.io XML exceeds the 10 MB render limit");
            expect(consoleWarnSpy).toHaveBeenCalledWith("[drawio] failed to render xml preview", expect.objectContaining({
                phase: "decompress",
                reason: "compressed-preview-decode-failed",
                filePath: "docs/oversized.drawio",
            }));
        } finally {
            restoreDecompressionStream();
        }
    });

    it("ignores stale compressed preview results after content changes", async () => {
        const delayedModel = `
            <mxGraphModel>
                <root>
                    <mxCell id="0"/>
                    <mxCell id="1" parent="0"/>
                    <mxCell id="2" value="Stale page" vertex="1" parent="1">
                        <mxGeometry x="0" y="0" width="120" height="60" as="geometry"/>
                    </mxCell>
                </root>
            </mxGraphModel>
        `;
        const restoreDecompressionStream = installInflatedTextStream(delayedModel, 30);
        try {
            act(() => {
                root.render(
                    <DrawioRenderer
                        kind="drawio-xml"
                        content="<mxfile><diagram id='slow'>AA==</diagram></mxfile>"
                        filePath="docs/stale.drawio"
                    />,
                );
            });
            act(() => {
                root.render(
                    <DrawioRenderer
                        kind="drawio-xml"
                        content={`
                            <mxGraphModel>
                                <root>
                                    <mxCell id="0"/>
                                    <mxCell id="1" parent="0"/>
                                    <mxCell id="2" value="Current page" vertex="1" parent="1">
                                        <mxGeometry x="0" y="0" width="120" height="60" as="geometry"/>
                                    </mxCell>
                                </root>
                            </mxGraphModel>
                        `}
                        filePath="docs/current.drawio"
                    />,
                );
            });
            await waitForRenderer(() => {
                expect(container.textContent).toContain("Current page");
            });
            await act(async () => {
                await new Promise((resolve) => setTimeout(resolve, 40));
            });

            expect(container.textContent).toContain("Current page");
            expect(container.textContent).not.toContain("Stale page");
        } finally {
            restoreDecompressionStream();
        }
    });

    it("aborts compressed preview work without updating state after unmount", async () => {
        const consoleErrorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
        const restoreDecompressionStream = installInflatedTextStream(`
            <mxGraphModel>
                <root>
                    <mxCell id="0"/>
                    <mxCell id="1" parent="0"/>
                    <mxCell id="2" value="Unmounted page" vertex="1" parent="1">
                        <mxGeometry x="0" y="0" width="120" height="60" as="geometry"/>
                    </mxCell>
                </root>
            </mxGraphModel>
        `, 30);
        try {
            act(() => {
                root.render(
                    <DrawioRenderer
                        kind="drawio-xml"
                        content="<mxfile><diagram id='slow'>AA==</diagram></mxfile>"
                        filePath="docs/unmounted.drawio"
                    />,
                );
            });
            act(() => {
                root.render(null);
            });
            await act(async () => {
                await new Promise((resolve) => setTimeout(resolve, 40));
            });

            expect(container.innerHTML).toBe("");
            expect(consoleErrorSpy).not.toHaveBeenCalled();
        } finally {
            restoreDecompressionStream();
            consoleErrorSpy.mockRestore();
        }
    });
});
