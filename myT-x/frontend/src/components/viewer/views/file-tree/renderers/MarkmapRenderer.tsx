import {memo, useEffect, useRef, useState} from "react";
import {toErrorMessage} from "../../../../../utils/errorUtils";
import {createRetriableAsyncSingleton} from "../../../../../utils/createRetriableAsyncSingleton";

type MarkmapLibModule = typeof import("markmap-lib");
type MarkmapViewModule = typeof import("markmap-view");
type TransformerInstance = InstanceType<MarkmapLibModule["Transformer"]>;

interface MarkmapModules {
    readonly transformer: TransformerInstance;
    readonly view: MarkmapViewModule;
}

interface MarkmapRendererProps {
    readonly code: string;
}

const MARKMAP_VIEW_OPTIONS = {
    autoFit: true,
    // Two levels keep large documents scannable without collapsing the main structure.
    initialExpandLevel: 2,
    // 240px preserves readable node labels without letting long paragraphs dominate the first view.
    maxWidth: 240,
} as const;

const loadMarkmap = createRetriableAsyncSingleton(async (): Promise<MarkmapModules> => {
    const [libModule, viewModule] = await Promise.all([
        import("markmap-lib"),
        import("markmap-view"),
    ]);
    return {
        transformer: new libModule.Transformer(),
        view: viewModule,
    };
});

export const MarkmapRenderer = memo(function MarkmapRenderer({code}: MarkmapRendererProps) {
    const svgRef = useRef<SVGSVGElement>(null);
    const [isReady, setIsReady] = useState(false);
    const [error, setError] = useState<string | null>(null);

    useEffect(() => {
        let cancelled = false;
        let activeInstance: {destroy: () => void} | null = null;
        setIsReady(false);
        setError(null);

        const renderDiagram = async () => {
            try {
                const {transformer, view} = await loadMarkmap();
                if (cancelled) {
                    return;
                }
                const svg = svgRef.current;
                if (!svg) {
                    return;
                }
                while (svg.firstChild !== null) {
                    svg.removeChild(svg.firstChild);
                }
                const {root} = transformer.transform(code);
                const instance = view.Markmap.create(svg, MARKMAP_VIEW_OPTIONS);
                if (cancelled) {
                    instance.destroy();
                    return;
                }
                activeInstance = instance;
                await instance.setData(root);
                if (cancelled) {
                    instance.destroy();
                    return;
                }
                await instance.fit();
                if (cancelled) {
                    return;
                }
                setIsReady(true);
            } catch (err: unknown) {
                console.warn("[markmap] failed to render diagram", err);
                if (cancelled) {
                    return;
                }
                setError(toErrorMessage(err, "Failed to render Markmap diagram."));
            }
        };

        void renderDiagram();

        return () => {
            cancelled = true;
            if (activeInstance !== null) {
                activeInstance.destroy();
                activeInstance = null;
            }
        };
    }, [code]);

    if (error) {
        return <div className="file-content-empty">{error}</div>;
    }

    return (
        <div className="file-view-markmap">
            {!isReady ? <div className="file-content-empty">Rendering Markmap diagram...</div> : null}
            <svg ref={svgRef} className="file-view-markmap-svg"/>
        </div>
    );
});
