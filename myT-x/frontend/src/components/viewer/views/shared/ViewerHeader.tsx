import type {ReactNode} from "react";

interface ViewerHeaderProps {
    title: string;
    onClose: () => void;
    onRefresh?: () => void;
    refreshTitle?: string;
    children?: ReactNode;
}

export function ViewerHeader({title, onClose, onRefresh, refreshTitle = "Refresh", children}: ViewerHeaderProps) {
    return (
        <div className="viewer-header">
            <h2 className="viewer-header-title">{title}</h2>
            {children}
            <div className="viewer-header-spacer"/>
            {onRefresh && (
                <button type="button" className="viewer-header-btn" onClick={onRefresh} title={refreshTitle} aria-label={refreshTitle}>
                    {"\u21BB"}
                </button>
            )}
            <button type="button" className="viewer-header-btn" onClick={onClose} title="Close" aria-label="Close">
                {"\u2715"}
            </button>
        </div>
    );
}
