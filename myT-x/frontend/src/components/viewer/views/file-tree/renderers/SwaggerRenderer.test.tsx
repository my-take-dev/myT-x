import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {SwaggerRenderer} from "./SwaggerRenderer";

const swaggerUIRenderMock = vi.fn(({spec}: {spec?: object}) => (
    <div className="swagger-ui" data-rendered="swagger-ui">{JSON.stringify(spec)}</div>
));

vi.mock("swagger-ui-react", () => ({
    default: swaggerUIRenderMock,
}));

describe("SwaggerRenderer", () => {
    let container: HTMLDivElement;
    let root: Root;
    let consoleWarnSpy: ReturnType<typeof vi.spyOn>;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
        swaggerUIRenderMock.mockClear();
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

    async function flushRenderer(): Promise<void> {
        await act(async () => {
            await Promise.resolve();
            await new Promise((resolve) => setTimeout(resolve, 0));
            await Promise.resolve();
            await new Promise((resolve) => setTimeout(resolve, 0));
        });
    }

    it("renders parsed OpenAPI content with swagger-ui-react", async () => {
        await act(async () => {
            root.render(
                <SwaggerRenderer
                    filePath="docs/openapi.yaml"
                    content={"openapi: 3.0.3\ninfo:\n  title: Sample\n  version: 1.0.0"}
                />,
            );
        });
        await flushRenderer();

        expect(swaggerUIRenderMock).toHaveBeenCalledTimes(1);
        expect(swaggerUIRenderMock.mock.calls[0]?.[0]).toMatchObject({
            docExpansion: "list",
            defaultModelsExpandDepth: -1,
            deepLinking: true,
            filter: true,
            displayRequestDuration: true,
            supportedSubmitMethods: [],
            tryItOutEnabled: false,
        });
        expect(container.textContent).toContain("\"openapi\":\"3.0.3\"");
        expect(container.querySelector(".file-view-swagger .swagger-ui")).not.toBeNull();
    });

    it("enables Try it out only after the local opt-in toggle is pressed", async () => {
        await act(async () => {
            root.render(
                <SwaggerRenderer
                    filePath="docs/openapi.yaml"
                    content={"openapi: 3.0.3\ninfo:\n  title: Sample\n  version: 1.0.0"}
                />,
            );
        });
        await flushRenderer();

        const toggleButton = container.querySelector<HTMLButtonElement>(".file-view-swagger-try-it-out-toggle");
        expect(toggleButton).not.toBeNull();
        expect(toggleButton?.getAttribute("aria-pressed")).toBe("false");
        expect(container.textContent).toContain("このプレビューで「Try it out」を有効にするまで、リクエスト実行は無効です。");
        expect(toggleButton?.textContent).toBe("Try it out を有効化");

        await act(async () => {
            toggleButton?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });

        expect(toggleButton?.getAttribute("aria-pressed")).toBe("true");
        expect(toggleButton?.textContent).toBe("Try it out を無効化");
        expect(swaggerUIRenderMock).toHaveBeenCalledTimes(2);
        expect(swaggerUIRenderMock.mock.calls[1]?.[0]).toMatchObject({
            supportedSubmitMethods: ["get", "post", "put", "patch", "delete"],
            tryItOutEnabled: true,
        });
    });

    it("keeps Try it out enabled when the same Swagger document refreshes", async () => {
        await act(async () => {
            root.render(
                <SwaggerRenderer
                    filePath="docs/openapi.yaml"
                    content={"openapi: 3.0.3\ninfo:\n  title: First\n  version: 1.0.0"}
                />,
            );
        });
        await flushRenderer();

        const toggleButton = container.querySelector<HTMLButtonElement>(".file-view-swagger-try-it-out-toggle");
        await act(async () => {
            toggleButton?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });

        await act(async () => {
            root.render(
                <SwaggerRenderer
                    filePath="docs/openapi.yaml"
                    content={"openapi: 3.0.3\ninfo:\n  title: Refreshed\n  version: 1.0.1"}
                />,
            );
        });
        await flushRenderer();

        const refreshedToggleButton = container.querySelector<HTMLButtonElement>(".file-view-swagger-try-it-out-toggle");
        expect(refreshedToggleButton?.getAttribute("aria-pressed")).toBe("true");
        expect(swaggerUIRenderMock.mock.calls.at(-1)?.[0]).toMatchObject({
            supportedSubmitMethods: ["get", "post", "put", "patch", "delete"],
            tryItOutEnabled: true,
        });
    });

    it("resets Try it out after switching to a different Swagger document", async () => {
        await act(async () => {
            root.render(
                <SwaggerRenderer
                    filePath="docs/first.yaml"
                    content={"openapi: 3.0.3\ninfo:\n  title: First\n  version: 1.0.0"}
                />,
            );
        });
        await flushRenderer();

        const toggleButton = container.querySelector<HTMLButtonElement>(".file-view-swagger-try-it-out-toggle");
        await act(async () => {
            toggleButton?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });

        await act(async () => {
            root.render(
                <SwaggerRenderer
                    filePath="docs/second.yaml"
                    content={"openapi: 3.0.3\ninfo:\n  title: Second\n  version: 1.0.0"}
                />,
            );
        });
        await flushRenderer();

        const nextToggleButton = container.querySelector<HTMLButtonElement>(".file-view-swagger-try-it-out-toggle");
        expect(nextToggleButton?.getAttribute("aria-pressed")).toBe("false");
        expect(swaggerUIRenderMock.mock.calls.at(-1)?.[0]).toMatchObject({
            supportedSubmitMethods: [],
            tryItOutEnabled: false,
        });
    });

    it("shows an inline error for invalid Swagger documents", async () => {
        await act(async () => {
            root.render(
                <SwaggerRenderer
                    filePath="docs/broken.json"
                    content={"{\"openapi\":"}
                />,
            );
        });
        await flushRenderer();

        expect(swaggerUIRenderMock).not.toHaveBeenCalled();
        expect(container.textContent).toContain("Unexpected end of JSON input");
    });
});
