import {memo, useEffect, useRef, useState} from "react";
import {toErrorMessage} from "../../../../../utils/errorUtils";
import {createRetriableAsyncSingleton} from "../../../../../utils/createRetriableAsyncSingleton";
import {useI18n} from "../../../../../i18n";

type VegaEmbedModule = typeof import("vega-embed");
type VegaEmbedResult = Awaited<ReturnType<VegaEmbedModule["default"]>>;

export type VegaKind = "vega" | "vega-lite";

interface VegaLiteRendererProps {
    readonly code: string;
    readonly kind: VegaKind;
    readonly filePath?: string;
}

const loadVegaEmbed = createRetriableAsyncSingleton(async (): Promise<VegaEmbedModule["default"]> => {
    const mod = await import("vega-embed");
    return mod.default;
});

const VEGA_ACTIONS = {
    export: {
        png: true,
        svg: true,
    },
    source: true,
    compiled: true,
    editor: true,
} as const;

function parseSpec(code: string, emptyMessage: string, objectMessage: string): object {
    const trimmed = code.trim();
    if (trimmed === "") {
        throw new Error(emptyMessage);
    }
    const parsed = JSON.parse(trimmed) as unknown;
    if (typeof parsed !== "object" || parsed === null || Array.isArray(parsed)) {
        throw new Error(objectMessage);
    }
    return parsed;
}

export const VegaLiteRenderer = memo(function VegaLiteRenderer({code, kind}: VegaLiteRendererProps) {
    const {t} = useI18n();
    const containerRef = useRef<HTMLDivElement>(null);
    const [isReady, setIsReady] = useState(false);
    const [error, setError] = useState<string | null>(null);

    useEffect(() => {
        let cancelled = false;
        let active: VegaEmbedResult | null = null;
        setIsReady(false);
        setError(null);

        const renderChart = async () => {
            try {
                const spec = parseSpec(
                    code,
                    t("viewer.vega.error.empty", "Vega の specification が空です。"),
                    t("viewer.vega.error.objectRequired", "Vega の specification は JSON オブジェクトである必要があります。"),
                );
                const embed = await loadVegaEmbed();
                if (cancelled) {
                    return;
                }
                const container = containerRef.current;
                if (!container) {
                    return;
                }
                const mode = kind === "vega-lite" ? "vega-lite" : "vega";
                const result = await embed(container, spec as object, {
                    mode,
                    actions: VEGA_ACTIONS,
                    i18n: {
                        CLICK_TO_VIEW_ACTIONS: t("viewer.vega.actions.openMenu", "操作"),
                        PNG_ACTION: t("viewer.vega.actions.exportPng", "PNG を書き出し"),
                        SVG_ACTION: t("viewer.vega.actions.exportSvg", "SVG を書き出し"),
                        SOURCE_ACTION: t("viewer.vega.actions.source", "ソースを表示"),
                        COMPILED_ACTION: t("viewer.vega.actions.compiled", "コンパイル結果を表示"),
                        EDITOR_ACTION: t("viewer.vega.actions.editor", "Vega Editor で開く"),
                    },
                    renderer: "svg",
                });
                if (cancelled) {
                    result.finalize();
                    return;
                }
                active = result;
                setIsReady(true);
            } catch (err: unknown) {
                console.warn("[vega] failed to render chart", err);
                if (cancelled) {
                    return;
                }
                setError(toErrorMessage(err, t("viewer.vega.error.renderFailed", "Vega チャートの描画に失敗しました。")));
            }
        };

        void renderChart();

        return () => {
            cancelled = true;
            if (active !== null) {
                active.finalize();
                active = null;
            }
        };
    }, [code, kind, t]);

    if (error) {
        return <div className="file-content-empty">{error}</div>;
    }

    return (
        <div className="file-view-vega">
            {!isReady ? <div className="file-content-empty">{t("viewer.vega.loading", "Vega チャートを描画中...")}</div> : null}
            <div ref={containerRef} className="file-view-vega-host"/>
        </div>
    );
});
