import {memo, useEffect, useMemo, useState, type ComponentType} from "react";
import {load as loadYaml} from "js-yaml";
import {toErrorMessage} from "../../../../../utils/errorUtils";
import {createRetriableAsyncSingleton} from "../../../../../utils/createRetriableAsyncSingleton";
import {translate, useI18n} from "../../../../../i18n";
import "swagger-ui-react/swagger-ui.css";

type SwaggerHTTPMethod = "get" | "put" | "post" | "delete" | "options" | "head" | "patch" | "trace";

interface SwaggerUIProps {
    readonly spec: object;
    readonly docExpansion?: "list" | "full" | "none";
    readonly defaultModelsExpandDepth?: number;
    readonly deepLinking?: boolean;
    readonly filter?: string | boolean;
    readonly displayRequestDuration?: boolean;
    readonly supportedSubmitMethods?: SwaggerHTTPMethod[];
    readonly tryItOutEnabled?: boolean;
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
type SwaggerUIModule = {
    readonly default: SwaggerUIComponent;
};

const NO_SUPPORTED_SUBMIT_METHODS: SwaggerHTTPMethod[] = [];
const TRY_IT_OUT_SUPPORTED_SUBMIT_METHODS: SwaggerHTTPMethod[] = ["get", "post", "put", "patch", "delete"];

function trimLeadingByteOrderMark(content: string): string {
    return content.startsWith("\uFEFF") ? content.slice(1) : content;
}

function isObjectRecord(value: unknown): value is Record<string, unknown> {
    return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isJSONDocument(path?: string): boolean {
    return path?.trim().toLowerCase().endsWith(".json") ?? false;
}

const loadSwaggerUI = createRetriableAsyncSingleton(
    async (): Promise<SwaggerUIComponent> => ((await import("swagger-ui-react")) as SwaggerUIModule).default,
);

function parseSwaggerSpec(content: string, filePath?: string): ParseResult {
    const normalizedContent = trimLeadingByteOrderMark(content).trim();
    if (normalizedContent === "") {
        return {
            spec: null,
            error: translate("viewer.swagger.error.empty", "Swagger プレビューには空でないドキュメントが必要です。"),
        };
    }

    try {
        const parsed = isJSONDocument(filePath)
            ? JSON.parse(normalizedContent) as unknown
            : loadYaml(normalizedContent, {json: true});
        if (!isObjectRecord(parsed)) {
            return {
                spec: null,
                error: translate("viewer.swagger.error.topLevelObject", "Swagger プレビューにはトップレベルのオブジェクトが必要です。"),
            };
        }
        if (!("openapi" in parsed) && !("swagger" in parsed)) {
            return {
                spec: null,
                error: translate("viewer.swagger.error.openapiRequired", "Swagger プレビューにはトップレベルの openapi または swagger フィールドが必要です。"),
            };
        }
        return {
            spec: parsed,
            error: null,
        };
    } catch (err: unknown) {
        return {
            spec: null,
            error: toErrorMessage(err, translate("viewer.swagger.error.parseFailed", "Swagger ドキュメントの解析に失敗しました。")),
        };
    }
}

export const SwaggerRenderer = memo(function SwaggerRenderer({content, filePath}: SwaggerRendererProps) {
    const {t} = useI18n();
    const parsedSpec = useMemo(() => parseSwaggerSpec(content, filePath), [content, filePath]);
    const [SwaggerUI, setSwaggerUI] = useState<SwaggerUIComponent | null>(null);
    const [loadError, setLoadError] = useState<string | null>(null);
    const [isTryItOutEnabled, setIsTryItOutEnabled] = useState(false);

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
                setLoadError(toErrorMessage(err, t("viewer.swagger.error.loadFailed", "Swagger プレビューの読み込みに失敗しました。")));
            }
        };

        void loadComponent();

        return () => {
            cancelled = true;
        };
    }, [t]);

    useEffect(() => {
        // Keep the opt-in state across same-document refreshes; switching files resets it.
        setIsTryItOutEnabled(false);
    }, [filePath]);

    if (parsedSpec.error) {
        return <div className="file-content-empty">{parsedSpec.error}</div>;
    }

    if (loadError) {
        return <div className="file-content-empty">{loadError}</div>;
    }

    if (SwaggerUI === null || parsedSpec.spec === null) {
        return <div className="file-content-empty">{t("viewer.swagger.loading", "Swagger プレビューを読み込み中...")}</div>;
    }

    const supportedSubmitMethods = isTryItOutEnabled
        ? TRY_IT_OUT_SUPPORTED_SUBMIT_METHODS
        : NO_SUPPORTED_SUBMIT_METHODS;

    return (
        <div className="file-view-swagger">
            <div className="file-view-swagger-toolbar">
                <p className="file-view-swagger-toolbar-copy">
                    {t("viewer.swagger.tryItOut.hint", "このプレビューで「Try it out」を有効にするまで、リクエスト実行は無効です。")}
                </p>
                <button
                    type="button"
                    className={`file-view-swagger-try-it-out-toggle${isTryItOutEnabled ? " is-enabled" : ""}`}
                    onClick={() => setIsTryItOutEnabled((currentValue) => !currentValue)}
                    aria-pressed={isTryItOutEnabled}
                >
                    {isTryItOutEnabled
                        ? t("viewer.swagger.tryItOut.disable", "Try it out を無効化")
                        : t("viewer.swagger.tryItOut.enable", "Try it out を有効化")}
                </button>
            </div>
            <SwaggerUI
                spec={parsedSpec.spec}
                docExpansion="list"
                defaultModelsExpandDepth={-1}
                deepLinking
                filter
                displayRequestDuration
                supportedSubmitMethods={supportedSubmitMethods}
                tryItOutEnabled={isTryItOutEnabled}
            />
        </div>
    );
});
