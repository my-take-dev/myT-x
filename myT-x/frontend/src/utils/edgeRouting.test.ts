import {describe, expect, it} from "vitest";
import {computeEdgeOffset, determineEdgeDirection} from "./edgeRouting";
import type {CanvasNodePosition} from "../types/canvas";

describe("determineEdgeDirection", () => {
    it("上にあるペインがsourceになる", () => {
        const positions: Record<string, CanvasNodePosition> = {
            paneA: {x: 0, y: 100},
            paneB: {x: 0, y: 300},
        };
        const result = determineEdgeDirection("paneA", "paneB", positions);
        expect(result).toEqual({sourcePane: "paneA", targetPane: "paneB"});
    });

    it("下にあるペインが先に渡されてもY座標で判定される", () => {
        const positions: Record<string, CanvasNodePosition> = {
            paneA: {x: 0, y: 500},
            paneB: {x: 0, y: 100},
        };
        const result = determineEdgeDirection("paneA", "paneB", positions);
        expect(result).toEqual({sourcePane: "paneB", targetPane: "paneA"});
    });

    it("同一Yの場合はアルファベット順", () => {
        const positions: Record<string, CanvasNodePosition> = {
            paneA: {x: 0, y: 200},
            paneB: {x: 100, y: 200},
        };
        const result = determineEdgeDirection("paneA", "paneB", positions);
        expect(result).toEqual({sourcePane: "paneA", targetPane: "paneB"});
    });

    it("片方の位置未定の場合はアルファベット順", () => {
        const positions: Record<string, CanvasNodePosition> = {
            paneA: {x: 0, y: 100},
        };
        const result = determineEdgeDirection("paneA", "paneB", positions);
        expect(result).toEqual({sourcePane: "paneA", targetPane: "paneB"});
    });

    it("両方の位置未定の場合はアルファベット順", () => {
        const positions: Record<string, CanvasNodePosition> = {};
        const result = determineEdgeDirection("paneA", "paneB", positions);
        expect(result).toEqual({sourcePane: "paneA", targetPane: "paneB"});
    });

    it("引数順がアルファベット逆でも位置未定時はそのまま返す", () => {
        const positions: Record<string, CanvasNodePosition> = {};
        const result = determineEdgeDirection("paneZ", "paneA", positions);
        expect(result).toEqual({sourcePane: "paneZ", targetPane: "paneA"});
    });
});

describe("computeEdgeOffset", () => {
    it("gap=200 → offset=70", () => {
        expect(computeEdgeOffset(100, 300)).toBe(70);
    });

    it("gap=400 → offset=80 (上限クランプ)", () => {
        expect(computeEdgeOffset(0, 400)).toBe(80);
    });

    it("gap=50 → offset=30 (下限クランプ)", () => {
        // 50 * 0.35 = 17.5 → clamped to 30
        expect(computeEdgeOffset(0, 50)).toBe(30);
    });

    it("gap=0 → offset=30 (最小保証)", () => {
        expect(computeEdgeOffset(100, 100)).toBe(30);
    });

    it("sourceY > targetY (逆方向) → 正常動作", () => {
        // Math.abs ensures direction doesn't matter
        expect(computeEdgeOffset(300, 100)).toBe(70);
    });

    it("gap=86 → offset=30 (境界: 86*0.35=30.1 → 30)", () => {
        expect(computeEdgeOffset(0, 86)).toBeCloseTo(30.1, 0);
    });

    it("gap=229 → offset=80 (境界: 229*0.35=80.15 → 80)", () => {
        expect(computeEdgeOffset(0, 229)).toBe(80);
    });
});
