import type {OrchestratorTask} from "../types/canvas";
import type {CanvasNodePosition} from "../types/canvas";

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
): Record<string, CanvasNodePosition> {
    if (paneIds.length === 0) return {};

    // 隣接リスト構築 (sender → [assignee, ...])
    const children = new Map<string, string[]>();
    const hasParent = new Set<string>();
    const paneSet = new Set(paneIds);

    for (const task of tasks) {
        const src = task.sender_pane_id;
        const dst = task.assignee_pane_id;
        if (!src || !dst || !paneSet.has(src) || !paneSet.has(dst)) continue;
        if (src === dst) continue;

        if (!children.has(src)) children.set(src, []);
        const list = children.get(src)!;
        if (!list.includes(dst)) {
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
        const queue: Array<{id: string; level: number}> = [];
        for (const r of roots) {
            queue.push({id: r, level: 0});
            placed.add(r);
        }

        const levels = new Map<number, string[]>();
        let idx = 0;
        while (idx < queue.length) {
            const {id, level} = queue[idx++];
            if (!levels.has(level)) levels.set(level, []);
            levels.get(level)!.push(id);

            const kids = children.get(id);
            if (!kids) continue;
            for (const kid of kids) {
                if (placed.has(kid)) continue;
                placed.add(kid);
                queue.push({id: kid, level: level + 1});
            }
        }

        // レベルごとに横並び配置
        for (const [level, ids] of levels) {
            const totalWidth = ids.length * NODE_WIDTH + (ids.length - 1) * H_GAP;
            let startX = -totalWidth / 2;
            for (let i = 0; i < ids.length; i++) {
                positions[ids[i]] = {
                    x: startX + i * (NODE_WIDTH + H_GAP),
                    y: level * (NODE_HEIGHT + V_GAP),
                };
            }
        }
    }

    // ツリーに含まれないペインをスタガード配置
    let staggerIdx = 0;
    const maxLevelY = Object.values(positions).reduce((max, p) => Math.max(max, p.y), 0);
    const orphanStartY = placed.size > 0 ? maxLevelY + NODE_HEIGHT + V_GAP * 2 : 0;
    const orphanStartX = placed.size > 0
        ? Math.max(...Object.values(positions).map((p) => p.x)) + NODE_WIDTH + H_GAP * 2
        : 0;

    for (const id of paneIds) {
        if (placed.has(id)) continue;
        positions[id] = {
            x: orphanStartX + (staggerIdx % 3) * (NODE_WIDTH + H_GAP),
            y: orphanStartY + Math.floor(staggerIdx / 3) * (NODE_HEIGHT + V_GAP),
        };
        staggerIdx++;
    }

    // ツリーも無くorphanもrootsもない場合のフォールバック
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
