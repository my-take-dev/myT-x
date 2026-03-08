import {afterEach, describe, expect, it} from "vitest";
import {consumeBoundaryWheel} from "../src/utils/scrollBoundary";

function makeScroller({
                          scrollTop,
                          scrollHeight,
                          clientHeight,
                      }: {
    readonly scrollTop: number;
    readonly scrollHeight: number;
    readonly clientHeight: number;
}): HTMLElement {
    const el = document.createElement("div");
    Object.defineProperty(el, "scrollTop", {
        value: scrollTop,
        writable: true,
        configurable: true,
    });
    Object.defineProperty(el, "scrollHeight", {
        value: scrollHeight,
        configurable: true,
    });
    Object.defineProperty(el, "clientHeight", {
        value: clientHeight,
        configurable: true,
    });
    return el;
}

describe("consumeBoundaryWheel", () => {
    const originalDpr = window.devicePixelRatio;

    afterEach(() => {
        Object.defineProperty(window, "devicePixelRatio", {
            value: originalDpr,
            configurable: true,
        });
    });

    it("consumes upward wheel at top boundary and clamps to exact zero", () => {
        const el = makeScroller({
            scrollTop: 1,
            scrollHeight: 1000,
            clientHeight: 400,
        });

        const consumed = consumeBoundaryWheel(el, -120);
        expect(consumed).toBe(true);
        expect(el.scrollTop).toBe(0);
    });

    it("consumes downward wheel at bottom boundary and clamps to exact max", () => {
        Object.defineProperty(window, "devicePixelRatio", {
            value: 2,
            configurable: true,
        });

        const el = makeScroller({
            scrollTop: 599,
            scrollHeight: 1000,
            clientHeight: 400,
        });

        const consumed = consumeBoundaryWheel(el, 120);
        expect(consumed).toBe(true);
        expect(el.scrollTop).toBe(600);
    });

    it("does not consume wheel when not at top/bottom boundary", () => {
        const el = makeScroller({
            scrollTop: 250,
            scrollHeight: 1000,
            clientHeight: 400,
        });

        expect(consumeBoundaryWheel(el, -120)).toBe(false);
        expect(consumeBoundaryWheel(el, 120)).toBe(false);
    });

    it("returns false for deltaY === 0 (horizontal-only scroll)", () => {
        const el = makeScroller({
            scrollTop: 0,
            scrollHeight: 1000,
            clientHeight: 400,
        });

        expect(consumeBoundaryWheel(el, 0)).toBe(false);
    });

    it("consumes wheel in both directions when content fits viewport (maxScrollTop=0)", () => {
        const el = makeScroller({
            scrollTop: 0,
            scrollHeight: 400,
            clientHeight: 400,
        });

        // At top AND bottom simultaneously - content fits, consume to prevent chaining
        expect(consumeBoundaryWheel(el, -120)).toBe(true);
        expect(consumeBoundaryWheel(el, 120)).toBe(true);
    });

    it("treats negative scrollTop as at-top boundary", () => {
        // Browsers should not produce negative scrollTop, but rubber-banding on some
        // platforms can briefly push it below zero. Treat as top boundary.
        const el = makeScroller({
            scrollTop: -1,
            scrollHeight: 1000,
            clientHeight: 400,
        });

        expect(consumeBoundaryWheel(el, -120)).toBe(true);
        // Not at bottom, so downward should not be consumed
        expect(consumeBoundaryWheel(el, 120)).toBe(false);
    });

    it("handles devicePixelRatio edge values gracefully", () => {
        // DPR = 0 → fallback to MIN_BOUNDARY_EPSILON_PX (2)
        Object.defineProperty(window, "devicePixelRatio", {
            value: 0,
            configurable: true,
        });
        const el = makeScroller({
            scrollTop: 1,
            scrollHeight: 1000,
            clientHeight: 400,
        });
        expect(consumeBoundaryWheel(el, -120)).toBe(true);

        // DPR = NaN → fallback
        Object.defineProperty(window, "devicePixelRatio", {
            value: NaN,
            configurable: true,
        });
        expect(consumeBoundaryWheel(el, -120)).toBe(true);

        // DPR = Infinity → fallback
        Object.defineProperty(window, "devicePixelRatio", {
            value: Infinity,
            configurable: true,
        });
        expect(consumeBoundaryWheel(el, -120)).toBe(true);
    });
});
