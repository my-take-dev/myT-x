import {memo} from "react";
import type {ListChildComponentProps} from "react-window";
import {ChevronIcon} from "../../icons/ChevronIcon";
import type {OperationType, StagingListItem} from "./sourceControlTypes";

interface StagingRowData {
    items: readonly StagingListItem[];
    selectedPath: string | null;
    operationInFlight: OperationType;
    onSelectFile: (path: string) => void;
    onStageFile: (path: string) => void;
    onUnstageFile: (path: string) => void;
    onDiscardFile: (path: string) => void;
    onToggleGroup: (group: "staged" | "unstaged") => void;
    onBatchAction: (group: "staged" | "unstaged") => void;
}

function getStatusBadge(status: string): {label: string; color: string} {
    switch (status) {
        case "added":
        case "untracked":
            return {label: status === "untracked" ? "?" : "A", color: "var(--git-staged)"};
        case "deleted":
            return {label: "D", color: "var(--danger)"};
        case "renamed":
            return {label: "R", color: "var(--warning, var(--fg-main))"};
        default:
            return {label: "M", color: "var(--fg-main)"};
    }
}

export const StagingRow = memo(function StagingRow({
    index,
    style,
    data,
}: ListChildComponentProps<StagingRowData>) {
    const item = data.items[index];
    if (!item) return null;

    if (item.type === "group-header") {
        const batchLabel = item.group === "staged" ? "Unstage All" : "Stage All";
        const isDisabled = data.operationInFlight !== null;
        return (
            <div
                className={`staging-group-header staging-group-header--${item.group}`}
                style={style}
                role="button"
                tabIndex={0}
                aria-expanded={item.isExpanded}
                onClick={() => data.onToggleGroup(item.group)}
                onKeyDown={(e) => {
                    if (e.key === "Enter" || e.key === " ") {
                        e.preventDefault();
                        data.onToggleGroup(item.group);
                    }
                }}
            >
                <span className={`staging-group-arrow${item.isExpanded ? " expanded" : ""}`}>
                    <ChevronIcon size={8} />
                </span>
                <span className="staging-group-label">
                    {item.group === "staged" ? "Staged Changes" : "Changes"}
                </span>
                <span className="staging-group-count">
                    ({item.count})
                </span>
                {item.count > 0 && (
                    <button
                        type="button"
                        className="staging-group-batch-btn"
                        disabled={isDisabled}
                        onClick={(e) => {
                            e.stopPropagation();
                            data.onBatchAction(item.group);
                        }}
                    >
                        {batchLabel}
                    </button>
                )}
            </div>
        );
    }

    // File row.
    const {file, group} = item;
    const isSelected = data.selectedPath === file.path;
    const badge = getStatusBadge(file.status);
    const isDisabled = data.operationInFlight !== null;

    return (
        <div
            className={`staging-file-row${isSelected ? " selected" : ""}`}
            style={style}
            role="treeitem"
            tabIndex={0}
            aria-selected={isSelected}
            onClick={() => data.onSelectFile(file.path)}
        >
            <span className="staging-file-badge" style={{color: badge.color}}>
                {badge.label}
            </span>
            <span className="staging-file-path" title={file.path}>
                {file.path}
            </span>
            <span className="staging-file-actions">
                {group === "unstaged" ? (
                    <button
                        type="button"
                        className="staging-action-btn staging-action-stage"
                        title="Stage"
                        disabled={isDisabled}
                        onClick={(e) => {
                            e.stopPropagation();
                            data.onStageFile(file.path);
                        }}
                    >
                        +
                    </button>
                ) : (
                    <button
                        type="button"
                        className="staging-action-btn staging-action-unstage"
                        title="Unstage"
                        disabled={isDisabled}
                        onClick={(e) => {
                            e.stopPropagation();
                            data.onUnstageFile(file.path);
                        }}
                    >
                        &minus;
                    </button>
                )}
                <button
                    type="button"
                    className="staging-action-btn staging-action-discard"
                    title="Discard changes"
                    disabled={isDisabled}
                    onClick={(e) => {
                        e.stopPropagation();
                        data.onDiscardFile(file.path);
                    }}
                >
                    &times;
                </button>
            </span>
        </div>
    );
}, (prev, next) => {
    if (prev.index !== next.index || prev.style !== next.style) return false;
    const pd = prev.data;
    const nd = next.data;
    const prevItem = pd.items[prev.index];
    const nextItem = nd.items[next.index];
    if (prevItem !== nextItem) return false;
    if (prevItem?.type === "file" && nextItem?.type === "file") {
        const wasSelected = pd.selectedPath === prevItem.file.path;
        const isSelected = nd.selectedPath === nextItem.file.path;
        if (wasSelected !== isSelected) return false;
    }
    return pd.operationInFlight === nd.operationInFlight;
});

export type {StagingRowData};
