import type {FileEntry, FlatNode} from "./fileTreeTypes";

/**
 * Flattens a tree structure into a list for virtualized rendering.
 * Only expanded directories have their children included.
 * Pattern from iori-editor's flattenTree utility.
 */
export function flattenTree(
    entries: readonly FileEntry[],
    expandedPaths: ReadonlySet<string>,
    childrenCache: ReadonlyMap<string, readonly FileEntry[]>,
    loadingPaths: ReadonlySet<string>,
    depth: number = 0,
): readonly FlatNode[] {
    const result: FlatNode[] = [];

    for (const entry of entries) {
        if (entry.is_dir) {
            const isExpanded = expandedPaths.has(entry.path);
            const isLoading = loadingPaths.has(entry.path);

            result.push({
                path: entry.path,
                name: entry.name,
                isDir: true,
                depth,
                isExpanded,
                isLoading,
            });

            // Recurse into expanded directories with cached children.
            if (isExpanded) {
                const children = childrenCache.get(entry.path);
                if (children) {
                    result.push(...flattenTree(children, expandedPaths, childrenCache, loadingPaths, depth + 1));
                }
            }
            continue; // Directory handled above — skip file-node branch below.
        }

        result.push({
            path: entry.path,
            name: entry.name,
            isDir: false,
            depth,
            size: entry.size,
        });
    }

    return result;
}

/** Formats file size in human-readable form. */
export function formatFileSize(bytes: number): string {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}
