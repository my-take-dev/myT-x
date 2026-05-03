import type {ReactNode} from "react";
import {useI18n} from "../../../../i18n";

interface ViewerHeaderProps {
    title: string;
    onClose: () => void;
    onRefresh?: () => void;
    refreshDisabled?: boolean;
    refreshTitle?: string;
    children?: ReactNode;
}

export function ViewerHeader({title, onClose, onRefresh, refreshDisabled = false, refreshTitle, children}: ViewerHeaderProps) {
    const {t} = useI18n();
    const resolvedRefreshTitle = refreshTitle ?? t("viewer.common.refresh", "更新");
    const closeTitle = t("common.action.close", "閉じる");
    return (
        <div className="viewer-header">
            <h2 className="viewer-header-title">{title}</h2>
            {children}
            <div className="viewer-header-spacer"/>
            {onRefresh && (
                <button
                    type="button"
                    className="viewer-header-btn"
                    onClick={onRefresh}
                    disabled={refreshDisabled}
                    title={resolvedRefreshTitle}
                    aria-label={resolvedRefreshTitle}
                >
                    {"\u21BB"}
                </button>
            )}
            <button type="button" className="viewer-header-btn" onClick={onClose} title={closeTitle} aria-label={closeTitle}>
                {"\u2715"}
            </button>
        </div>
    );
}
