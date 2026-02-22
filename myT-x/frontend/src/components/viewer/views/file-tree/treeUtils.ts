import type { FileEntry, FlatNode } from "./fileTreeTypes";

/**
 * Flattens a tree structure into a list for virtualized rendering.
 * Only expanded directories have their children included.
 * Pattern from iori-editor's flattenTree utility.
 */
export function flattenTree(
  entries: FileEntry[],
  expandedPaths: Set<string>,
  childrenCache: Map<string, FileEntry[]>,
  loadingPaths: Set<string>,
  depth: number = 0,
): FlatNode[] {
  const result: FlatNode[] = [];

  for (const entry of entries) {
    const isExpanded = entry.is_dir && expandedPaths.has(entry.path);
    const isLoading = loadingPaths.has(entry.path);

    result.push({
      path: entry.path,
      name: entry.name,
      isDir: entry.is_dir,
      depth,
      isExpanded,
      isLoading,
      size: entry.size,
    });

    // Recurse into expanded directories with cached children.
    if (entry.is_dir && isExpanded) {
      const children = childrenCache.get(entry.path);
      if (children) {
        result.push(...flattenTree(children, expandedPaths, childrenCache, loadingPaths, depth + 1));
      }
    }
  }

  return result;
}

/** Formats file size in human-readable form. */
export function formatFileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}
