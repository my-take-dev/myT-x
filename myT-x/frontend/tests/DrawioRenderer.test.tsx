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

async function flushAsyncRender(): Promise<void> {
    await act(async () => {
        await new Promise((resolve) => setTimeout(resolve, 0));
        await new Promise((resolve) => setTimeout(resolve, 0));
    });
}

describe("DrawioRenderer", () => {
    let container: HTMLDivElement;
    let root: Root;
    let createObjectURLSpy: ReturnType<typeof vi.spyOn>;
    let revokeObjectURLSpy: ReturnType<typeof vi.spyOn>;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        apiMock.DevPanelReadBinary.mockReset();
        createObjectURLSpy = vi.spyOn(URL, "createObjectURL")
            .mockReturnValueOnce("blob:drawio-preview-a")
            .mockReturnValueOnce("blob:drawio-preview-b");
        revokeObjectURLSpy = vi.spyOn(URL, "revokeObjectURL").mockImplementation(() => {});
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        container.remove();
        createObjectURLSpy.mockRestore();
        revokeObjectURLSpy.mockRestore();
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
        await flushAsyncRender();

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
        await flushAsyncRender();

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
        await flushAsyncRender();
        expect(container.textContent).toContain("temporary read failure");

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
        await flushAsyncRender();

        expect(apiMock.DevPanelReadBinary).toHaveBeenCalledTimes(2);
        expect(container.querySelector("img")?.getAttribute("src")).toBe("blob:drawio-preview-a");
        expect(container.textContent).not.toContain("temporary read failure");
    });
});
