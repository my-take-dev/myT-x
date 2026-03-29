import type {CanvasNodePosition, CanvasNodeSize, OrchestratorTask} from "../types/canvas";

const NODE_WIDTH = 450;
const NODE_HEIGHT = 350;
const H_GAP = 80;
const V_GAP = 60;
const STAGGER_OFFSET = 40;

/**
 * タスクエッジからツリーレイアウトを計算する。
 *
 * Algorithm (6 Phase):
 * 1. 無方向グラフ + 有向度数を構築
 * 2. ルート選出: 自然ルート(sender-only) → フォールバック(接続加重スコアリング)
 * 3. BFS spanning tree構築 (childrenOf マップ)
 * 4. レベルごとの垂直位置計算
 * 5. サブツリー幅考慮の水平配置 (親を子の中心に配置)
 * 6. orphan/フォールバック配置
 */
export function computeTreeLayout(
    paneIds: string[],
    tasks: OrchestratorTask[],
    nodeSizes?: Record<string, CanvasNodeSize>,
): Record<string, CanvasNodePosition> {
    if (paneIds.length === 0) return {};

    const getSize = (id: string): CanvasNodeSize =>
        nodeSizes?.[id] ?? {width: NODE_WIDTH, height: NODE_HEIGHT};

    // === Phase 1: グラフ構築 ===
    const neighbors = new Map<string, Set<string>>();
    const dirChildren = new Map<string, string[]>();
    const outDeg = new Map<string, number>();
    const inDeg = new Map<string, number>();
    const paneSet = new Set(paneIds);
    const dirEdgeSeen = new Set<string>();

    for (const task of tasks) {
        const src = task.sender_pane_id;
        const dst = task.assignee_pane_id;
        if (!src || !dst || !paneSet.has(src) || !paneSet.has(dst) || src === dst) continue;

        // 無方向隣接
        if (!neighbors.has(src)) neighbors.set(src, new Set());
        if (!neighbors.has(dst)) neighbors.set(dst, new Set());
        neighbors.get(src)!.add(dst);
        neighbors.get(dst)!.add(src);

        // 有向エッジ (重複除去)
        const dirKey = `${src}\0${dst}`;
        if (!dirEdgeSeen.has(dirKey)) {
            dirEdgeSeen.add(dirKey);
            outDeg.set(src, (outDeg.get(src) ?? 0) + 1);
            inDeg.set(dst, (inDeg.get(dst) ?? 0) + 1);
            if (!dirChildren.has(src)) dirChildren.set(src, []);
            dirChildren.get(src)!.push(dst);
        }
    }

    const degree = new Map<string, number>();
    for (const id of paneIds) {
        degree.set(id, neighbors.get(id)?.size ?? 0);
    }

    // === Phase 2 + 3: ルート選出 + BFS spanning tree ===
    const rootScore = (id: string): number =>
        (degree.get(id) ?? 0) * 100
        + (outDeg.get(id) ?? 0) * 10
        - (inDeg.get(id) ?? 0) * 5;

    const roots: string[] = [];
    const placed = new Set<string>();
    const childrenOf = new Map<string, string[]>();
    const nodeLevel = new Map<string, number>();

    // Step 1: 自然ルート (outDeg > 0 && inDeg == 0) → 有向BFS
    const naturalRoots = paneIds
        .filter(id => (outDeg.get(id) ?? 0) > 0 && (inDeg.get(id) ?? 0) === 0)
        .sort((a, b) => (degree.get(b) ?? 0) - (degree.get(a) ?? 0) || a.localeCompare(b));

    for (const root of naturalRoots) {
        if (placed.has(root)) continue;
        roots.push(root);
        directedBFS(root, dirChildren, placed, childrenOf, nodeLevel, degree);
    }

    // Step 2: 残りの連結成分 → 接続加重スコアリング + 無方向BFS
    const remainingConnected = paneIds
        .filter(id => !placed.has(id) && (degree.get(id) ?? 0) > 0)
        .sort((a, b) => rootScore(b) - rootScore(a) || a.localeCompare(b));

    for (const candidate of remainingConnected) {
        if (placed.has(candidate)) continue;
        roots.push(candidate);
        undirectedBFS(candidate, neighbors, placed, childrenOf, nodeLevel, degree);
    }

    // === Phase 4: 垂直位置計算 ===
    const positions: Record<string, CanvasNodePosition> = {};
    const levels = new Map<number, string[]>();
    for (const [id, lvl] of nodeLevel) {
        if (!levels.has(lvl)) levels.set(lvl, []);
        levels.get(lvl)!.push(id);
    }

    const sortedLevels = [...levels.keys()].sort((a, b) => a - b);
    const maxHeightPerLevel = new Map<number, number>();
    for (const [lvl, ids] of levels) {
        maxHeightPerLevel.set(lvl, Math.max(...ids.map(id => getSize(id).height)));
    }
    const levelY = new Map<number, number>();
    if (sortedLevels.length > 0) {
        levelY.set(sortedLevels[0], 0);
        for (let i = 1; i < sortedLevels.length; i++) {
            const prev = sortedLevels[i - 1];
            levelY.set(
                sortedLevels[i],
                (levelY.get(prev) ?? 0) + (maxHeightPerLevel.get(prev) ?? NODE_HEIGHT) + V_GAP,
            );
        }
    }

    // === Phase 5: サブツリー幅考慮の水平配置 ===
    const subtreeWidthCache = new Map<string, number>();

    function computeSubtreeWidth(nodeId: string): number {
        if (subtreeWidthCache.has(nodeId)) return subtreeWidthCache.get(nodeId)!;
        const kids = childrenOf.get(nodeId) ?? [];
        if (kids.length === 0) {
            const w = getSize(nodeId).width;
            subtreeWidthCache.set(nodeId, w);
            return w;
        }
        let total = 0;
        for (const kid of kids) {
            total += computeSubtreeWidth(kid);
        }
        total += (kids.length - 1) * H_GAP;
        const w = Math.max(getSize(nodeId).width, total);
        subtreeWidthCache.set(nodeId, w);
        return w;
    }

    function layoutSubtree(nodeId: string, leftEdge: number): void {
        const kids = childrenOf.get(nodeId) ?? [];
        const myWidth = getSize(nodeId).width;
        const mySubW = subtreeWidthCache.get(nodeId) ?? myWidth;
        const y = levelY.get(nodeLevel.get(nodeId) ?? 0) ?? 0;

        if (kids.length === 0) {
            positions[nodeId] = {x: leftEdge + (mySubW - myWidth) / 2, y};
            return;
        }

        // 子のサブツリー幅合計
        let childTotalW = 0;
        for (const kid of kids) childTotalW += (subtreeWidthCache.get(kid) ?? 0);
        childTotalW += (kids.length - 1) * H_GAP;

        // 子を左から順に配置
        let cx = leftEdge + (mySubW - childTotalW) / 2;
        for (const kid of kids) {
            layoutSubtree(kid, cx);
            cx += (subtreeWidthCache.get(kid) ?? 0) + H_GAP;
        }

        // 親を子の中心に配置
        const firstKid = kids[0];
        const lastKid = kids[kids.length - 1];
        const fc = positions[firstKid].x + getSize(firstKid).width / 2;
        const lc = positions[lastKid].x + getSize(lastKid).width / 2;
        positions[nodeId] = {x: (fc + lc) / 2 - myWidth / 2, y};
    }

    // サブツリー幅を計算し、ルートツリーを横に並べて配置
    for (const root of roots) computeSubtreeWidth(root);

    let currentRootX = 0;
    for (const root of roots) {
        layoutSubtree(root, currentRootX);
        currentRootX += (subtreeWidthCache.get(root) ?? NODE_WIDTH) + H_GAP * 2;
    }

    // === Phase 6: Orphan配置 ===
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

    // フォールバック
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

/** 有向BFS: sender→assignee エッジのみ辿る */
function directedBFS(
    root: string,
    dirChildren: Map<string, string[]>,
    placed: Set<string>,
    childrenOf: Map<string, string[]>,
    nodeLevel: Map<string, number>,
    degree: Map<string, number>,
): void {
    const queue: Array<{ id: string; depth: number }> = [{id: root, depth: 0}];
    placed.add(root);
    nodeLevel.set(root, 0);
    let qi = 0;
    while (qi < queue.length) {
        const {id, depth} = queue[qi++];
        const kids = (dirChildren.get(id) ?? [])
            .filter(kid => !placed.has(kid))
            .sort((a, b) => (degree.get(b) ?? 0) - (degree.get(a) ?? 0) || a.localeCompare(b));
        for (const kid of kids) {
            if (placed.has(kid)) continue;
            placed.add(kid);
            nodeLevel.set(kid, depth + 1);
            if (!childrenOf.has(id)) childrenOf.set(id, []);
            childrenOf.get(id)!.push(kid);
            queue.push({id: kid, depth: depth + 1});
        }
    }
}

/** 無方向BFS: 全隣接ノードを辿る (双方向エッジ対応) */
function undirectedBFS(
    root: string,
    neighbors: Map<string, Set<string>>,
    placed: Set<string>,
    childrenOf: Map<string, string[]>,
    nodeLevel: Map<string, number>,
    degree: Map<string, number>,
): void {
    const queue: Array<{ id: string; depth: number }> = [{id: root, depth: 0}];
    placed.add(root);
    nodeLevel.set(root, 0);
    let qi = 0;
    while (qi < queue.length) {
        const {id, depth} = queue[qi++];
        const nbs = [...(neighbors.get(id) ?? [])]
            .filter(nb => !placed.has(nb))
            .sort((a, b) => (degree.get(b) ?? 0) - (degree.get(a) ?? 0) || a.localeCompare(b));
        for (const nb of nbs) {
            if (placed.has(nb)) continue;
            placed.add(nb);
            nodeLevel.set(nb, depth + 1);
            if (!childrenOf.has(id)) childrenOf.set(id, []);
            childrenOf.get(id)!.push(nb);
            queue.push({id: nb, depth: depth + 1});
        }
    }
}
