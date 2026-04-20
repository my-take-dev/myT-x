import {Suspense, type ReactNode} from "react";
import {RendererErrorBoundary} from "./RendererErrorBoundary";

interface RendererSurfaceProps {
    readonly children: ReactNode;
    readonly fallback?: ReactNode;
    readonly filePath?: string;
    readonly loadingMessage: string;
    readonly rendererName: string;
}

export function RendererSurface({
    children,
    fallback,
    filePath,
    loadingMessage,
    rendererName,
}: RendererSurfaceProps) {
    return (
        <RendererErrorBoundary filePath={filePath} rendererName={rendererName}>
            <Suspense fallback={fallback ?? <div className="file-content-empty">{loadingMessage}</div>}>
                {children}
            </Suspense>
        </RendererErrorBoundary>
    );
}
