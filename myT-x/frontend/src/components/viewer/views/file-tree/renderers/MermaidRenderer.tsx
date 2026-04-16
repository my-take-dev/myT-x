import {memo, useEffect, useId, useMemo, useRef, useState} from "react";
import {toErrorMessage} from "../../../../../utils/errorUtils";

type MermaidModule = typeof import("mermaid");
type MermaidAPI = MermaidModule["default"];

interface MermaidRendererProps {
    readonly code: string;
}

interface MermaidRenderResult {
    readonly svg: string;
    readonly bindFunctions?: (element: Element) => void;
}

let mermaidModulePromise: Promise<MermaidAPI> | null = null;

async function loadMermaid(): Promise<MermaidAPI> {
    if (mermaidModulePromise === null) {
        mermaidModulePromise = import("mermaid").then(({default: mermaid}) => {
            mermaid.initialize({
                startOnLoad: false,
            });
            return mermaid;
        });
    }
    return mermaidModulePromise;
}

export const MermaidRenderer = memo(function MermaidRenderer({code}: MermaidRendererProps) {
    const reactId = useId();
    const renderId = useMemo(
        () => `file-view-mermaid-${reactId.replace(/[^a-zA-Z0-9_-]/g, "")}`,
        [reactId],
    );
    const containerRef = useRef<HTMLDivElement>(null);
    const [svg, setSvg] = useState<string | null>(null);
    const [error, setError] = useState<string | null>(null);
    const [bindFunctions, setBindFunctions] = useState<MermaidRenderResult["bindFunctions"] | null>(null);

    useEffect(() => {
        let cancelled = false;
        setSvg(null);
        setError(null);
        setBindFunctions(null);

        const renderDiagram = async () => {
            try {
                const mermaid = await loadMermaid();
                const renderResult = await mermaid.render(renderId, code) as MermaidRenderResult;
                if (cancelled) {
                    return;
                }
                setSvg(renderResult.svg);
                setError(null);
                setBindFunctions(() => renderResult.bindFunctions ?? null);
            } catch (err: unknown) {
                console.warn("[mermaid] failed to render diagram", err);
                if (cancelled) {
                    return;
                }
                setSvg(null);
                setError(toErrorMessage(err, "Failed to render Mermaid diagram."));
                setBindFunctions(null);
            }
        };

        void renderDiagram();

        return () => {
            cancelled = true;
        };
    }, [code, renderId]);

    useEffect(() => {
        if (!svg || !bindFunctions || !containerRef.current) {
            return;
        }
        bindFunctions(containerRef.current);
    }, [bindFunctions, svg]);

    if (error) {
        return <div className="file-content-empty">{error}</div>;
    }

    if (svg === null) {
        return <div className="file-content-empty">Rendering Mermaid diagram...</div>;
    }

    return (
        <div
            ref={containerRef}
            className="file-view-mermaid"
            dangerouslySetInnerHTML={{__html: svg}}
        />
    );
});
