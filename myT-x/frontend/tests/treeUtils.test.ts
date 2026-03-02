import {describe, expect, it} from "vitest";
import {flattenTree, formatFileSize} from "../src/components/viewer/views/file-tree/treeUtils";
import type {FileEntry} from "../src/components/viewer/views/file-tree/fileTreeTypes";

describe("flattenTree", () => {
    const empty = {
        expanded: new Set<string>(),
        children: new Map<string, readonly FileEntry[]>(),
        loading: new Set<string>(),
    };

    it("returns empty array for empty entries", () => {
        expect(flattenTree([], empty.expanded, empty.children, empty.loading)).toEqual([]);
    });

    it("flattens file entries at root depth", () => {
        const entries: FileEntry[] = [
            {name: "a.ts", path: "a.ts", is_dir: false, size: 100},
            {name: "b.ts", path: "b.ts", is_dir: false, size: 200},
        ];
        const result = flattenTree(entries, empty.expanded, empty.children, empty.loading);
        expect(result).toHaveLength(2);
        expect(result[0]).toEqual({path: "a.ts", name: "a.ts", isDir: false, depth: 0, size: 100});
        expect(result[1]).toEqual({path: "b.ts", name: "b.ts", isDir: false, depth: 0, size: 200});
    });

    it("flattens collapsed directory (no children emitted)", () => {
        const entries: FileEntry[] = [
            {name: "src", path: "src", is_dir: true, size: 0},
        ];
        const result = flattenTree(entries, empty.expanded, empty.children, empty.loading);
        expect(result).toHaveLength(1);
        expect(result[0]).toMatchObject({
            path: "src",
            isDir: true,
            isExpanded: false,
            isLoading: false,
            depth: 0,
        });
    });

    it("flattens expanded directory with cached children", () => {
        const entries: FileEntry[] = [
            {name: "src", path: "src", is_dir: true, size: 0},
        ];
        const expanded = new Set(["src"]);
        const children = new Map<string, readonly FileEntry[]>([
            ["src", [{name: "main.ts", path: "src/main.ts", is_dir: false, size: 50}]],
        ]);
        const result = flattenTree(entries, expanded, children, empty.loading);
        expect(result).toHaveLength(2);
        expect(result[0]).toMatchObject({path: "src", isDir: true, isExpanded: true, depth: 0});
        expect(result[1]).toMatchObject({path: "src/main.ts", isDir: false, depth: 1, size: 50});
    });

    it("does not emit children for expanded directory without cache", () => {
        const entries: FileEntry[] = [
            {name: "lib", path: "lib", is_dir: true, size: 0},
        ];
        const expanded = new Set(["lib"]);
        const result = flattenTree(entries, expanded, empty.children, empty.loading);
        expect(result).toHaveLength(1);
        expect(result[0]).toMatchObject({path: "lib", isDir: true, isExpanded: true});
    });

    it("marks loading directory nodes", () => {
        const entries: FileEntry[] = [
            {name: "src", path: "src", is_dir: true, size: 0},
        ];
        const loading = new Set(["src"]);
        const result = flattenTree(entries, empty.expanded, empty.children, loading);
        expect(result[0]).toMatchObject({isDir: true, isLoading: true});
    });

    it("recurses into nested directories with correct depth", () => {
        const entries: FileEntry[] = [
            {name: "src", path: "src", is_dir: true, size: 0},
        ];
        const expanded = new Set(["src", "src/components"]);
        const children = new Map<string, readonly FileEntry[]>([
            ["src", [{name: "components", path: "src/components", is_dir: true, size: 0}]],
            ["src/components", [{name: "App.tsx", path: "src/components/App.tsx", is_dir: false, size: 300}]],
        ]);
        const result = flattenTree(entries, expanded, children, empty.loading);
        expect(result).toHaveLength(3);
        expect(result[0]).toMatchObject({path: "src", depth: 0, isDir: true});
        expect(result[1]).toMatchObject({path: "src/components", depth: 1, isDir: true});
        expect(result[2]).toMatchObject({path: "src/components/App.tsx", depth: 2, isDir: false});
    });

    it("interleaves directories and files preserving entry order", () => {
        const entries: FileEntry[] = [
            {name: "src", path: "src", is_dir: true, size: 0},
            {name: "README.md", path: "README.md", is_dir: false, size: 1024},
            {name: "lib", path: "lib", is_dir: true, size: 0},
        ];
        const result = flattenTree(entries, empty.expanded, empty.children, empty.loading);
        expect(result.map((n) => n.name)).toEqual(["src", "README.md", "lib"]);
    });

    it("produces FlatDirNode with isDir=true discriminant and no size", () => {
        const entries: FileEntry[] = [
            {name: "dir", path: "dir", is_dir: true, size: 0},
        ];
        const result = flattenTree(entries, empty.expanded, empty.children, empty.loading);
        const node = result[0];
        expect(node.isDir).toBe(true);
        if (node.isDir) {
            // Type narrowing: these fields exist only on FlatDirNode
            expect(typeof node.isExpanded).toBe("boolean");
            expect(typeof node.isLoading).toBe("boolean");
        }
        // FlatDirNode must NOT carry a size field (S-7).
        expect("size" in node).toBe(false);
    });

    it("produces FlatFileNode with isDir=false discriminant and size", () => {
        const entries: FileEntry[] = [
            {name: "file.ts", path: "file.ts", is_dir: false, size: 512},
        ];
        const result = flattenTree(entries, empty.expanded, empty.children, empty.loading);
        const node = result[0];
        expect(node.isDir).toBe(false);
        if (!node.isDir) {
            expect(node.size).toBe(512);
        }
    });
});

describe("formatFileSize", () => {
    it.each([
        [0, "0 B"],
        [512, "512 B"],
        [1023, "1023 B"],
        [1024, "1.0 KB"],
        [1536, "1.5 KB"],
        [1048575, "1024.0 KB"],
        [1048576, "1.0 MB"],
        [10485760, "10.0 MB"],
    ])("formats %d bytes as %s", (bytes, expected) => {
        expect(formatFileSize(bytes)).toBe(expected);
    });
});
