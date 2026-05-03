import type {FileEntry, FileNode, FlatNode} from "./fileTreeTypes";

export function normalizePath(path: string): string {
    return path.replaceAll("\\", "/");
}

export function isSameOrDescendantPath(path: string, targetPath: string): boolean {
    const normalizedPath = normalizePath(path);
    const normalizedTarget = normalizePath(targetPath);
    return normalizedPath === normalizedTarget || normalizedPath.startsWith(`${normalizedTarget}/`);
}

function mergeLoadedSubtrees(
    nextNodes: readonly FileNode[],
    existingNodes: readonly FileNode[],
): readonly FileNode[] {
    const existingByPath = new Map(existingNodes.map((node) => [normalizePath(node.path), node]));

    return nextNodes.map((node) => {
        const existingNode = existingByPath.get(normalizePath(node.path));
        if (!node.isDir || !existingNode?.isDir) {
            return node;
        }

        if (node.children) {
            const children = mergeLoadedSubtrees(node.children, existingNode.children ?? []);
            return {
                ...node,
                hasChildren: node.hasChildren || children.length > 0,
                hasViewTarget: node.hasViewTarget || hasViewTargetInNodes(children),
                children,
            };
        }

        if (!node.hasChildren || !existingNode.children) {
            return node;
        }

        return {
            ...node,
            hasViewTarget: node.hasViewTarget || hasViewTargetInNodes(existingNode.children),
            children: existingNode.children,
        };
    });
}

function hasViewTargetInNodes(nodes: readonly FileNode[]): boolean {
    return nodes.some((node) => node.hasViewTarget);
}

function remapNodePath(path: string, oldPath: string, newPath: string): string {
    if (path === oldPath) {
        return newPath;
    }
    return `${newPath}${path.slice(oldPath.length)}`;
}

export function fileEntriesToNodes(entries: readonly FileEntry[]): readonly FileNode[] {
    return entries.map((entry) => ({
        name: entry.name,
        path: normalizePath(entry.path),
        isDir: entry.is_dir,
        hasChildren: entry.is_dir ? entry.has_children : false,
        hasViewTarget: entry.has_view_target,
        size: entry.is_dir ? undefined : entry.size,
    }));
}

export function findNodeByPath(nodes: readonly FileNode[], targetPath: string): FileNode | null {
    const normalizedTarget = normalizePath(targetPath);

    for (const node of nodes) {
        if (normalizePath(node.path) === normalizedTarget) {
            return node;
        }
        if (node.isDir && node.children) {
            const childMatch = findNodeByPath(node.children, normalizedTarget);
            if (childMatch) {
                return childMatch;
            }
        }
    }

    return null;
}

export function isPathKnownAbsent(nodes: readonly FileNode[], targetPath: string): boolean {
    const normalizedTarget = normalizePath(targetPath);
    if (normalizedTarget === "") {
        return false;
    }

    const segments = normalizedTarget.split("/");
    let currentNodes = nodes;
    let currentPath = "";

    for (let index = 0; index < segments.length; index += 1) {
        currentPath = currentPath === "" ? segments[index] : `${currentPath}/${segments[index]}`;
        const node = currentNodes.find((entry) => normalizePath(entry.path) === currentPath);
        if (!node) {
            return true;
        }
        if (index === segments.length - 1) {
            return false;
        }
        if (!node.isDir) {
            return true;
        }
        if (!node.children) {
            return false;
        }
        currentNodes = node.children;
    }

    return false;
}

export function mergeRootNodes(
    nextNodes: readonly FileNode[],
    existingTree: readonly FileNode[],
): readonly FileNode[] {
    return mergeLoadedSubtrees(nextNodes, existingTree);
}

function clearLoadedChildren(node: FileNode): FileNode {
    const cleared: FileNode = {
        name: node.name,
        path: node.path,
        isDir: node.isDir,
        hasChildren: node.hasChildren,
        hasViewTarget: node.hasViewTarget,
    };
    if (node.size === undefined) {
        return cleared;
    }
    return {
        ...cleared,
        size: node.size,
    };
}

export function clearLoadedChildrenForPaths(
    tree: readonly FileNode[],
    paths: readonly string[],
): readonly FileNode[] {
    const targetPaths = new Set(paths.map(normalizePath));
    if (targetPaths.size === 0) {
        return tree;
    }

    return clearLoadedChildrenForTargetPaths(tree, targetPaths);
}

function clearLoadedChildrenForTargetPaths(
    tree: readonly FileNode[],
    targetPaths: ReadonlySet<string>,
): readonly FileNode[] {
    return tree.map((node) => {
        if (!node.isDir) {
            return node;
        }
        if (targetPaths.has(normalizePath(node.path))) {
            return clearLoadedChildren(node);
        }
        if (!node.children) {
            return node;
        }
        return {
            ...node,
            children: clearLoadedChildrenForTargetPaths(node.children, targetPaths),
        };
    });
}

/**
 * Flattens a tree structure into a list for virtualized rendering.
 * Only expanded directories have their children included.
 */
export function flattenTree(
    entries: readonly FileNode[],
    expandedPaths: ReadonlySet<string>,
    loadingPaths: ReadonlySet<string>,
    depth: number = 0,
): readonly FlatNode[] {
    const result: FlatNode[] = [];
    const currentDepth = depth;

    for (const entry of entries) {
        if (entry.isDir) {
            const isExpanded = expandedPaths.has(entry.path);
            const isLoading = loadingPaths.has(entry.path);

            result.push({
                path: entry.path,
                name: entry.name,
                isDir: true,
                depth: currentDepth,
                hasChildren: entry.hasChildren,
                isExpanded,
                isLoading,
            });

            if (isExpanded && entry.children) {
                result.push(...flattenTree(entry.children, expandedPaths, loadingPaths, currentDepth + 1));
            }
            continue;
        }

        result.push({
            path: entry.path,
            name: entry.name,
            isDir: false,
            depth: currentDepth,
            size: entry.size ?? 0,
        });
    }

    return result;
}

export function mergeChildrenIntoTree(
    tree: readonly FileNode[],
    targetPath: string,
    children: readonly FileNode[],
): readonly FileNode[] {
    const normalizedTarget = normalizePath(targetPath);

    return tree.map((node) => {
        if (normalizePath(node.path) === normalizedTarget) {
            if (!node.isDir) {
                return node;
            }
            return {
                ...node,
                hasChildren: children.length > 0,
                hasViewTarget: hasViewTargetInNodes(children),
                children: mergeLoadedSubtrees(children, node.children ?? []),
            };
        }

        if (node.isDir && node.children && isSameOrDescendantPath(normalizedTarget, node.path)) {
            const nextChildren = mergeChildrenIntoTree(node.children, normalizedTarget, children);
            return {
                ...node,
                hasViewTarget: node.hasViewTarget || hasViewTargetInNodes(nextChildren),
                children: nextChildren,
            };
        }

        return node;
    });
}

export function removePathFromTree(tree: readonly FileNode[], targetPath: string): readonly FileNode[] {
    const normalizedTarget = normalizePath(targetPath);
    const nextTree: FileNode[] = [];

    for (const node of tree) {
        if (isSameOrDescendantPath(node.path, normalizedTarget)) {
            continue;
        }

        if (node.isDir && node.children) {
            const nextChildren = removePathFromTree(node.children, normalizedTarget);
            nextTree.push({
                ...node,
                children: nextChildren,
                hasChildren: nextChildren.length > 0,
                hasViewTarget: hasViewTargetInNodes(nextChildren),
            });
            continue;
        }

        nextTree.push(node);
    }

    return nextTree;
}

export function renamePathInTree(
    tree: readonly FileNode[],
    oldPath: string,
    newPath: string,
): readonly FileNode[] {
    const normalizedOldPath = normalizePath(oldPath);
    const normalizedNewPath = normalizePath(newPath);

    return tree.map((node) => {
        const nextNode = node.isDir && node.children
            ? {
                ...node,
                children: renamePathInTree(node.children, normalizedOldPath, normalizedNewPath),
            }
            : node;

        if (!isSameOrDescendantPath(nextNode.path, normalizedOldPath)) {
            return nextNode;
        }

        const nextPath = remapNodePath(nextNode.path, normalizedOldPath, normalizedNewPath);
        return {
            ...nextNode,
            path: nextPath,
            name: nextPath.split("/").pop() ?? nextNode.name,
        };
    });
}

/** Formats file size in human-readable form. */
export function formatFileSize(bytes: number): string {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}
