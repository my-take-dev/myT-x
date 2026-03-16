import type {CanvasNodePosition, CanvasNodeSize, OrchestratorTask} from "../types/canvas";

const NODE_WIDTH = 450;
const NODE_HEIGHT = 350;
const H_GAP = 80;
const V_GAP = 60;
const STAGGER_OFFSET = 40;

/**
 * タスクエッジから隣接リストを構築し、BFSでツリーレイアウトを計算する。
 * ツリーに含まれないペインは右側にスタガード配置する。
 */
export function computeTreeLayout(
    paneIds: string[],
    tasks: OrchestratorTask[],
    nodeSizes?: Record<string, CanvasNodeSize>,
): Record<string, CanvasNodePosition> {
    if (paneIds.length === 0) return {};

    const getSize = (id: string): CanvasNodeSize =>
        nodeSizes?.[id] ?? {width: NODE_WIDTH, height: NODE_HEIGHT};

    // 隣接リスト構築 (sender → [assignee, ...])
    const children = new Map<string, string[]>();
    const hasParent = new Set<string>();
    const paneSet = new Set(paneIds);
    const childSet = new Set<string>();

    for (const task of tasks) {
        const src = task.sender_pane_id;
        const dst = task.assignee_pane_id;
        if (!src || !dst || !paneSet.has(src) || !paneSet.has(dst)) continue;
        if (src === dst) continue;

        if (!children.has(src)) children.set(src, []);
        const list = children.get(src);
        if (!list) continue;
        if (!childSet.has(`${src}\0${dst}`)) {
            childSet.add(`${src}\0${dst}`);
            list.push(dst);
        }
        hasParent.add(dst);
    }

    // ルートノード検出（送信元だが受信先でない）
    const roots: string[] = [];
    for (const id of paneIds) {
        if (children.has(id) && !hasParent.has(id)) {
            roots.push(id);
        }
    }

    const positions: Record<string, CanvasNodePosition> = {};
    const placed = new Set<string>();

    // BFS でレベルごとに配置
    if (roots.length > 0) {
        const queue: Array<{ id: string; level: number }> = [];
        for (const r of roots) {
            queue.push({id: r, level: 0});
            placed.add(r);
        }

        const levels = new Map<number, string[]>();
        let idx = 0;
        while (idx < queue.length) {
            const {id, level} = queue[idx++];
            if (!levels.has(level)) levels.set(level, []);
            levels.get(level)?.push(id);

            const kids = children.get(id);
            if (!kids) continue;
            for (const kid of kids) {
                if (placed.has(kid)) continue;
                placed.add(kid);
                queue.push({id: kid, level: level + 1});
            }
        }

        // レベルごとの最大高さを計算し、累積Y位置を算出
        const sortedLevels = [...levels.keys()].sort((a, b) => a - b);
        const maxHeightPerLevel = new Map<number, number>();
        for (const [level, ids] of levels) {
            maxHeightPerLevel.set(level, Math.max(...ids.map((id) => getSize(id).height)));
        }
        const levelY = new Map<number, number>();
        levelY.set(sortedLevels[0], 0);
        for (let i = 1; i < sortedLevels.length; i++) {
            const prev = sortedLevels[i - 1];
            levelY.set(sortedLevels[i], (levelY.get(prev) ?? 0) + (maxHeightPerLevel.get(prev) ?? NODE_HEIGHT) + V_GAP);
        }

        // レベルごとに実サイズで横並び配置
        for (const [level, ids] of levels) {
            const widths = ids.map((id) => getSize(id).width);
            const totalWidth = widths.reduce((sum, w) => sum + w, 0) + (ids.length - 1) * H_GAP;
            let cumulativeX = -totalWidth / 2;
            for (let i = 0; i < ids.length; i++) {
                positions[ids[i]] = {
                    x: cumulativeX,
                    y: levelY.get(level) ?? 0,
                };
                cumulativeX += widths[i] + H_GAP;
            }
        }
    }

    // ツリーに含まれないペインを実サイズで配置
    let maxBottomEdge = 0;
    let maxRightEdge = 0;
    for (const id of placed) {
        const pos = positions[id];
        if (pos) {
            const size = getSize(id);
            maxBottomEdge = Math.max(maxBottomEdge, pos.y + size.height);
            maxRightEdge = Math.max(maxRightEdge, pos.x + size.width);
        }
    }
    const orphanStartY = placed.size > 0 ? maxBottomEdge + V_GAP * 2 : 0;
    const orphanStartX = placed.size > 0 ? maxRightEdge + H_GAP * 2 : 0;

    const orphanIds: string[] = [];
    for (const id of paneIds) {
        if (!placed.has(id)) orphanIds.push(id);
    }

    const ORPHAN_COLS = 3;
    let rowY = orphanStartY;
    for (let row = 0; row * ORPHAN_COLS < orphanIds.length; row++) {
        const rowIds = orphanIds.slice(row * ORPHAN_COLS, (row + 1) * ORPHAN_COLS);
        let colX = orphanStartX;
        let maxRowHeight = 0;
        for (const id of rowIds) {
            const size = getSize(id);
            positions[id] = {x: colX, y: rowY};
            colX += size.width + H_GAP;
            maxRowHeight = Math.max(maxRowHeight, size.height);
        }
        rowY += maxRowHeight + V_GAP;
    }

    // ツリーも無くorphanもrootsもない場合のフォールバック
    let staggerIdx = 0;
    for (const id of paneIds) {
        if (!positions[id]) {
            positions[id] = {
                x: staggerIdx * STAGGER_OFFSET,
                y: staggerIdx * STAGGER_OFFSET,
            };
            staggerIdx++;
        }
    }

    return positions;
}
