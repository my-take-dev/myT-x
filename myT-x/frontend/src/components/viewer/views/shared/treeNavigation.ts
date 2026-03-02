export interface TreeNavigationNode {
    readonly depth: number;
    readonly isDir: boolean;
}

/**
 * Find the nearest parent directory index for a node in a flattened tree.
 * Returns -1 when no parent directory exists.
 */
export function findParentDirectoryIndex<T extends TreeNavigationNode>(
    nodes: readonly T[],
    index: number,
): number {
    const node = nodes[index];
    if (!node || node.depth <= 0) return -1;

    const targetDepth = node.depth - 1;
    for (let i = index - 1; i >= 0; i--) {
        const candidate = nodes[i];
        if (candidate.depth === targetDepth && candidate.isDir) {
            return i;
        }
    }
    return -1;
}
