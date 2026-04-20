import {lazy, memo, useCallback, type ReactElement} from "react";
import type {PluggableList} from "unified";
import {MarkdownPreview, type MarkdownCodeBlockRendererProps} from "../MarkdownPreview";
import {useRehypeResolveImages} from "../rehypeResolveImages";
import {RendererSurface} from "./RendererSurface";

interface MarkdownRendererProps {
    readonly content: string;
    readonly filePath?: string;
    readonly sessionKey?: string;
    readonly sessionName?: string | null;
}

const EMPTY_REHYPE_PLUGINS: PluggableList = [];
const LazyMermaidRenderer = lazy(async () => ({
    default: (await import("./MermaidRenderer")).MermaidRenderer,
}));
const LazyGraphvizRenderer = lazy(async () => ({
    default: (await import("./GraphvizRenderer")).GraphvizRenderer,
}));
const LazyMarkmapRenderer = lazy(async () => ({
    default: (await import("./MarkmapRenderer")).MarkmapRenderer,
}));
const LazyWavedromRenderer = lazy(async () => ({
    default: (await import("./WavedromRenderer")).WavedromRenderer,
}));
const LazyVegaLiteRenderer = lazy(async () => ({
    default: (await import("./VegaLiteRenderer")).VegaLiteRenderer,
}));

type DiagramLanguage = "mermaid" | "graphviz" | "markmap" | "wavedrom" | "vega-lite" | "vega";

const LANGUAGE_TO_DIAGRAM: ReadonlyMap<string, DiagramLanguage> = new Map([
    ["language-mermaid", "mermaid"],
    ["language-graphviz", "graphviz"],
    ["language-dot", "graphviz"],
    ["language-markmap", "markmap"],
    ["language-wavedrom", "wavedrom"],
    ["language-vega-lite", "vega-lite"],
    ["language-vegalite", "vega-lite"],
    ["language-vega", "vega"],
]);

function detectDiagramLanguage(className?: string): DiagramLanguage | null {
    if (!className) {
        return null;
    }
    for (const token of className.toLowerCase().split(/\s+/)) {
        const match = LANGUAGE_TO_DIAGRAM.get(token);
        if (match !== undefined) {
            return match;
        }
    }
    return null;
}

function renderDiagram(language: DiagramLanguage, code: string): ReactElement {
    switch (language) {
        case "mermaid":
            return <LazyMermaidRenderer code={code}/>;
        case "graphviz":
            return <LazyGraphvizRenderer code={code}/>;
        case "markmap":
            return <LazyMarkmapRenderer code={code}/>;
        case "wavedrom":
            return <LazyWavedromRenderer code={code}/>;
        case "vega-lite":
            return <LazyVegaLiteRenderer code={code} kind="vega-lite"/>;
        case "vega":
            return <LazyVegaLiteRenderer code={code} kind="vega"/>;
    }
}

export const MarkdownRenderer = memo(function MarkdownRenderer({
    content,
    filePath,
    sessionKey,
    sessionName,
}: MarkdownRendererProps) {
    const rehypePlugins = useRehypeResolveImages({
        content,
        filePath,
        sessionKey,
        sessionName,
    });

    const codeBlockRenderer = useCallback(({className, code}: MarkdownCodeBlockRendererProps) => {
        const language = detectDiagramLanguage(className);
        if (language === null) {
            return null;
        }

        return (
            <RendererSurface
                fallback={<pre className="md-preview-code-block"><code className={className}>{code}</code></pre>}
                filePath={filePath}
                loadingMessage="Loading embedded diagram preview..."
                rendererName={language}
            >
                {renderDiagram(language, code)}
            </RendererSurface>
        );
    }, [filePath]);

    return (
        <MarkdownPreview
            content={content}
            rehypePlugins={rehypePlugins ?? EMPTY_REHYPE_PLUGINS}
            codeBlockRenderer={codeBlockRenderer}
        />
    );
});
