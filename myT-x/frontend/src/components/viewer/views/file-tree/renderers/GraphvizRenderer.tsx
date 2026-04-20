import {memo, useEffect, useRef, useState} from "react";
import {toErrorMessage} from "../../../../../utils/errorUtils";
import {createRetriableAsyncSingleton} from "../../../../../utils/createRetriableAsyncSingleton";

// Minimal surface of @hpcc-js/wasm-graphviz (the type re-export target
// `@hpcc-js/wasm-graphviz` is not published as a standalone package at the
// time of writing; we declare only the methods we actually call).
interface GraphvizInstance {
    dot(source: string): string;
}

interface GraphvizNamespace {
    load(): Promise<GraphvizInstance>;
}

interface GraphvizModule {
    Graphviz: GraphvizNamespace;
}

interface GraphvizRendererProps {
    readonly code: string;
}

const loadGraphviz = createRetriableAsyncSingleton(async (): Promise<GraphvizInstance> => {
    const imported = (await import("@hpcc-js/wasm/graphviz")) as unknown as GraphvizModule;
    return imported.Graphviz.load();
});

function parseSvgDocument(svgSource: string): SVGElement | null {
    const parser = new DOMParser();
    const parsed = parser.parseFromString(svgSource, "image/svg+xml");
    if (parsed.querySelector("parsererror") !== null) {
        return null;
    }
    const root = parsed.documentElement;
    if (!(root instanceof SVGElement)) {
        return null;
    }
    return root;
}

export const GraphvizRenderer = memo(function GraphvizRenderer({code}: GraphvizRendererProps) {
    const containerRef = useRef<HTMLDivElement>(null);
    const [svg, setSvg] = useState<string | null>(null);
    const [error, setError] = useState<string | null>(null);

    useEffect(() => {
        let cancelled = false;
        setSvg(null);
        setError(null);

        const renderDiagram = async () => {
            try {
                const graphviz = await loadGraphviz();
                const rendered = graphviz.dot(code);
                if (cancelled) {
                    return;
                }
                setSvg(rendered);
                setError(null);
            } catch (err: unknown) {
                console.warn("[graphviz] failed to render diagram", err);
                if (cancelled) {
                    return;
                }
                setSvg(null);
                setError(toErrorMessage(err, "Failed to render Graphviz diagram."));
            }
        };

        void renderDiagram();

        return () => {
            cancelled = true;
        };
    }, [code]);

    useEffect(() => {
        const container = containerRef.current;
        if (!container) {
            return;
        }
        if (svg === null) {
            container.replaceChildren();
            return;
        }
        const svgElement = parseSvgDocument(svg);
        if (svgElement === null) {
            container.replaceChildren();
            setError("Graphviz returned invalid SVG output.");
            return;
        }
        container.replaceChildren(svgElement);
    }, [svg]);

    if (error) {
        return <div className="file-content-empty">{error}</div>;
    }

    if (svg === null) {
        return <div className="file-content-empty">Rendering Graphviz diagram...</div>;
    }

    return <div ref={containerRef} className="file-view-graphviz"/>;
});
