import {memo, useEffect, useId, useMemo, useRef, useState} from "react";
import {toErrorMessage} from "../../../../../utils/errorUtils";
import {createRetriableAsyncSingleton} from "../../../../../utils/createRetriableAsyncSingleton";

type WavedromApi = {
    renderWaveElement: (
        index: number,
        source: object,
        outputElement: Element,
        waveSkin: Record<string, unknown>,
    ) => void;
    waveSkin: Record<string, unknown>;
};

interface WavedromRendererProps {
    readonly code: string;
}

const loadWavedrom = createRetriableAsyncSingleton(async (): Promise<WavedromApi> => {
    const mod = await import("wavedrom");
    const api = (mod.default ?? mod) as Partial<WavedromApi>;
    if (typeof api.renderWaveElement !== "function" || typeof api.waveSkin !== "object" || api.waveSkin === null) {
        throw new Error("wavedrom module missing renderWaveElement or waveSkin");
    }
    return api as WavedromApi;
});

function hashRenderIndex(value: string): number {
    let hash = 0;
    for (const character of value) {
        hash = ((hash << 5) - hash + character.charCodeAt(0)) | 0;
    }
    return (Math.abs(hash) % 1_000_000) + 1;
}

async function parseSource(code: string): Promise<object> {
    const trimmed = code.trim();
    if (trimmed === "") {
        throw new Error("WaveDrom source is empty.");
    }
    // WaveDrom diagrams use JSON5-style relaxed syntax (unquoted keys,
    // single quotes, trailing commas). json5 parses this without eval.
    const {default: JSON5} = await import("json5");
    const parsed = JSON5.parse(trimmed) as unknown;
    if (typeof parsed !== "object" || parsed === null) {
        throw new Error("WaveDrom source must be a JSON/JSON5 object.");
    }
    return parsed;
}

export const WavedromRenderer = memo(function WavedromRenderer({code}: WavedromRendererProps) {
    const reactId = useId();
    const renderIndex = useMemo(() => hashRenderIndex(reactId), [reactId]);
    const containerRef = useRef<HTMLDivElement>(null);
    const [isReady, setIsReady] = useState(false);
    const [error, setError] = useState<string | null>(null);

    useEffect(() => {
        let cancelled = false;
        setIsReady(false);
        setError(null);

        const renderDiagram = async () => {
            try {
                const [wavedrom, source] = await Promise.all([
                    loadWavedrom(),
                    parseSource(code),
                ]);
                if (cancelled) {
                    return;
                }
                const container = containerRef.current;
                if (!container) {
                    return;
                }
                wavedrom.renderWaveElement(renderIndex, source, container, wavedrom.waveSkin);
                if (cancelled) {
                    return;
                }
                setIsReady(true);
            } catch (err: unknown) {
                console.warn("[wavedrom] failed to render diagram", err);
                if (cancelled) {
                    return;
                }
                setError(toErrorMessage(err, "Failed to render WaveDrom diagram."));
            }
        };

        void renderDiagram();

        return () => {
            cancelled = true;
        };
    }, [code, renderIndex]);

    if (error) {
        return <div className="file-content-empty">{error}</div>;
    }

    return (
        <div className="file-view-wavedrom">
            {!isReady ? <div className="file-content-empty">Rendering WaveDrom diagram...</div> : null}
            <div ref={containerRef} className="file-view-wavedrom-host"/>
        </div>
    );
});
