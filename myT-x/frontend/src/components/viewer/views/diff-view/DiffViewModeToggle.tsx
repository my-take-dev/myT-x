import type {DiffSidebarMode} from "./sourceControlTypes";

interface DiffViewModeToggleProps {
    mode: DiffSidebarMode;
    onModeChange: (mode: DiffSidebarMode) => void;
}

export function DiffViewModeToggle({mode, onModeChange}: DiffViewModeToggleProps) {
    return (
        <span className="diff-mode-toggle">
            <button
                type="button"
                className={`diff-mode-btn${mode === "tree" ? " active" : ""}`}
                aria-label="Tree view"
                title="Tree view"
                onClick={() => onModeChange("tree")}
            >
                Tree
            </button>
            <button
                type="button"
                className={`diff-mode-btn${mode === "flat" ? " active" : ""}`}
                aria-label="Source control view"
                title="Source control view (Stage / Commit / Push)"
                onClick={() => onModeChange("flat")}
            >
                Git
            </button>
        </span>
    );
}
