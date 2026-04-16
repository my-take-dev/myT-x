import {describe, expect, it} from "vitest";
import {buildDiffTree, collectDirectorySet} from "./diffTreeUtils";
import type {WorkingDiffFile} from "./diffViewTypes";

function diffFile(path: string, additions: number, deletions: number): WorkingDiffFile {
    return {
        path,
        old_path: "",
        status: "modified",
        additions,
        deletions,
        diff: "",
    };
}

describe("diffTreeUtils", () => {
    it("builds a flat tree that respects expanded directories", () => {
        const files = [
            diffFile("src/app.ts", 10, 2),
            diffFile("src/nested/deep.ts", 4, 1),
            diffFile("README.md", 1, 0),
        ];

        const nodes = buildDiffTree(files, new Set(["src"]));
        expect(nodes.map((node) => node.path)).toEqual([
            "README.md",
            "src",
            "src/app.ts",
            "src/nested",
        ]);
    });

    it("expands nested files when every ancestor is expanded", () => {
        const files = [diffFile("src/nested/deep.ts", 4, 1)];

        const nodes = buildDiffTree(files, new Set(["src", "src/nested"]));
        expect(nodes.map((node) => node.path)).toEqual([
            "src",
            "src/nested",
            "src/nested/deep.ts",
        ]);
    });

    it("records hand-calculated depth and expansion metadata", () => {
        const files = [
            diffFile("src/app.ts", 10, 2),
            diffFile("src/nested/deep.ts", 4, 1),
        ];

        expect(buildDiffTree(files, new Set(["src"]))).toEqual([
            {
                path: "src",
                name: "src",
                isDir: true,
                depth: 0,
                isExpanded: true,
            },
            {
                path: "src/app.ts",
                name: "app.ts",
                isDir: false,
                depth: 1,
                file: diffFile("src/app.ts", 10, 2),
            },
            {
                path: "src/nested",
                name: "nested",
                isDir: true,
                depth: 1,
                isExpanded: false,
            },
        ]);
    });

    it("collects unique directory paths from diff files", () => {
        const files = [
            diffFile("src/app.ts", 1, 0),
            diffFile("src/nested/deep.ts", 1, 0),
            diffFile("docs/spec.md", 1, 0),
        ];

        expect([...collectDirectorySet(files)].sort()).toEqual([
            "docs",
            "src",
            "src/nested",
        ]);
    });
});
