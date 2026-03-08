import type {ViewerSidebarMode} from "../../utils/viewerSidebarMode";

// Keep both panes usable at the 980px minimum window width enforced by main.go.
export const DOCK_RATIO_MIN = 0.3;
export const DOCK_RATIO_DEFAULT = 0.5;
export const DOCK_RATIO_MAX = 0.8;

export function clampDockRatio(ratio: number): number {
    if (!Number.isFinite(ratio)) {
        return DOCK_RATIO_DEFAULT;
    }
    return Math.max(DOCK_RATIO_MIN, Math.min(DOCK_RATIO_MAX, ratio));
}

export function isViewerDocked(sidebarMode: ViewerSidebarMode, activeViewId: string | null): boolean {
    return sidebarMode === "docked" && activeViewId !== null;
}
