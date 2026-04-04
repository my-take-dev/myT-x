import {describe, expect, it} from "vitest";
import type {FileEntry, FileNode} from "./fileTreeTypes";
import {
    fileEntriesToNodes,
    flattenTree,
    mergeChildrenIntoTree,
    mergeRootNodes,
    removePathFromTree,
    renamePathInTree,
} from "./treeUtils";

function directoryEntry(path: string, hasChildren: boolean): FileEntry {
    const segments = path.split("/");
    return {
        name: segments[segments.length - 1],
        path,
        is_dir: true,
        size: 0,
        has_children: hasChildren,
    };
}

function fileEntry(path: string, size: number): FileEntry {
    const segments = path.split("/");
    return {
        name: segments[segments.length - 1],
        path,
        is_dir: false,
        size,
    };
}

describe("treeUtils", () => {
    it("converts backend entries into hierarchical nodes", () => {
        const nodes = fileEntriesToNodes([
            directoryEntry("src", true),
            fileEntry("README.md", 128),
        ]);

        expect(nodes).toEqual([
            {
                name: "src",
                path: "src",
                isDir: true,
                hasChildren: true,
                size: undefined,
            },
            {
                name: "README.md",
                path: "README.md",
                isDir: false,
                hasChildren: false,
                size: 128,
            },
        ]);
    });

    it("preserves loaded descendants when refreshing root nodes", () => {
        const existingTree: readonly FileNode[] = [
            {
                name: "src",
                path: "src",
                isDir: true,
                hasChildren: true,
                children: [{
                    name: "nested",
                    path: "src/nested",
                    isDir: true,
                    hasChildren: true,
                    children: [{
                        name: "deep.txt",
                        path: "src/nested/deep.txt",
                        isDir: false,
                        hasChildren: false,
                        size: 42,
                    }],
                }],
            },
        ];

        const refreshed = mergeRootNodes(fileEntriesToNodes([directoryEntry("src", true)]), existingTree);
        expect(refreshed[0]?.children?.[0]?.path).toBe("src/nested");
        expect(refreshed[0]?.children?.[0]?.children?.[0]?.path).toBe("src/nested/deep.txt");
    });

    it("replaces the target directory while retaining matching loaded descendants", () => {
        const existingTree: readonly FileNode[] = [{
            name: "src",
            path: "src",
            isDir: true,
            hasChildren: true,
            children: [{
                name: "nested",
                path: "src/nested",
                isDir: true,
                hasChildren: true,
                children: [{
                    name: "deep.txt",
                    path: "src/nested/deep.txt",
                    isDir: false,
                    hasChildren: false,
                    size: 42,
                }],
            }],
        }];

        const merged = mergeChildrenIntoTree(existingTree, "src", fileEntriesToNodes([
            directoryEntry("src/nested", true),
            fileEntry("src/app.ts", 64),
        ]));

        expect(merged[0]?.children?.map((node) => node.path)).toEqual(["src/nested", "src/app.ts"]);
        expect(merged[0]?.children?.[0]?.children?.[0]?.path).toBe("src/nested/deep.txt");
    });

    it("flattens only expanded directories and carries hasChildren", () => {
        const tree = mergeChildrenIntoTree(
            fileEntriesToNodes([directoryEntry("src", true)]),
            "src",
            fileEntriesToNodes([fileEntry("src/app.ts", 64)]),
        );

        const flatNodes = flattenTree(tree, new Set(["src"]), new Set(["src"]));
        expect(flatNodes).toEqual([
            {
                path: "src",
                name: "src",
                isDir: true,
                depth: 0,
                hasChildren: true,
                isExpanded: true,
                isLoading: true,
            },
            {
                path: "src/app.ts",
                name: "app.ts",
                isDir: false,
                depth: 1,
                size: 64,
            },
        ]);
    });

    it("removes a subtree cleanly", () => {
        const tree = mergeChildrenIntoTree(
            fileEntriesToNodes([directoryEntry("src", true)]),
            "src",
            fileEntriesToNodes([
                directoryEntry("src/nested", true),
                fileEntry("src/app.ts", 64),
            ]),
        );

        expect(removePathFromTree(tree, "src/nested")[0]?.children?.map((node) => node.path)).toEqual(["src/app.ts"]);
    });

    it("marks a directory as empty after removing its last child", () => {
        const tree = mergeChildrenIntoTree(
            fileEntriesToNodes([directoryEntry("src", true)]),
            "src",
            fileEntriesToNodes([directoryEntry("src/nested", false)]),
        );

        const nextTree = removePathFromTree(tree, "src/nested");
        expect(nextTree[0]).toMatchObject({
            path: "src",
            hasChildren: false,
        });
    });

    it("renames a subtree and all descendants", () => {
        const tree = mergeChildrenIntoTree(
            fileEntriesToNodes([directoryEntry("src", true)]),
            "src",
            fileEntriesToNodes([directoryEntry("src/nested", true)]),
        );
        const nestedTree = mergeChildrenIntoTree(
            tree,
            "src/nested",
            fileEntriesToNodes([fileEntry("src/nested/deep.txt", 64)]),
        );

        const renamed = renamePathInTree(nestedTree, "src/nested", "src/renamed");
        expect(renamed[0]?.children?.[0]?.path).toBe("src/renamed");
        expect(renamed[0]?.children?.[0]?.name).toBe("renamed");
        expect(renamed[0]?.children?.[0]?.children?.[0]?.path).toBe("src/renamed/deep.txt");
    });
});
