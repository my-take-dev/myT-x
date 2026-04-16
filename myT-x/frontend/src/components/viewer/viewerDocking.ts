import type {ViewerSidebarMode} from "../../utils/viewerSidebarMode";

// 1440px is the point where a typical 1080p-class desktop still keeps the
// terminal pane and docked viewer readable without shrinking either pane below
// its intended baseline size.
export const DOCKED_LAYOUT_BASE_WIDTH = 1440;
// Mirrors the Wails window minimum in main.go so frontend layout math matches runtime constraints.
export const DOCKED_WINDOW_MIN_WIDTH = 980;

// Docked layout widths expressed in the layout's unscaled coordinate space.
export const DOCKED_SIDEBAR_WIDTH = 280;
// The ActivityStrip stays portaled to document.body. This constant therefore
// describes the strip's visible viewport width, while the reserved dock gap is
// derived from the current scale factor.
export const DOCKED_ACTIVITY_STRIP_WIDTH = 36;
// Matches the CSS divider handle width.
export const DOCKED_DIVIDER_WIDTH = 4;
// Minimum unscaled viewer pane width; overrides the ratio-based allocation.
export const DOCKED_VIEWER_MIN_WIDTH = 320;
export const DOCKED_LAYOUT_FIXED_CHROME_WIDTH = DOCKED_SIDEBAR_WIDTH + DOCKED_DIVIDER_WIDTH;

// Clamp the main pane to 30%-80% of the docked content span.
// The viewer's minimum width floor can still override the requested ratio.
export const DOCK_RATIO_MIN = 0.3;
export const DOCK_RATIO_DEFAULT = 0.5;
export const DOCK_RATIO_MAX = 0.8;

export interface DockedLayoutMetrics {
    /** Unscaled docked layout width. Always >= DOCKED_LAYOUT_BASE_WIDTH. */
    readonly layoutWidth: number;
    /**
     * Visual scale applied to the unscaled layout.
     * Invariant: 0 < appScale <= 1 because computeDockedLayoutWidth() always
     * returns a layoutWidth that is >= every positive windowWidth.
     */
    readonly appScale: number;
    /**
     * Unscaled gap reserved inside the docked layout for the portaled
     * ActivityStrip. Equals DOCKED_ACTIVITY_STRIP_WIDTH (36px) when appScale is
     * 1 (window width >= 1440px), and grows as the app scales down so the strip
     * still occupies a 36px viewport column after scaling.
     */
    readonly activityStripReservedWidth: number;
    /**
     * Usable unscaled width after subtracting the sidebar, divider, and
     * ActivityStrip reserve from layoutWidth.
     */
    readonly contentWidth: number;
    /**
     * Visible content span after scale is applied. This is the viewport-space
     * denominator for divider dragging because the drag gesture itself is also
     * measured in viewport pixels.
     */
    readonly displayedContentWidth: number;
}

export interface DockedPaneWidths {
    /**
     * Main terminal pane width inside the docked content span.
     * @invariant mainWidth + viewerWidth === contentWidth passed to computeDockedPaneWidths()
     */
    readonly mainWidth: number;
    /** Viewer pane width inside the docked content span. */
    readonly viewerWidth: number;
}

export interface DockedLayout {
    readonly inverseAppScale: number;
    readonly isScaled: boolean;
    readonly metrics: DockedLayoutMetrics;
    readonly paneWidths: DockedPaneWidths;
}

export interface DockedCSSVariables {
    readonly "--dock-app-scale": string;
    readonly "--dock-activity-strip-reserved-width": string;
    readonly "--dock-app-unscaled-height": string;
    readonly "--dock-app-unscaled-width": string;
    readonly "--dock-main-width": string;
    readonly "--dock-viewer-width": string;
}

function warnInvalidInput(functionName: string, inputName: string, value: number, fallback: number) {
    if (import.meta.env.DEV) {
        console.warn(
            `[viewerDocking] ${functionName}: invalid ${inputName}; using fallback`,
            {fallback, value},
        );
    }
}

export function clampDockRatio(ratio: number): number {
    if (!Number.isFinite(ratio)) {
        return DOCK_RATIO_DEFAULT;
    }
    return Math.max(DOCK_RATIO_MIN, Math.min(DOCK_RATIO_MAX, ratio));
}

export function normalizeDockedViewportWidth(windowWidth: number): number {
    if (!Number.isFinite(windowWidth) || windowWidth <= 0) {
        warnInvalidInput(
            "normalizeDockedViewportWidth",
            "windowWidth",
            windowWidth,
            DOCKED_WINDOW_MIN_WIDTH,
        );
        return DOCKED_WINDOW_MIN_WIDTH;
    }
    return Math.max(windowWidth, DOCKED_WINDOW_MIN_WIDTH);
}

export function computeDockedLayoutWidth(windowWidth: number): number {
    if (!Number.isFinite(windowWidth) || windowWidth <= 0) {
        warnInvalidInput(
            "computeDockedLayoutWidth",
            "windowWidth",
            windowWidth,
            DOCKED_LAYOUT_BASE_WIDTH,
        );
        return DOCKED_LAYOUT_BASE_WIDTH;
    }
    return Math.max(windowWidth, DOCKED_LAYOUT_BASE_WIDTH);
}

export function computeDockedActivityStripReservedWidth(appScale: number): number {
    const safeAppScale = Number.isFinite(appScale) && appScale > 0 ? appScale : 1;
    if (safeAppScale !== appScale) {
        warnInvalidInput(
            "computeDockedActivityStripReservedWidth",
            "appScale",
            appScale,
            1,
        );
    }
    return DOCKED_ACTIVITY_STRIP_WIDTH / safeAppScale;
}

export function computeDockedLayoutMetrics(windowWidth: number): DockedLayoutMetrics {
    const hasPositiveWindowWidth = Number.isFinite(windowWidth) && windowWidth > 0;
    if (!hasPositiveWindowWidth) {
        warnInvalidInput(
            "computeDockedLayoutMetrics",
            "windowWidth",
            windowWidth,
            DOCKED_LAYOUT_BASE_WIDTH,
        );
    }
    const layoutWidth = hasPositiveWindowWidth
        ? Math.max(windowWidth, DOCKED_LAYOUT_BASE_WIDTH)
        : DOCKED_LAYOUT_BASE_WIDTH;
    const appScale = hasPositiveWindowWidth ? windowWidth / layoutWidth : 1;
    const activityStripReservedWidth = computeDockedActivityStripReservedWidth(appScale);
    const contentWidth = Math.max(
        layoutWidth - DOCKED_LAYOUT_FIXED_CHROME_WIDTH - activityStripReservedWidth,
        0,
    );
    return {
        layoutWidth,
        appScale,
        activityStripReservedWidth,
        contentWidth,
        displayedContentWidth: hasPositiveWindowWidth ? contentWidth * appScale : 0,
    };
}

/**
 * @invariant mainWidth + viewerWidth === safeContentWidth
 * Proof: viewerWidth = max(requested, min(MIN, total)), so viewerWidth <= total.
 * mainWidth = max(total - viewerWidth, 0) = total - viewerWidth because viewerWidth <= total.
 */
export function computeDockedPaneWidths(contentWidth: number, dockRatio: number): DockedPaneWidths {
    const safeContentWidth = Number.isFinite(contentWidth) && contentWidth > 0 ? contentWidth : 0;
    if (safeContentWidth !== contentWidth) {
        warnInvalidInput(
            "computeDockedPaneWidths",
            "contentWidth",
            contentWidth,
            0,
        );
    }
    const clampedDockRatio = clampDockRatio(dockRatio);
    const requestedViewerWidth = safeContentWidth * (1 - clampedDockRatio);
    const minimumViewerWidth = Math.min(DOCKED_VIEWER_MIN_WIDTH, safeContentWidth);
    const viewerWidth = Math.max(requestedViewerWidth, minimumViewerWidth);
    return {
        mainWidth: Math.max(safeContentWidth - viewerWidth, 0),
        viewerWidth,
    };
}

export function buildDockedLayout(windowWidth: number, dockRatio: number): DockedLayout {
    const metrics = computeDockedLayoutMetrics(windowWidth);
    const paneWidths = computeDockedPaneWidths(metrics.contentWidth, dockRatio);
    const inverseAppScale = metrics.appScale > 0 ? 1 / metrics.appScale : 1;
    return {
        inverseAppScale,
        isScaled: metrics.appScale < 1,
        metrics,
        paneWidths,
    };
}

export function buildDockedCssVariables(layout: DockedLayout): DockedCSSVariables {
    const safeAppScale = Number.isFinite(layout.metrics.appScale) && layout.metrics.appScale > 0
        ? layout.metrics.appScale
        : 1;
    const safeInverseAppScale = Number.isFinite(layout.inverseAppScale) && layout.inverseAppScale > 0
        ? layout.inverseAppScale
        : 1;
    if (safeAppScale !== layout.metrics.appScale && import.meta.env.DEV) {
        console.error(
            "[viewerDocking] buildDockedCssVariables: invalid appScale",
            layout.metrics.appScale,
        );
    }
    if (safeInverseAppScale !== layout.inverseAppScale && import.meta.env.DEV) {
        console.error(
            "[viewerDocking] buildDockedCssVariables: invalid inverseAppScale",
            layout.inverseAppScale,
        );
    }
    return {
        "--dock-app-scale": `${safeAppScale}`,
        "--dock-activity-strip-reserved-width": `${layout.metrics.activityStripReservedWidth}px`,
        "--dock-app-unscaled-height": `${safeInverseAppScale * 100}%`,
        "--dock-app-unscaled-width": `${safeInverseAppScale * 100}%`,
        "--dock-main-width": `${layout.paneWidths.mainWidth}px`,
        "--dock-viewer-width": `${layout.paneWidths.viewerWidth}px`,
    };
}

export function isViewerDocked(sidebarMode: ViewerSidebarMode, activeViewId: string | null): boolean {
    return sidebarMode === "docked" && activeViewId !== null;
}
