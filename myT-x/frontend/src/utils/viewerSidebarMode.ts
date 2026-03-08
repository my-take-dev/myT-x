export type ViewerSidebarMode = "overlay" | "docked";

export const DEFAULT_VIEWER_SIDEBAR_MODE: ViewerSidebarMode = "overlay";

export function isViewerSidebarMode(mode: string | null | undefined): mode is ViewerSidebarMode {
    return mode === "overlay" || mode === "docked";
}

export function normalizeViewerSidebarMode(mode: string | null | undefined): ViewerSidebarMode {
    if (!isViewerSidebarMode(mode)) {
        return DEFAULT_VIEWER_SIDEBAR_MODE;
    }
    return mode;
}

export function serializeViewerSidebarMode(mode: ViewerSidebarMode | "" | null | undefined): ViewerSidebarMode | undefined {
    if (mode == null || mode === "") {
        return undefined;
    }
    return mode;
}
