import {readFileSync} from "node:fs";
import {resolve} from "node:path";
import {beforeEach, describe, expect, it, vi} from "vitest";
import {useViewerStore} from "../src/components/viewer/viewerStore";
import {
    buildDockedLayout,
    buildDockedCssVariables,
    clampDockRatio,
    computeDockedActivityStripReservedWidth,
    computeDockedLayoutWidth,
    computeDockedLayoutMetrics,
    computeDockedPaneWidths,
    type DockedLayout,
    DOCKED_ACTIVITY_STRIP_WIDTH,
    DOCKED_DIVIDER_WIDTH,
    DOCKED_LAYOUT_BASE_WIDTH,
    DOCKED_LAYOUT_FIXED_CHROME_WIDTH,
    DOCKED_SIDEBAR_WIDTH,
    DOCKED_WINDOW_MIN_WIDTH,
    DOCK_RATIO_DEFAULT,
    DOCK_RATIO_MAX,
    DOCK_RATIO_MIN,
    DOCKED_VIEWER_MIN_WIDTH,
    isViewerDocked,
    normalizeDockedViewportWidth,
} from "../src/components/viewer/viewerDocking";

function resetViewerStore() {
    useViewerStore.setState({
        activeViewId: null,
        dockRatio: DOCK_RATIO_DEFAULT,
    });
}

describe("viewerDocking", () => {
    beforeEach(() => {
        resetViewerStore();
    });

    it("clamps dock ratios and falls back for non-finite inputs", () => {
        const cases = [
            {input: DOCK_RATIO_MIN - 0.1, expected: DOCK_RATIO_MIN},
            {input: DOCK_RATIO_DEFAULT, expected: DOCK_RATIO_DEFAULT},
            {input: DOCK_RATIO_MAX + 0.1, expected: DOCK_RATIO_MAX},
            {input: Number.NaN, expected: DOCK_RATIO_DEFAULT},
            {input: Number.POSITIVE_INFINITY, expected: DOCK_RATIO_DEFAULT},
            {input: Number.NEGATIVE_INFINITY, expected: DOCK_RATIO_DEFAULT},
        ];

        for (const testCase of cases) {
            expect(clampDockRatio(testCase.input)).toBe(testCase.expected);
        }
    });

    it("normalizes transient viewport widths to the runtime minimum", () => {
        const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
        warnSpy.mockClear();
        expect(normalizeDockedViewportWidth(1200)).toBe(1200);
        expect(normalizeDockedViewportWidth(DOCKED_WINDOW_MIN_WIDTH)).toBe(DOCKED_WINDOW_MIN_WIDTH);
        expect(normalizeDockedViewportWidth(600)).toBe(DOCKED_WINDOW_MIN_WIDTH);
        expect(normalizeDockedViewportWidth(-100)).toBe(DOCKED_WINDOW_MIN_WIDTH);
        expect(normalizeDockedViewportWidth(0)).toBe(DOCKED_WINDOW_MIN_WIDTH);
        expect(normalizeDockedViewportWidth(Number.NaN)).toBe(DOCKED_WINDOW_MIN_WIDTH);
        expect(normalizeDockedViewportWidth(Number.POSITIVE_INFINITY)).toBe(DOCKED_WINDOW_MIN_WIDTH);
        expect(normalizeDockedViewportWidth(Number.NEGATIVE_INFINITY)).toBe(DOCKED_WINDOW_MIN_WIDTH);
        expect(warnSpy).toHaveBeenCalledTimes(5);
        for (const [message] of warnSpy.mock.calls) {
            expect(message).toBe("[viewerDocking] normalizeDockedViewportWidth: invalid windowWidth; using fallback");
        }
    });

    it("treats the viewer as docked only when docked mode has an active view", () => {
        expect(isViewerDocked("overlay", "git-graph")).toBe(false);
        expect(isViewerDocked("docked", null)).toBe(false);
        expect(isViewerDocked("docked", "git-graph")).toBe(true);
    });

    it("shrinks docked app content when the window is narrower than the base layout width", () => {
        expect(computeDockedLayoutWidth(DOCKED_LAYOUT_BASE_WIDTH)).toBe(DOCKED_LAYOUT_BASE_WIDTH);
        expect(computeDockedLayoutWidth(1600)).toBe(1600);
        expect(computeDockedLayoutWidth(1200)).toBe(DOCKED_LAYOUT_BASE_WIDTH);
        expect(computeDockedLayoutMetrics(DOCKED_LAYOUT_BASE_WIDTH).appScale).toBe(1);
        expect(computeDockedLayoutMetrics(1600).appScale).toBe(1);
        expect(computeDockedLayoutMetrics(1200).appScale).toBeCloseTo(1200 / DOCKED_LAYOUT_BASE_WIDTH);
        expect(computeDockedLayoutMetrics(DOCKED_WINDOW_MIN_WIDTH).appScale).toBeCloseTo(
            DOCKED_WINDOW_MIN_WIDTH / DOCKED_LAYOUT_BASE_WIDTH,
        );
    });

    it("sizes docked panes from the layout width after subtracting fixed chrome", () => {
        const wideContentWidth = 1600 - DOCKED_LAYOUT_FIXED_CHROME_WIDTH - DOCKED_ACTIVITY_STRIP_WIDTH;
        expect(computeDockedPaneWidths(wideContentWidth, DOCK_RATIO_DEFAULT)).toEqual({
            mainWidth: 640,
            viewerWidth: 640,
        });

        const baseContentWidth = DOCKED_LAYOUT_BASE_WIDTH - DOCKED_LAYOUT_FIXED_CHROME_WIDTH - DOCKED_ACTIVITY_STRIP_WIDTH;
        expect(computeDockedPaneWidths(baseContentWidth, DOCK_RATIO_DEFAULT)).toEqual({
            mainWidth: 560,
            viewerWidth: 560,
        });
    });

    it("respects the minimum and maximum dock-ratio boundaries", () => {
        const contentWidth = DOCKED_LAYOUT_BASE_WIDTH - DOCKED_LAYOUT_FIXED_CHROME_WIDTH - DOCKED_ACTIVITY_STRIP_WIDTH;
        expect(computeDockedPaneWidths(contentWidth, DOCK_RATIO_MIN)).toEqual({
            mainWidth: 336,
            viewerWidth: 784,
        });

        const maxWidths = computeDockedPaneWidths(contentWidth, DOCK_RATIO_MAX);
        expect(maxWidths.viewerWidth).toBe(DOCKED_VIEWER_MIN_WIDTH);
        expect(maxWidths.mainWidth + maxWidths.viewerWidth).toBe(contentWidth);
    });

    it("keeps the docked viewer at the minimum usable width when ratios get too aggressive", () => {
        const contentWidth = DOCKED_LAYOUT_BASE_WIDTH - DOCKED_LAYOUT_FIXED_CHROME_WIDTH - DOCKED_ACTIVITY_STRIP_WIDTH;
        const widths = computeDockedPaneWidths(contentWidth, DOCK_RATIO_MAX);
        expect(widths.viewerWidth).toBe(DOCKED_VIEWER_MIN_WIDTH);
        expect(widths.mainWidth + widths.viewerWidth).toBe(contentWidth);
    });

    it("expands the unscaled dock reserve so the portaled strip stays 36px wide in the viewport", () => {
        const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
        warnSpy.mockClear();
        expect(computeDockedActivityStripReservedWidth(1)).toBe(DOCKED_ACTIVITY_STRIP_WIDTH);
        expect(computeDockedActivityStripReservedWidth(1200 / DOCKED_LAYOUT_BASE_WIDTH)).toBeCloseTo(
            DOCKED_ACTIVITY_STRIP_WIDTH / (1200 / DOCKED_LAYOUT_BASE_WIDTH),
        );
        expect(computeDockedActivityStripReservedWidth(0)).toBe(DOCKED_ACTIVITY_STRIP_WIDTH);
        expect(computeDockedActivityStripReservedWidth(-1)).toBe(DOCKED_ACTIVITY_STRIP_WIDTH);
        expect(computeDockedActivityStripReservedWidth(Number.NaN)).toBe(DOCKED_ACTIVITY_STRIP_WIDTH);
        expect(computeDockedActivityStripReservedWidth(Number.POSITIVE_INFINITY)).toBe(DOCKED_ACTIVITY_STRIP_WIDTH);
        expect(warnSpy).toHaveBeenCalledTimes(4);
        for (const [message] of warnSpy.mock.calls) {
            expect(message).toBe("[viewerDocking] computeDockedActivityStripReservedWidth: invalid appScale; using fallback");
        }
    });

    it("reports the visible docked content span after layout scaling", () => {
        const wideMetrics = computeDockedLayoutMetrics(1600);
        const wideReservedWidth = DOCKED_ACTIVITY_STRIP_WIDTH;
        expect(wideMetrics).toEqual({
            layoutWidth: 1600,
            appScale: 1,
            activityStripReservedWidth: wideReservedWidth,
            contentWidth: 1600 - DOCKED_LAYOUT_FIXED_CHROME_WIDTH - wideReservedWidth,
            displayedContentWidth: 1600 - DOCKED_LAYOUT_FIXED_CHROME_WIDTH - wideReservedWidth,
        });

        const scaledMetrics = computeDockedLayoutMetrics(1200);
        const scaledAppScale = 1200 / DOCKED_LAYOUT_BASE_WIDTH;
        const scaledReservedWidth = DOCKED_ACTIVITY_STRIP_WIDTH / scaledAppScale;
        expect(scaledMetrics.layoutWidth).toBe(DOCKED_LAYOUT_BASE_WIDTH);
        expect(scaledMetrics.appScale).toBeCloseTo(scaledAppScale);
        expect(scaledMetrics.activityStripReservedWidth).toBeCloseTo(scaledReservedWidth);
        expect(scaledMetrics.contentWidth).toBeCloseTo(
            DOCKED_LAYOUT_BASE_WIDTH - DOCKED_LAYOUT_FIXED_CHROME_WIDTH - scaledReservedWidth,
        );
        expect(scaledMetrics.displayedContentWidth).toBeCloseTo(
            (
                DOCKED_LAYOUT_BASE_WIDTH - DOCKED_LAYOUT_FIXED_CHROME_WIDTH - scaledReservedWidth
            ) * scaledAppScale,
        );
    });

    it("returns safe layout metrics for invalid window widths", () => {
        const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
        warnSpy.mockClear();
        expect(computeDockedLayoutMetrics(0)).toEqual({
            layoutWidth: DOCKED_LAYOUT_BASE_WIDTH,
            appScale: 1,
            activityStripReservedWidth: DOCKED_ACTIVITY_STRIP_WIDTH,
            contentWidth: DOCKED_LAYOUT_BASE_WIDTH - DOCKED_LAYOUT_FIXED_CHROME_WIDTH - DOCKED_ACTIVITY_STRIP_WIDTH,
            displayedContentWidth: 0,
        });

        expect(computeDockedLayoutMetrics(Number.NaN)).toEqual({
            layoutWidth: DOCKED_LAYOUT_BASE_WIDTH,
            appScale: 1,
            activityStripReservedWidth: DOCKED_ACTIVITY_STRIP_WIDTH,
            contentWidth: DOCKED_LAYOUT_BASE_WIDTH - DOCKED_LAYOUT_FIXED_CHROME_WIDTH - DOCKED_ACTIVITY_STRIP_WIDTH,
            displayedContentWidth: 0,
        });

        expect(computeDockedLayoutMetrics(-100)).toEqual({
            layoutWidth: DOCKED_LAYOUT_BASE_WIDTH,
            appScale: 1,
            activityStripReservedWidth: DOCKED_ACTIVITY_STRIP_WIDTH,
            contentWidth: DOCKED_LAYOUT_BASE_WIDTH - DOCKED_LAYOUT_FIXED_CHROME_WIDTH - DOCKED_ACTIVITY_STRIP_WIDTH,
            displayedContentWidth: 0,
        });
        expect(warnSpy).toHaveBeenCalledTimes(3);
        for (const [message] of warnSpy.mock.calls) {
            expect(message).toBe("[viewerDocking] computeDockedLayoutMetrics: invalid windowWidth; using fallback");
        }
    });

    it("returns safe pane widths for invalid content spans and ratios", () => {
        const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
        warnSpy.mockClear();
        expect(computeDockedPaneWidths(0, DOCK_RATIO_DEFAULT)).toEqual({
            mainWidth: 0,
            viewerWidth: 0,
        });
        expect(computeDockedPaneWidths(-500, DOCK_RATIO_DEFAULT)).toEqual({
            mainWidth: 0,
            viewerWidth: 0,
        });
        expect(computeDockedPaneWidths(Number.NaN, DOCK_RATIO_DEFAULT)).toEqual({
            mainWidth: 0,
            viewerWidth: 0,
        });
        expect(computeDockedPaneWidths(1600, Number.NaN)).toEqual({
            mainWidth: 800,
            viewerWidth: 800,
        });
        expect(computeDockedPaneWidths(1600, 0)).toEqual({
            mainWidth: 480,
            viewerWidth: 1120,
        });
        expect(computeDockedPaneWidths(1600, 1)).toEqual({
            mainWidth: 1280,
            viewerWidth: 320,
        });
        expect(warnSpy).toHaveBeenCalledTimes(2);
        for (const [message] of warnSpy.mock.calls) {
            expect(message).toBe("[viewerDocking] computeDockedPaneWidths: invalid contentWidth; using fallback");
        }
    });

    it("builds docked layouts from the shared layout factories", () => {
        const layout = buildDockedLayout(1200, DOCK_RATIO_DEFAULT);
        const appScale = 1200 / DOCKED_LAYOUT_BASE_WIDTH;
        const reservedWidth = DOCKED_ACTIVITY_STRIP_WIDTH / appScale;
        const contentWidth = DOCKED_LAYOUT_BASE_WIDTH - DOCKED_LAYOUT_FIXED_CHROME_WIDTH - reservedWidth;

        expect(layout).toEqual({
            inverseAppScale: DOCKED_LAYOUT_BASE_WIDTH / 1200,
            isScaled: true,
            metrics: {
                activityStripReservedWidth: reservedWidth,
                appScale,
                contentWidth,
                displayedContentWidth: contentWidth * appScale,
                layoutWidth: DOCKED_LAYOUT_BASE_WIDTH,
            },
            paneWidths: {
                mainWidth: contentWidth / 2,
                viewerWidth: contentWidth / 2,
            },
        });
    });

    it("builds docked CSS variables from layout geometry", () => {
        const layout = {
            inverseAppScale: 1.2,
            isScaled: true,
            metrics: {
                activityStripReservedWidth: 43.2,
                appScale: 5 / 6,
                contentWidth: 927.3333333333334,
                displayedContentWidth: 772.7777777777778,
                layoutWidth: DOCKED_LAYOUT_BASE_WIDTH,
            },
            paneWidths: {
                mainWidth: 463.6666666666667,
                viewerWidth: 463.6666666666667,
            },
        } satisfies DockedLayout;

        expect(buildDockedCssVariables(layout)).toEqual({
            "--dock-app-scale": `${5 / 6}`,
            "--dock-activity-strip-reserved-width": "43.2px",
            "--dock-app-unscaled-height": "120%",
            "--dock-app-unscaled-width": "120%",
            "--dock-main-width": "463.6666666666667px",
            "--dock-viewer-width": "463.6666666666667px",
        });
    });

    it("sanitizes invalid docked CSS scale values before writing CSS variables", () => {
        const errorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
        const layout = {
            inverseAppScale: Number.NaN,
            isScaled: true,
            metrics: {
                activityStripReservedWidth: 36,
                appScale: Number.POSITIVE_INFINITY,
                contentWidth: 900,
                displayedContentWidth: 900,
                layoutWidth: DOCKED_LAYOUT_BASE_WIDTH,
            },
            paneWidths: {
                mainWidth: 450,
                viewerWidth: 450,
            },
        } satisfies DockedLayout;

        expect(buildDockedCssVariables(layout)).toEqual({
            "--dock-app-scale": "1",
            "--dock-activity-strip-reserved-width": "36px",
            "--dock-app-unscaled-height": "100%",
            "--dock-app-unscaled-width": "100%",
            "--dock-main-width": "450px",
            "--dock-viewer-width": "450px",
        });
        expect(errorSpy).toHaveBeenCalledTimes(2);
        expect(errorSpy.mock.calls[0]?.[0]).toBe("[viewerDocking] buildDockedCssVariables: invalid appScale");
        expect(errorSpy.mock.calls[1]?.[0]).toBe("[viewerDocking] buildDockedCssVariables: invalid inverseAppScale");
    });

    it("keeps docked layout tokens aligned across TS, CSS, and Wails runtime", () => {
        // These assertions intentionally read the repo-local CSS and Go sources
        // so the shared dock tokens cannot drift across layers.
        const cssSource = readFileSync(resolve(import.meta.dirname, "../src/styles/base.css"), "utf8");
        const mainGoSource = readFileSync(resolve(import.meta.dirname, "../../main.go"), "utf8");

        const sidebarWidth = Number.parseInt(
            cssSource.match(/--sidebar-width:\s*(\d+)px;/)?.[1] ?? "",
            10,
        );
        const activityStripWidth = Number.parseInt(
            cssSource.match(/--activity-strip-width:\s*(\d+)px;/)?.[1] ?? "",
            10,
        );
        const dividerWidth = Number.parseInt(
            cssSource.match(/--dock-divider-width:\s*(\d+)px;/)?.[1] ?? "",
            10,
        );
        const minWidth = Number.parseInt(
            mainGoSource.match(/\bMinWidth:\s*(\d+)/)?.[1] ?? "",
            10,
        );

        expect(sidebarWidth).toBe(DOCKED_SIDEBAR_WIDTH);
        expect(activityStripWidth).toBe(DOCKED_ACTIVITY_STRIP_WIDTH);
        expect(dividerWidth).toBe(DOCKED_DIVIDER_WIDTH);
        expect(minWidth).toBe(DOCKED_WINDOW_MIN_WIDTH);
    });

    it("clamps store updates and resets invalid values to the default ratio", () => {
        useViewerStore.getState().setDockRatio(DOCK_RATIO_MIN - 0.1);
        expect(useViewerStore.getState().dockRatio).toBe(DOCK_RATIO_MIN);

        useViewerStore.getState().setDockRatio(DOCK_RATIO_MAX + 0.1);
        expect(useViewerStore.getState().dockRatio).toBe(DOCK_RATIO_MAX);

        useViewerStore.getState().setDockRatio(Number.NaN);
        expect(useViewerStore.getState().dockRatio).toBe(DOCK_RATIO_DEFAULT);
    });

    it("restores the default dock ratio", () => {
        useViewerStore.getState().setDockRatio(DOCK_RATIO_MAX);
        expect(useViewerStore.getState().dockRatio).toBe(DOCK_RATIO_MAX);

        useViewerStore.getState().resetDockRatio();
        expect(useViewerStore.getState().dockRatio).toBe(DOCK_RATIO_DEFAULT);
    });
});
