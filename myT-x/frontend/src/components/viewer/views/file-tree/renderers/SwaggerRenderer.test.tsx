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
            deepLinking: false,
            supportedSubmitMethods: [],
        });
        expect(container.textContent).toContain("\"openapi\":\"3.0.3\"");
        expect(container.querySelector(".file-view-swagger .swagger-ui")).not.toBeNull();
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
