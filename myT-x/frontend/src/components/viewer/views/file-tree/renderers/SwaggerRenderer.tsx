import {memo, useEffect, useMemo, useState, type ComponentType} from "react";
import {load as loadYaml} from "js-yaml";
import {toErrorMessage} from "../../../../../utils/errorUtils";
import "swagger-ui-react/swagger-ui.css";

type SwaggerHTTPMethod = "get" | "put" | "post" | "delete" | "options" | "head" | "patch" | "trace";

interface SwaggerUIProps {
    readonly spec: object;
    readonly docExpansion?: "list" | "full" | "none";
    readonly defaultModelsExpandDepth?: number;
    readonly deepLinking?: boolean;
    readonly supportedSubmitMethods?: SwaggerHTTPMethod[];
}

interface SwaggerRendererProps {
    readonly content: string;
    readonly filePath?: string;
}

interface ParseResult {
    readonly spec: object | null;
    readonly error: string | null;
}

type SwaggerUIComponent = ComponentType<SwaggerUIProps>;

const NO_SUPPORTED_SUBMIT_METHODS: SwaggerHTTPMethod[] = [];
let swaggerModulePromise: Promise<SwaggerUIComponent> | null = null;

function trimLeadingByteOrderMark(content: string): string {
    return content.startsWith("\uFEFF") ? content.slice(1) : content;
}

function isObjectRecord(value: unknown): value is Record<string, unknown> {
    return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isJSONDocument(path?: string): boolean {
    return path?.trim().toLowerCase().endsWith(".json") ?? false;
}

async function loadSwaggerUI(): Promise<SwaggerUIComponent> {
    if (swaggerModulePromise === null) {
        swaggerModulePromise = import("swagger-ui-react").then((module) => module.default as SwaggerUIComponent);
    }
    return swaggerModulePromise;
}

function parseSwaggerSpec(content: string, filePath?: string): ParseResult {
    const normalizedContent = trimLeadingByteOrderMark(content).trim();
    if (normalizedContent === "") {
        return {
            spec: null,
            error: "Swagger preview requires a non-empty document.",
        };
    }

    try {
        const parsed = isJSONDocument(filePath)
            ? JSON.parse(normalizedContent) as unknown
            : loadYaml(normalizedContent, {json: true});
        if (!isObjectRecord(parsed)) {
            return {
                spec: null,
                error: "Swagger preview requires a top-level object document.",
            };
        }
        if (!("openapi" in parsed) && !("swagger" in parsed)) {
            return {
                spec: null,
                error: "Swagger preview requires a top-level openapi or swagger field.",
            };
        }
        return {
            spec: parsed,
            error: null,
        };
    } catch (err: unknown) {
        return {
            spec: null,
            error: toErrorMessage(err, "Failed to parse Swagger document."),
        };
    }
}

export const SwaggerRenderer = memo(function SwaggerRenderer({content, filePath}: SwaggerRendererProps) {
    const parsedSpec = useMemo(() => parseSwaggerSpec(content, filePath), [content, filePath]);
    const [SwaggerUI, setSwaggerUI] = useState<SwaggerUIComponent | null>(null);
    const [loadError, setLoadError] = useState<string | null>(null);

    useEffect(() => {
        let cancelled = false;
        setSwaggerUI(null);
        setLoadError(null);

        const loadComponent = async () => {
            try {
                const component = await loadSwaggerUI();
                if (cancelled) {
                    return;
                }
                setSwaggerUI(() => component);
            } catch (err: unknown) {
                console.warn("[swagger] failed to load preview", err);
                if (cancelled) {
                    return;
                }
                setLoadError(toErrorMessage(err, "Failed to load Swagger preview."));
            }
        };

        void loadComponent();

        return () => {
            cancelled = true;
        };
    }, []);

    if (parsedSpec.error) {
        return <div className="file-content-empty">{parsedSpec.error}</div>;
    }

    if (loadError) {
        return <div className="file-content-empty">{loadError}</div>;
    }

    if (SwaggerUI === null || parsedSpec.spec === null) {
        return <div className="file-content-empty">Loading Swagger preview...</div>;
    }

    return (
        <div className="file-view-swagger">
            <SwaggerUI
                spec={parsedSpec.spec}
                docExpansion="list"
                defaultModelsExpandDepth={-1}
                deepLinking={false}
                supportedSubmitMethods={NO_SUPPORTED_SUBMIT_METHODS}
            />
        </div>
    );
});
