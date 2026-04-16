import {describe, expect, it} from "vitest";
import type {FileEntry, FileNode} from "./fileTreeTypes";
import {
    fileEntriesToNodes,
    findNodeByPath,
    flattenTree,
    formatFileSize,
    isSameOrDescendantPath,
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
        has_children: false,
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

    it("returns the original tree when removing a missing path", () => {
        const tree = mergeChildrenIntoTree(
            fileEntriesToNodes([directoryEntry("src", true)]),
            "src",
            fileEntriesToNodes([fileEntry("src/app.ts", 64)]),
        );

        expect(removePathFromTree(tree, "src/missing")).toEqual(tree);
    });

    it("returns the original tree when renaming a missing path", () => {
        const tree = mergeChildrenIntoTree(
            fileEntriesToNodes([directoryEntry("src", true)]),
            "src",
            fileEntriesToNodes([fileEntry("src/app.ts", 64)]),
        );

        expect(renamePathInTree(tree, "src/missing", "src/renamed")).toEqual(tree);
    });

    it("matches descendant paths only on directory boundaries", () => {
        expect(isSameOrDescendantPath("src\\nested", "src")).toBe(true);
        expect(isSameOrDescendantPath("src-other", "src")).toBe(false);
    });

    it("finds nested nodes through normalized paths", () => {
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

        expect(findNodeByPath(nestedTree, "src\\nested\\deep.txt")?.path).toBe("src/nested/deep.txt");
        expect(findNodeByPath(nestedTree, "src/missing")).toBeNull();
    });

    it("renames a root node and deep descendants without touching siblings", () => {
        const tree = mergeChildrenIntoTree(
            fileEntriesToNodes([
                directoryEntry("src", true),
                directoryEntry("docs", true),
            ]),
            "src",
            fileEntriesToNodes([directoryEntry("src/nested", true)]),
        );
        const nestedTree = mergeChildrenIntoTree(
            tree,
            "src/nested",
            fileEntriesToNodes([fileEntry("src/nested/deep/file.txt", 64)]),
        );

        const renamed = renamePathInTree(nestedTree, "src", "app");
        expect(renamed.map((node) => node.path)).toEqual(["app", "docs"]);
        expect(renamed[0]?.children?.[0]?.path).toBe("app/nested");
        expect(renamed[0]?.children?.[0]?.children?.[0]?.path).toBe("app/nested/deep/file.txt");
        expect(renamed[1]?.path).toBe("docs");
    });

    it.each([
        [0, "0 B"],
        [512, "512 B"],
        [1023, "1023 B"],
        [1024, "1.0 KB"],
        [1536, "1.5 KB"],
        [1024 * 1024 - 1, "1024.0 KB"],
        [1024 * 1024, "1.0 MB"],
        [5 * 1024 * 1024, "5.0 MB"],
    ])("formats %d bytes as %s", (bytes, expected) => {
        expect(formatFileSize(bytes)).toBe(expected);
    });
});
