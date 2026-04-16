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

import {MarkdownRenderer} from "../src/components/viewer/views/file-tree/renderers/MarkdownRenderer";

async function flushAsyncRender(): Promise<void> {
    await act(async () => {
        await new Promise((resolve) => setTimeout(resolve, 0));
        await new Promise((resolve) => setTimeout(resolve, 0));
    });
}

describe("MarkdownRenderer", () => {
    let container: HTMLDivElement;
    let root: Root;
    let createObjectURLSpy: ReturnType<typeof vi.spyOn>;
    let revokeObjectURLSpy: ReturnType<typeof vi.spyOn>;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        apiMock.DevPanelReadBinary.mockReset();
        createObjectURLSpy = vi.spyOn(URL, "createObjectURL").mockReturnValue("blob:resolved-image");
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

    it("renders markdown content through MarkdownPreview", () => {
        act(() => {
            root.render(<MarkdownRenderer content={"# Title\n\nBody"}/>);
        });

        const heading = container.querySelector("h1");
        expect(heading?.textContent).toBe("Title");
        expect(container.querySelector(".md-preview-body")?.textContent).toContain("Body");
    });

    it("resolves relative markdown images through DevPanelReadBinary", async () => {
        apiMock.DevPanelReadBinary.mockResolvedValue({
            path: "docs/images/example.png",
            data: "iVBORw0KGgo=",
            mime: "image/png",
        });

        act(() => {
            root.render(
                <MarkdownRenderer
                    content={"![diagram](./images/example.png)"}
                    filePath="docs/readme.md"
                    sessionKey="session-a:1"
                    sessionName="session-a"
                />,
            );
        });
        await flushAsyncRender();

        expect(apiMock.DevPanelReadBinary).toHaveBeenCalledWith("session-a", "docs/images/example.png");
        const image = container.querySelector("img");
        expect(image?.getAttribute("src")).toBe("blob:resolved-image");
        expect(image?.getAttribute("data-local-image-resolved")).toBe("true");
        expect(createObjectURLSpy).toHaveBeenCalledTimes(1);
    });

    it("revokes object URLs when resolved images are removed", async () => {
        apiMock.DevPanelReadBinary.mockResolvedValue({
            path: "docs/images/example.png",
            data: "iVBORw0KGgo=",
            mime: "image/png",
        });

        act(() => {
            root.render(
                <MarkdownRenderer
                    content={"![diagram](./images/example.png)"}
                    filePath="docs/readme.md"
                    sessionKey="session-a:1"
                    sessionName="session-a"
                />,
            );
        });
        await flushAsyncRender();

        act(() => {
            root.render(
                <MarkdownRenderer
                    content={"# Title\n\nNo images"}
                    filePath="docs/readme.md"
                    sessionKey="session-a:1"
                    sessionName="session-a"
                />,
            );
        });
        await flushAsyncRender();

        expect(revokeObjectURLSpy).toHaveBeenCalledWith("blob:resolved-image");
    });

    it("reloads resolved images when same-file markdown content changes", async () => {
        createObjectURLSpy.mockRestore();
        createObjectURLSpy = vi.spyOn(URL, "createObjectURL")
            .mockReturnValueOnce("blob:resolved-image-a")
            .mockReturnValueOnce("blob:resolved-image-b");

        apiMock.DevPanelReadBinary
            .mockResolvedValueOnce({
                path: "docs/images/example.png",
                data: "iVBORw0KGgo=",
                mime: "image/png",
            })
            .mockResolvedValueOnce({
                path: "docs/images/example.png",
                data: "iVBORw0KGgoAAAANSUhEUg==",
                mime: "image/png",
            });

        act(() => {
            root.render(
                <MarkdownRenderer
                    content={"![diagram](./images/example.png)"}
                    filePath="docs/readme.md"
                    sessionKey="session-a:1"
                    sessionName="session-a"
                />,
            );
        });
        await flushAsyncRender();

        act(() => {
            root.render(
                <MarkdownRenderer
                    content={"# Updated\n\n![diagram](./images/example.png)"}
                    filePath="docs/readme.md"
                    sessionKey="session-a:1"
                    sessionName="session-a"
                />,
            );
        });
        await flushAsyncRender();

        expect(apiMock.DevPanelReadBinary).toHaveBeenCalledTimes(2);
        expect(container.querySelector("img")?.getAttribute("src")).toBe("blob:resolved-image-b");
        expect(revokeObjectURLSpy).toHaveBeenCalledWith("blob:resolved-image-a");
    });

    it("retries failed image loads when same-file markdown content changes", async () => {
        apiMock.DevPanelReadBinary
            .mockRejectedValueOnce(new Error("temporary read failure"))
            .mockResolvedValueOnce({
                path: "docs/images/example.png",
                data: "iVBORw0KGgo=",
                mime: "image/png",
            });

        act(() => {
            root.render(
                <MarkdownRenderer
                    content={"![diagram](./images/example.png)"}
                    filePath="docs/readme.md"
                    sessionKey="session-a:1"
                    sessionName="session-a"
                />,
            );
        });
        await flushAsyncRender();
        expect(apiMock.DevPanelReadBinary).toHaveBeenCalledTimes(1);
        expect(container.querySelector("img")?.getAttribute("data-local-image-resolved")).toBeNull();

        act(() => {
            root.render(
                <MarkdownRenderer
                    content={"# Retry\n\n![diagram](./images/example.png)"}
                    filePath="docs/readme.md"
                    sessionKey="session-a:1"
                    sessionName="session-a"
                />,
            );
        });
        await flushAsyncRender();

        expect(apiMock.DevPanelReadBinary).toHaveBeenCalledTimes(2);
        expect(container.querySelector("img")?.getAttribute("data-local-image-resolved")).toBe("true");
    });

    it("retries failed image loads within the same file and session", async () => {
        vi.useFakeTimers();
        try {
            apiMock.DevPanelReadBinary
                .mockRejectedValueOnce(new Error("temporary read failure"))
                .mockResolvedValueOnce({
                    path: "docs/images/example.png",
                    data: "iVBORw0KGgo=",
                    mime: "image/png",
                });

            act(() => {
                root.render(
                    <MarkdownRenderer
                        content={"![diagram](./images/example.png)"}
                        filePath="docs/readme.md"
                        sessionKey="session-a:1"
                        sessionName="session-a"
                    />,
                );
            });

            await act(async () => {
                await Promise.resolve();
                await Promise.resolve();
            });
            expect(apiMock.DevPanelReadBinary).toHaveBeenCalledTimes(1);
            expect(container.querySelector("img")?.getAttribute("data-local-image-resolved")).toBeNull();

            await act(async () => {
                await vi.advanceTimersByTimeAsync(750);
                await Promise.resolve();
                await Promise.resolve();
            });

            expect(apiMock.DevPanelReadBinary).toHaveBeenCalledTimes(2);
            expect(container.querySelector("img")?.getAttribute("data-local-image-resolved")).toBe("true");
        } finally {
            vi.useRealTimers();
        }
    });

    it("keeps retrying repeated transient image failures until the file becomes available", async () => {
        vi.useFakeTimers();
        try {
            apiMock.DevPanelReadBinary
                .mockRejectedValueOnce(new Error("temporary read failure 1"))
                .mockRejectedValueOnce(new Error("temporary read failure 2"))
                .mockRejectedValueOnce(new Error("temporary read failure 3"))
                .mockRejectedValueOnce(new Error("temporary read failure 4"))
                .mockResolvedValueOnce({
                    path: "docs/images/example.png",
                    data: "iVBORw0KGgo=",
                    mime: "image/png",
                });

            act(() => {
                root.render(
                    <MarkdownRenderer
                        content={"![diagram](./images/example.png)"}
                        filePath="docs/readme.md"
                        sessionKey="session-a:1"
                        sessionName="session-a"
                    />,
                );
            });

            await act(async () => {
                await Promise.resolve();
                await Promise.resolve();
            });
            expect(apiMock.DevPanelReadBinary).toHaveBeenCalledTimes(1);

            for (const delay of [750, 1_500, 3_000, 6_000]) {
                await act(async () => {
                    await vi.advanceTimersByTimeAsync(delay);
                    await Promise.resolve();
                    await Promise.resolve();
                });
            }

            expect(apiMock.DevPanelReadBinary).toHaveBeenCalledTimes(5);
            expect(container.querySelector("img")?.getAttribute("data-local-image-resolved")).toBe("true");
        } finally {
            vi.useRealTimers();
        }
    });

    it("preserves scheduled retries when another image resolves first", async () => {
        vi.useFakeTimers();
        try {
            let secondImageAttempt = 0;
            apiMock.DevPanelReadBinary.mockImplementation((_session, path) => {
                if (path === "docs/images/a.png") {
                    return Promise.resolve({
                        path,
                        data: "iVBORw0KGgo=",
                        mime: "image/png",
                    });
                }
                if (path === "docs/images/b.png") {
                    secondImageAttempt += 1;
                    if (secondImageAttempt === 1) {
                        return Promise.reject(new Error("temporary read failure"));
                    }
                    return Promise.resolve({
                        path,
                        data: "iVBORw0KGgoAAAANSUhEUg==",
                        mime: "image/png",
                    });
                }
                return Promise.reject(new Error(`unexpected path: ${String(path)}`));
            });

            act(() => {
                root.render(
                    <MarkdownRenderer
                        content={"![a](./images/a.png)\n\n![b](./images/b.png)"}
                        filePath="docs/readme.md"
                        sessionKey="session-a:1"
                        sessionName="session-a"
                    />,
                );
            });

            await act(async () => {
                await Promise.resolve();
                await Promise.resolve();
            });
            expect(apiMock.DevPanelReadBinary).toHaveBeenCalledTimes(2);

            await act(async () => {
                await vi.advanceTimersByTimeAsync(750);
                await Promise.resolve();
                await Promise.resolve();
            });

            expect(apiMock.DevPanelReadBinary).toHaveBeenCalledTimes(3);
            expect(container.querySelectorAll("img[data-local-image-resolved='true']")).toHaveLength(2);
        } finally {
            vi.useRealTimers();
        }
    });

    it("revokes partially resolved object URLs when the component unmounts mid-load", async () => {
        createObjectURLSpy.mockRestore();
        createObjectURLSpy = vi.spyOn(URL, "createObjectURL").mockReturnValue("blob:first-image");

        let resolveFirst: ((value: unknown) => void) | null = null;
        let resolveSecond: ((value: unknown) => void) | null = null;
        const firstPromise = new Promise((resolve) => {
            resolveFirst = resolve;
        });
        const secondPromise = new Promise((resolve) => {
            resolveSecond = resolve;
        });

        apiMock.DevPanelReadBinary.mockImplementation((_session, path) => {
            if (path === "docs/images/a.png") {
                return firstPromise;
            }
            if (path === "docs/images/b.png") {
                return secondPromise;
            }
            return Promise.reject(new Error(`unexpected path: ${String(path)}`));
        });

        act(() => {
            root.render(
                <MarkdownRenderer
                    content={"![a](./images/a.png)\n\n![b](./images/b.png)"}
                    filePath="docs/readme.md"
                    sessionKey="session-a:1"
                    sessionName="session-a"
                />,
            );
        });

        await act(async () => {
            resolveFirst?.({
                path: "docs/images/a.png",
                data: "iVBORw0KGgo=",
                mime: "image/png",
            });
            await Promise.resolve();
            await Promise.resolve();
        });

        expect(apiMock.DevPanelReadBinary).toHaveBeenCalledTimes(2);

        act(() => {
            root.unmount();
        });

        await act(async () => {
            resolveSecond?.({
                path: "docs/images/b.png",
                data: "iVBORw0KGgo=",
                mime: "image/png",
            });
            await Promise.resolve();
            await Promise.resolve();
        });

        expect(revokeObjectURLSpy).toHaveBeenCalledWith("blob:first-image");

        root = createRoot(container);
    });
});
