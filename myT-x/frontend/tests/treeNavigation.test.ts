import {describe, expect, it} from "vitest";
import {
    findParentDirectoryIndex,
    type TreeNavigationNode,
} from "../src/components/viewer/views/shared/treeNavigation";

describe("findParentDirectoryIndex", () => {
    it("returns parent dir index for a file at depth 2 with parent dir at depth 1", () => {
        const nodes: TreeNavigationNode[] = [
            {depth: 0, isDir: true},  // 0: root dir
            {depth: 1, isDir: true},  // 1: parent dir
            {depth: 2, isDir: false}, // 2: target file
        ];
        expect(findParentDirectoryIndex(nodes, 2)).toBe(1);
    });

    it("returns -1 for a root node at depth 0", () => {
        const nodes: TreeNavigationNode[] = [
            {depth: 0, isDir: true},
        ];
        expect(findParentDirectoryIndex(nodes, 0)).toBe(-1);
    });

    it("returns parent index when node is at depth 1 and parent at depth 0 is a directory", () => {
        const nodes: TreeNavigationNode[] = [
            {depth: 0, isDir: true},  // 0: dir at depth 0
            {depth: 1, isDir: false}, // 1: file at depth 1
        ];
        expect(findParentDirectoryIndex(nodes, 1)).toBe(0);
    });

    it("returns -1 when node is at depth 1 but parent at depth 0 is NOT a directory", () => {
        const nodes: TreeNavigationNode[] = [
            {depth: 0, isDir: false}, // 0: file at depth 0 (not a dir)
            {depth: 1, isDir: false}, // 1: file at depth 1
        ];
        expect(findParentDirectoryIndex(nodes, 1)).toBe(-1);
    });

    it("returns -1 when no nodes before the given index exist at the target depth", () => {
        const nodes: TreeNavigationNode[] = [
            {depth: 0, isDir: true},  // 0: root dir
            {depth: 2, isDir: false}, // 1: file at depth 2 (no depth-1 dir before it)
        ];
        expect(findParentDirectoryIndex(nodes, 1)).toBe(-1);
    });

    it("returns -1 for empty nodes array", () => {
        const nodes: TreeNavigationNode[] = [];
        expect(findParentDirectoryIndex(nodes, 0)).toBe(-1);
    });

    it("returns -1 when index is out of bounds", () => {
        const nodes: TreeNavigationNode[] = [
            {depth: 0, isDir: true},
        ];
        expect(findParentDirectoryIndex(nodes, 5)).toBe(-1);
        expect(findParentDirectoryIndex(nodes, -1)).toBe(-1);
    });

    it("returns the nearest parent dir when multiple dirs exist at the target depth", () => {
        const nodes: TreeNavigationNode[] = [
            {depth: 0, isDir: true},  // 0: root
            {depth: 1, isDir: true},  // 1: dir A
            {depth: 1, isDir: true},  // 2: dir B (closer)
            {depth: 2, isDir: false}, // 3: target file
        ];
        expect(findParentDirectoryIndex(nodes, 3)).toBe(2);
    });

    it("correctly finds parent at depth 2 for deep nesting at depth 3+", () => {
        const nodes: TreeNavigationNode[] = [
            {depth: 0, isDir: true},  // 0: root
            {depth: 1, isDir: true},  // 1: level 1 dir
            {depth: 2, isDir: true},  // 2: level 2 dir
            {depth: 3, isDir: true},  // 3: level 3 dir
            {depth: 4, isDir: false}, // 4: deeply nested file
        ];
        expect(findParentDirectoryIndex(nodes, 4)).toBe(3);
        expect(findParentDirectoryIndex(nodes, 3)).toBe(2);
        expect(findParentDirectoryIndex(nodes, 2)).toBe(1);
    });
});
