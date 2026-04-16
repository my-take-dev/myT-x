import {describe, expect, it} from "vitest";
import type {FileNode} from "../src/components/viewer/views/file-tree/fileTreeTypes";
import {createFileTreeStore} from "../src/stores/fileTreeStore";

function makeTree(): readonly FileNode[] {
    return [
        {
            name: "src",
            path: "src",
            isDir: true,
            hasChildren: true,
            children: [
                {
                    name: "nested",
                    path: "src/nested",
                    isDir: true,
                    hasChildren: true,
                    children: [
                        {
                            name: "file.ts",
                            path: "src/nested/file.ts",
                            isDir: false,
                            hasChildren: false,
                            size: 12,
                        },
                    ],
                },
            ],
        },
        {
            name: "README.md",
            path: "README.md",
            isDir: false,
            hasChildren: false,
            size: 5,
        },
    ];
}

describe("fileTreeStore", () => {
    it("removePath prunes descendant state alongside the tree", () => {
        const store = createFileTreeStore();

        store.getState().setRootNodes(makeTree());
        store.getState().setExpanded("src", true);
        store.getState().setExpanded("src/nested", true);
        store.getState().setExpanded("other", true);
        store.getState().setLoadingPath("src/nested", true);
        store.getState().setLoadingPath("other", true);
        store.getState().setSelectedPath("src/nested/file.ts");
        store.getState().setFileContent({
            path: "src/nested/file.ts",
            content: "console.log('x')",
            line_count: 1,
            size: 16,
            truncated: false,
            binary: false,
        });

        store.getState().removePath("src");

        const state = store.getState();
        expect(state.tree.map((node) => node.path)).toEqual(["README.md"]);
        expect(Array.from(state.expandedPaths)).toEqual(["other"]);
        expect(Array.from(state.loadingPaths)).toEqual(["other"]);
        expect(state.selectedPath).toBeNull();
        expect(state.fileContent).toBeNull();
    });

    it("renamePath remaps descendant state alongside the tree", () => {
        const store = createFileTreeStore();

        store.getState().setRootNodes(makeTree());
        store.getState().setExpanded("src", true);
        store.getState().setExpanded("src/nested", true);
        store.getState().setLoadingPath("src/nested", true);
        store.getState().setSelectedPath("src/nested/file.ts");
        store.getState().setFileContent({
            path: "src/nested/file.ts",
            content: "console.log('x')",
            line_count: 1,
            size: 16,
            truncated: false,
            binary: false,
        });

        store.getState().renamePath("src", "app");

        const state = store.getState();
        expect(state.tree[0]?.path).toBe("app");
        expect(state.tree[0]?.children?.[0]?.path).toBe("app/nested");
        expect(state.tree[0]?.children?.[0]?.children?.[0]?.path).toBe("app/nested/file.ts");
        expect(Array.from(state.expandedPaths)).toEqual(["app", "app/nested"]);
        expect(Array.from(state.loadingPaths)).toEqual(["app/nested"]);
        expect(state.selectedPath).toBe("app/nested/file.ts");
        expect(state.fileContent?.path).toBe("app/nested/file.ts");
    });
});
