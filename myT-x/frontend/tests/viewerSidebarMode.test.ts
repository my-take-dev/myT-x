import {describe, expect, it} from "vitest";
import {
    DEFAULT_VIEWER_SIDEBAR_MODE,
    normalizeViewerSidebarMode,
    serializeViewerSidebarMode,
} from "../src/utils/viewerSidebarMode";

describe("viewerSidebarMode helpers", () => {
    it("falls back to overlay for nullish and invalid values", () => {
        expect(normalizeViewerSidebarMode(undefined)).toBe(DEFAULT_VIEWER_SIDEBAR_MODE);
        expect(normalizeViewerSidebarMode(null)).toBe(DEFAULT_VIEWER_SIDEBAR_MODE);
        expect(normalizeViewerSidebarMode("")).toBe(DEFAULT_VIEWER_SIDEBAR_MODE);
        expect(normalizeViewerSidebarMode("docked")).toBe("docked");
        expect(normalizeViewerSidebarMode("stacked")).toBe(DEFAULT_VIEWER_SIDEBAR_MODE);
    });

    it("serializes supported modes without collapsing them", () => {
        expect(serializeViewerSidebarMode("overlay")).toBe("overlay");
        expect(serializeViewerSidebarMode("docked")).toBe("docked");
    });

    it("omits empty values from persisted config", () => {
        expect(serializeViewerSidebarMode("")).toBeUndefined();
        expect(serializeViewerSidebarMode(undefined)).toBeUndefined();
        expect(serializeViewerSidebarMode(null)).toBeUndefined();
    });
});
