import React from "react";
import {toErrorMessage} from "../../../../../utils/errorUtils";

interface RendererErrorBoundaryProps {
    readonly children: React.ReactNode;
    readonly filePath?: string;
    readonly rendererName: string;
}

interface RendererErrorBoundaryState {
    readonly message: string | null;
}

export class RendererErrorBoundary extends React.Component<
    RendererErrorBoundaryProps,
    RendererErrorBoundaryState
> {
    constructor(props: RendererErrorBoundaryProps) {
        super(props);
        this.state = {message: null};
    }

    static getDerivedStateFromError(error: unknown): RendererErrorBoundaryState {
        return {
            message: toErrorMessage(error, "Failed to load renderer."),
        };
    }

    componentDidCatch(error: unknown): void {
        console.warn(`[${this.props.rendererName.toLowerCase()}] renderer surface crashed`, error);
    }

    render(): React.ReactNode {
        if (this.state.message === null) {
            return this.props.children;
        }

        const fileContext = this.props.filePath ? ` (${this.props.filePath})` : "";
        return (
            <div className="file-content-empty">
                {`Failed to load ${this.props.rendererName} preview${fileContext}: ${this.state.message}`}
            </div>
        );
    }
}
