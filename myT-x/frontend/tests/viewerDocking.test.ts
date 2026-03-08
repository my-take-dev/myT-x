import {beforeEach, describe, expect, it} from "vitest";
import {useViewerStore} from "../src/components/viewer/viewerStore";
import {
    clampDockRatio,
    DOCK_RATIO_DEFAULT,
    DOCK_RATIO_MAX,
    DOCK_RATIO_MIN,
    isViewerDocked,
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

    it("treats the viewer as docked only when docked mode has an active view", () => {
        expect(isViewerDocked("overlay", "git-graph")).toBe(false);
        expect(isViewerDocked("docked", null)).toBe(false);
        expect(isViewerDocked("docked", "git-graph")).toBe(true);
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
