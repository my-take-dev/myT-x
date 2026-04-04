import type {DiffTreeNode, WorkingDiffFile} from "./diffViewTypes";

function createDiffDirNode(name: string, path: string, depth: number, isExpanded: boolean): DiffTreeNode {
    return {name, path, isDir: true, depth, isExpanded};
}

function createDiffFileNode(file: WorkingDiffFile, name: string, depth: number): DiffTreeNode {
    return {name, path: file.path, isDir: false, depth, file};
}

export function buildDiffTree(files: WorkingDiffFile[], expandedDirs: Set<string>): DiffTreeNode[] {
    const sortedFiles = [...files].sort((a, b) => a.path.localeCompare(b.path));

    const nodes: DiffTreeNode[] = [];
    const addedDirs = new Set<string>();

    for (const file of sortedFiles) {
        const parts = file.path.split("/");

        for (let i = 1; i < parts.length; i++) {
            const dirPath = parts.slice(0, i).join("/");
            if (addedDirs.has(dirPath)) {
                continue;
            }

            const parentPath = parts.slice(0, i - 1).join("/");
            if (i > 1 && !expandedDirs.has(parentPath)) {
                continue;
            }

            addedDirs.add(dirPath);
            nodes.push(createDiffDirNode(parts[i - 1], dirPath, i - 1, expandedDirs.has(dirPath)));
        }

        const parentDir = parts.length > 1 ? parts.slice(0, -1).join("/") : "";
        if (parentDir === "" || expandedDirs.has(parentDir)) {
            nodes.push(createDiffFileNode(file, parts[parts.length - 1], parts.length - 1));
        }
    }

    return nodes;
}

export function collectDirectorySet(files: WorkingDiffFile[]): Set<string> {
    const allDirs = new Set<string>();

    for (const file of files) {
        const parts = file.path.split("/");
        for (let i = 1; i < parts.length; i++) {
            allDirs.add(parts.slice(0, i).join("/"));
        }
    }

    return allDirs;
}
