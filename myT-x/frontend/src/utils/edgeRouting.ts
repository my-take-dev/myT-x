import type {CanvasNodePosition} from "../types/canvas";

export interface DirectedEdge {
    sourcePane: string;
    targetPane: string;
}

/**
 * 2ペインのY座標を比較し、上にあるペインをsource（Bottom出力）、
 * 下にあるペインをtarget（Top入力）とする。
 * 同一Y・位置未定の場合はアルファベット順にフォールバック。
 */
export function determineEdgeDirection(
    paneA: string,
    paneB: string,
    positions: Record<string, CanvasNodePosition>,
): DirectedEdge {
    const posA = positions[paneA];
    const posB = positions[paneB];
    if (posA && posB && posA.y !== posB.y) {
        const aIsHigher = posA.y < posB.y;
        return {
            sourcePane: aIsHigher ? paneA : paneB,
            targetPane: aIsHigher ? paneB : paneA,
        };
    }
    // 同一Y or 未定 → アルファベット順
    return {sourcePane: paneA, targetPane: paneB};
}

/** ノード間の垂直ギャップから適切なoffsetを算出 */
export function computeEdgeOffset(sourceY: number, targetY: number): number {
    const gap = Math.abs(targetY - sourceY);
    // ギャップの35%、30〜80pxにクランプ
    return Math.max(30, Math.min(80, gap * 0.35));
}
