import {describe, expect, it} from "vitest";
import {
    computeHunkGaps,
    diffHeaderToFilePath,
    parseDiffFiles,
    parseSingleFileDiff
} from "../src/utils/diffParser";
import type {DiffLineType} from "../src/utils/diffParser";

function expectParseSuccess(result: ReturnType<typeof parseSingleFileDiff>): Extract<ReturnType<typeof parseSingleFileDiff>, { status: "success" }> {
    if (result.status !== "success") {
        throw new Error(`Expected parse success, got status="${result.status}": ${result.message}`);
    }
    return result;
}

function expectParseError(result: ReturnType<typeof parseSingleFileDiff>): Extract<ReturnType<typeof parseSingleFileDiff>, { status: "error" }> {
    if (result.status !== "error") {
        throw new Error(`Expected parse error, got status="${result.status}"`);
    }
    return result;
}

describe("parseDiffFiles", () => {
    it("returns empty array for empty input", () => {
        expect(parseDiffFiles("")).toEqual([]);
    });

    it("parses a single file with one hunk", () => {
        const raw = [
            "diff --git a/file.ts b/file.ts",
            "index abc..def 100644",
            "--- a/file.ts",
            "+++ b/file.ts",
            "@@ -1,3 +1,4 @@",
            " line1",
            "+added",
            " line2",
            " line3",
        ].join("\n");

        const files = parseDiffFiles(raw);
        expect(files).toHaveLength(1);
        expect(files[0].header).toBe("diff --git a/file.ts b/file.ts");
        expect(files[0].hunks).toHaveLength(1);

        const hunk = files[0].hunks[0];
        expect(hunk.startOldLine).toBe(1);
        expect(hunk.startNewLine).toBe(1);
        expect(hunk.lines).toHaveLength(4);
        expect(hunk.lines[0]).toEqual({type: "context", content: "line1", oldLineNum: 1, newLineNum: 1});
        expect(hunk.lines[1]).toEqual({type: "added", content: "added", newLineNum: 2});
        expect(hunk.lines[2]).toEqual({type: "context", content: "line2", oldLineNum: 2, newLineNum: 3});
        expect(hunk.lines[3]).toEqual({type: "context", content: "line3", oldLineNum: 3, newLineNum: 4});
    });

    it("parses multiple files", () => {
        const raw = [
            "diff --git a/a.ts b/a.ts",
            "@@ -1,1 +1,1 @@",
            "-old",
            "+new",
            "diff --git a/b.ts b/b.ts",
            "@@ -1,1 +1,2 @@",
            " unchanged",
            "+added",
        ].join("\n");

        const files = parseDiffFiles(raw);
        expect(files).toHaveLength(2);
        expect(files[0].header).toContain("a.ts");
        expect(files[1].header).toContain("b.ts");
        expect(files[0].hunks[0].lines).toHaveLength(2);
        expect(files[1].hunks[0].lines).toHaveLength(2);
    });

    it("parses file with multiple hunks", () => {
        const raw = [
            "diff --git a/file.ts b/file.ts",
            "@@ -1,2 +1,2 @@",
            "-old1",
            "+new1",
            " ctx",
            "@@ -10,2 +10,3 @@",
            " ctx2",
            "+added",
            " ctx3",
        ].join("\n");

        const files = parseDiffFiles(raw);
        expect(files).toHaveLength(1);
        expect(files[0].hunks).toHaveLength(2);
        expect(files[0].hunks[0].startOldLine).toBe(1);
        expect(files[0].hunks[1].startOldLine).toBe(10);
    });

    it("handles orphan hunk without preceding file header", () => {
        const raw = [
            "@@ -1,1 +1,2 @@",
            " line1",
            "+line2",
        ].join("\n");

        const files = parseDiffFiles(raw);
        expect(files).toHaveLength(1);
        expect(files[0].header).toBeNull();
        expect(files[0].hunks).toHaveLength(1);
    });

    it("skips metadata lines", () => {
        const raw = [
            "diff --git a/file.ts b/file.ts",
            "new file mode 100644",
            "index 0000000..abc1234",
            "--- /dev/null",
            "+++ b/file.ts",
            "@@ -0,0 +1,2 @@",
            "+line1",
            "+line2",
        ].join("\n");

        const files = parseDiffFiles(raw);
        expect(files).toHaveLength(1);
        expect(files[0].hunks[0].lines).toHaveLength(2);
        expect(files[0].hunks[0].lines[0].type).toBe("added");
    });

    it("skips \\ No newline at end of file", () => {
        const raw = [
            "diff --git a/file.ts b/file.ts",
            "@@ -1,1 +1,1 @@",
            "-old",
            "+new",
            "\\ No newline at end of file",
        ].join("\n");

        const files = parseDiffFiles(raw);
        expect(files[0].hunks[0].lines).toHaveLength(2);
    });

    it("handles removed lines with line number tracking", () => {
        const raw = [
            "diff --git a/file.ts b/file.ts",
            "@@ -5,3 +5,2 @@",
            " ctx",
            "-removed",
            " ctx2",
        ].join("\n");

        const files = parseDiffFiles(raw);
        const lines = files[0].hunks[0].lines;
        expect(lines[0]).toEqual({type: "context", content: "ctx", oldLineNum: 5, newLineNum: 5});
        expect(lines[1]).toEqual({type: "removed", content: "removed", oldLineNum: 6});
        expect(lines[2]).toEqual({type: "context", content: "ctx2", oldLineNum: 7, newLineNum: 6});
    });

    it("handles malformed hunk header gracefully", () => {
        const raw = [
            "diff --git a/file.ts b/file.ts",
            "@@ invalid header @@",
            "+should be skipped",
            "@@ -1,1 +1,1 @@",
            "+valid",
        ].join("\n");

        const files = parseDiffFiles(raw);
        expect(files[0].hunks).toHaveLength(1);
        expect(files[0].hunks[0].lines[0].content).toBe("valid");
    });

    it("handles context line without leading space", () => {
        const raw = [
            "diff --git a/file.ts b/file.ts",
            "@@ -1,1 +1,1 @@",
            "context without space prefix",
        ].join("\n");

        const files = parseDiffFiles(raw);
        expect(files[0].hunks[0].lines[0].type).toBe("context");
        expect(files[0].hunks[0].lines[0].content).toBe("context without space prefix");
    });

    it("keeps removed/added content lines that begin with metadata-like prefixes", () => {
        const raw = [
            "diff --git a/doc.md b/doc.md",
            "@@ -1,2 +1,2 @@",
            "----",
            "++++",
        ].join("\n");

        const files = parseDiffFiles(raw);
        expect(files).toHaveLength(1);
        expect(files[0].hunks).toHaveLength(1);
        expect(files[0].hunks[0].lines).toEqual([
            {type: "removed", content: "---", oldLineNum: 1},
            {type: "added", content: "+++", newLineNum: 1},
        ]);
    });

    // ── New file addition (@@ -0,0 +1,N @@) ──

    it("parses new file addition with @@ -0,0 +1 @@", () => {
        const raw = [
            "diff --git a/newfile.ts b/newfile.ts",
            "new file mode 100644",
            "index 0000000..abc1234",
            "--- /dev/null",
            "+++ b/newfile.ts",
            "@@ -0,0 +1,3 @@",
            "+line1",
            "+line2",
            "+line3",
        ].join("\n");

        const files = parseDiffFiles(raw);
        expect(files).toHaveLength(1);
        expect(files[0].hunks).toHaveLength(1);
        expect(files[0].hunks[0].startOldLine).toBe(0);
        expect(files[0].hunks[0].startNewLine).toBe(1);
        expect(files[0].hunks[0].lines).toHaveLength(3);
        expect(files[0].hunks[0].lines.every(l => l.type === "added")).toBe(true);
        expect(files[0].hunks[0].lines[0].newLineNum).toBe(1);
        expect(files[0].hunks[0].lines[2].newLineNum).toBe(3);
    });

    // ── Rename diff ──

    it("parses rename diff with rename from/to metadata", () => {
        const raw = [
            "diff --git a/old/path.ts b/new/path.ts",
            "similarity index 95%",
            "rename from old/path.ts",
            "rename to new/path.ts",
            "index abc..def 100644",
            "--- a/old/path.ts",
            "+++ b/new/path.ts",
            "@@ -1,2 +1,2 @@",
            " unchanged",
            "-old line",
            "+new line",
        ].join("\n");

        const files = parseDiffFiles(raw);
        expect(files).toHaveLength(1);
        expect(files[0].header).toBe("diff --git a/old/path.ts b/new/path.ts");
        expect(files[0].hunks).toHaveLength(1);
        expect(files[0].hunks[0].lines).toHaveLength(3);
    });

    // ── Binary file diff ──

    it("parses binary file diff (no hunks)", () => {
        // "Binary files ..." is not matched by isMetadataLine, but with no currentHunk
        // it's silently skipped — resulting in a file entry with no hunks. This is correct.
        const raw = [
            "diff --git a/image.png b/image.png",
            "index abc..def 100644",
            "Binary files a/image.png and b/image.png differ",
        ].join("\n");

        const files = parseDiffFiles(raw);
        expect(files).toHaveLength(1);
        expect(files[0].header).toBe("diff --git a/image.png b/image.png");
        expect(files[0].hunks).toHaveLength(0);
    });

    // ── Diff with no hunks (header only) ──

    it("parses file header with no hunks", () => {
        const raw = [
            "diff --git a/empty.ts b/empty.ts",
            "index abc..def 100644",
        ].join("\n");

        const files = parseDiffFiles(raw);
        expect(files).toHaveLength(1);
        expect(files[0].header).toBe("diff --git a/empty.ts b/empty.ts");
        expect(files[0].hunks).toHaveLength(0);
    });

    // ── Single file diff (no diff --git header, just hunk) ──

    it("handles hunk-only input without diff --git header", () => {
        const raw = [
            "@@ -1,2 +1,3 @@",
            " existing",
            "+added1",
            "+added2",
            " existing2",
        ].join("\n");

        const files = parseDiffFiles(raw);
        expect(files).toHaveLength(1);
        expect(files[0].header).toBeNull();
        expect(files[0].hunks).toHaveLength(1);
        expect(files[0].hunks[0].lines).toHaveLength(4);
    });

    // ── Multiple files with varying content ──

    it("parses three files with different change types", () => {
        const raw = [
            "diff --git a/add.ts b/add.ts",
            "new file mode 100644",
            "--- /dev/null",
            "+++ b/add.ts",
            "@@ -0,0 +1,1 @@",
            "+new content",
            "diff --git a/modify.ts b/modify.ts",
            "@@ -1,1 +1,1 @@",
            "-old",
            "+new",
            "diff --git a/delete.ts b/delete.ts",
            "deleted file mode 100644",
            "--- a/delete.ts",
            "+++ /dev/null",
            "@@ -1,2 +0,0 @@",
            "-line1",
            "-line2",
        ].join("\n");

        const files = parseDiffFiles(raw);
        expect(files).toHaveLength(3);
        expect(files[0].hunks[0].lines[0].type).toBe("added");
        expect(files[1].hunks[0].lines[0].type).toBe("removed");
        expect(files[2].hunks[0].lines.every(l => l.type === "removed")).toBe(true);
    });

    // ── Whitespace-only diff content ──

    it("returns empty array for whitespace-only input", () => {
        expect(parseDiffFiles("   \n  \n")).toEqual([]);
    });
});

describe("parseSingleFileDiff", () => {
    it("returns empty hunks and gaps for empty input", () => {
        const result = expectParseSuccess(parseSingleFileDiff(""));
        expect(result.hunks).toEqual([]);
        expect(result.gaps.size).toBe(0);
        expect(result.fileCount).toBe(0);
    });

    it("returns first file with hunks", () => {
        const raw = [
            "diff --git a/empty.ts b/empty.ts",
            "diff --git a/file.ts b/file.ts",
            "@@ -1,1 +1,1 @@",
            "-old",
            "+new",
        ].join("\n");

        const result = expectParseSuccess(parseSingleFileDiff(raw));
        expect(result.hunks).toHaveLength(1);
        expect(result.fileCount).toBe(2);
    });

    it("returns fileCount=1 for single-file input", () => {
        const raw = [
            "diff --git a/file.ts b/file.ts",
            "@@ -1,1 +1,1 @@",
            "-old",
            "+new",
        ].join("\n");

        const result = expectParseSuccess(parseSingleFileDiff(raw));
        expect(result.fileCount).toBe(1);
    });

    it("returns fileCount=2 for multi-file input", () => {
        const raw = [
            "diff --git a/a.ts b/a.ts",
            "@@ -1,1 +1,1 @@",
            "-a",
            "+b",
            "diff --git a/b.ts b/b.ts",
            "@@ -1,1 +1,1 @@",
            "-c",
            "+d",
        ].join("\n");

        const result = expectParseSuccess(parseSingleFileDiff(raw));
        expect(result.fileCount).toBe(2);
    });

    it("skips first file when it has no hunks and uses first file with hunks", () => {
        const raw = [
            "diff --git a/no-hunk.ts b/no-hunk.ts",
            "index abc..def 100644",
            "diff --git a/with-hunk.ts b/with-hunk.ts",
            "@@ -1,1 +1,1 @@",
            "-old",
            "+new",
        ].join("\n");

        const result = expectParseSuccess(parseSingleFileDiff(raw));
        expect(result.fileCount).toBe(2);
        expect(result.hunks).toHaveLength(1);
        expect(result.hunks[0].header).toBe("@@ -1,1 +1,1 @@");
    });

    it("returns status='error' when hunk headers exist but none are valid", () => {
        // The hunk header matches the hasHunkHeader regex (@@ -\d+) but fails
        // parseHunkHeader's stricter validation, so no hunks are produced.
        const raw = [
            "diff --git a/file.ts b/file.ts",
            "@@ -1 invalid rest @@",
            "+skipped",
        ].join("\n");

        const result = expectParseError(parseSingleFileDiff(raw));
        expect(result.message).toMatch(/hunk headers found but not recognized/i);
    });

    it("succeeds when one file has invalid @@ and another has valid @@", () => {
        const raw = [
            "diff --git a/bad.ts b/bad.ts",
            "@@ -1 invalid @@",
            "+skipped",
            "diff --git a/good.ts b/good.ts",
            "@@ -1,1 +1,1 @@",
            "-old",
            "+new",
        ].join("\n");

        const result = expectParseSuccess(parseSingleFileDiff(raw));
        expect(result.fileCount).toBe(2);
        expect(result.hunks).toHaveLength(1);
        expect(result.hunks[0].lines[0].content).toBe("old");
    });

    it("returns status='error' for non-string runtime input", () => {
        const result = expectParseError(parseSingleFileDiff(null as unknown as string));
        // Generic fallback message — technical details logged to console, not exposed in UI.
        expect(result.message).toMatch(/failed to parse diff/i);
    });

});

describe("computeHunkGaps", () => {
    it("returns empty map for empty hunks", () => {
        expect(computeHunkGaps([]).size).toBe(0);
    });

    it("returns empty map for single hunk", () => {
        const hunks = [{
            header: "@@ -1,2 +1,2 @@",
            lines: [
                {type: "context" as const, content: "a", oldLineNum: 1, newLineNum: 1},
                {type: "context" as const, content: "b", oldLineNum: 2, newLineNum: 2},
            ],
            startOldLine: 1,
            startNewLine: 1,
        }];
        expect(computeHunkGaps(hunks).size).toBe(0);
    });

    it("computes gaps between non-adjacent hunks", () => {
        const ctx: DiffLineType = "context";
        const hunks = [
            {
                header: "@@ -1,2 +1,2 @@",
                lines: [
                    {type: ctx, content: "a", oldLineNum: 1, newLineNum: 1},
                    {type: ctx, content: "b", oldLineNum: 2, newLineNum: 2},
                ],
                startOldLine: 1,
                startNewLine: 1,
            },
            {
                header: "@@ -10,1 +10,1 @@",
                lines: [
                    {type: ctx, content: "c", oldLineNum: 10, newLineNum: 10},
                ],
                startOldLine: 10,
                startNewLine: 10,
            },
        ];

        const gaps = computeHunkGaps(hunks);
        expect(gaps.size).toBe(1);
        const gap = gaps.get(0);
        expect(gap).toBeDefined();
        expect(gap!.afterHunkIndex).toBe(0);
        expect(gap!.hiddenLineCount).toBe(7); // Lines 3-9 are hidden
    });

    it("no gap when hunks are adjacent", () => {
        const hunks = [
            {
                header: "@@ -1,2 +1,2 @@",
                lines: [
                    {type: "context" as const, content: "a", oldLineNum: 1, newLineNum: 1},
                    {type: "context" as const, content: "b", oldLineNum: 2, newLineNum: 2},
                ],
                startOldLine: 1,
                startNewLine: 1,
            },
            {
                header: "@@ -3,1 +3,1 @@",
                lines: [
                    {type: "context" as const, content: "c", oldLineNum: 3, newLineNum: 3},
                ],
                startOldLine: 3,
                startNewLine: 3,
            },
        ];

        expect(computeHunkGaps(hunks).size).toBe(0);
    });

    it("computes gaps between three non-adjacent hunks", () => {
        const hunks = [
            {
                header: "@@ -1,1 +1,1 @@",
                lines: [
                    {type: "context" as const, content: "a", oldLineNum: 1, newLineNum: 1},
                ],
                startOldLine: 1,
                startNewLine: 1,
            },
            {
                header: "@@ -5,1 +5,1 @@",
                lines: [
                    {type: "context" as const, content: "b", oldLineNum: 5, newLineNum: 5},
                ],
                startOldLine: 5,
                startNewLine: 5,
            },
            {
                header: "@@ -20,1 +20,1 @@",
                lines: [
                    {type: "context" as const, content: "c", oldLineNum: 20, newLineNum: 20},
                ],
                startOldLine: 20,
                startNewLine: 20,
            },
        ];

        const gaps = computeHunkGaps(hunks);
        expect(gaps.size).toBe(2);
        expect(gaps.get(0)!.hiddenLineCount).toBe(3); // Lines 2,3,4
        expect(gaps.get(1)!.hiddenLineCount).toBe(14); // Lines 6-19
    });

    it("accounts for removed lines consuming old line numbers", () => {
        const hunks = [
            {
                header: "@@ -1,3 +1,1 @@",
                lines: [
                    {type: "context" as const, content: "a", oldLineNum: 1, newLineNum: 1},
                    {type: "removed" as const, content: "b", oldLineNum: 2},
                    {type: "removed" as const, content: "c", oldLineNum: 3},
                ],
                startOldLine: 1,
                startNewLine: 1,
            },
            {
                header: "@@ -6,1 +4,1 @@",
                lines: [
                    {type: "context" as const, content: "d", oldLineNum: 6, newLineNum: 4},
                ],
                startOldLine: 6,
                startNewLine: 4,
            },
        ];

        const gaps = computeHunkGaps(hunks);
        expect(gaps.size).toBe(1);
        // oldLineCursor after first hunk: 1 + 3 (context + 2 removed) = 4
        // next startOldLine: 6 → gap = 6 - 4 = 2
        expect(gaps.get(0)!.hiddenLineCount).toBe(2);
    });

    it("returns zero-gap for adjacent hunks where first ends exactly where second begins", () => {
        // Hunk 1: starts at old line 1, has 3 context lines (consumes 1,2,3)
        // oldLineCursor after hunk 1 = 1 + 3 = 4
        // Hunk 2: starts at old line 4 — exactly adjacent
        const hunks = [
            {
                header: "@@ -1,3 +1,3 @@",
                lines: [
                    {type: "context" as const, content: "a", oldLineNum: 1, newLineNum: 1},
                    {type: "context" as const, content: "b", oldLineNum: 2, newLineNum: 2},
                    {type: "context" as const, content: "c", oldLineNum: 3, newLineNum: 3},
                ],
                startOldLine: 1,
                startNewLine: 1,
            },
            {
                header: "@@ -4,2 +4,2 @@",
                lines: [
                    {type: "context" as const, content: "d", oldLineNum: 4, newLineNum: 4},
                    {type: "context" as const, content: "e", oldLineNum: 5, newLineNum: 5},
                ],
                startOldLine: 4,
                startNewLine: 4,
            },
        ];

        // Gap = next.startOldLine - oldLineCursorAfterHunk(current) = 4 - 4 = 0
        // Since hiddenLineCount is 0, the gap is NOT added to the map
        expect(computeHunkGaps(hunks).size).toBe(0);
    });

    it("handles a single hunk (no gaps possible)", () => {
        const hunks = [
            {
                header: "@@ -5,4 +5,5 @@",
                lines: [
                    {type: "context" as const, content: "a", oldLineNum: 5, newLineNum: 5},
                    {type: "removed" as const, content: "b", oldLineNum: 6},
                    {type: "added" as const, content: "c", newLineNum: 6},
                    {type: "added" as const, content: "d", newLineNum: 7},
                    {type: "context" as const, content: "e", oldLineNum: 7, newLineNum: 8},
                    {type: "context" as const, content: "f", oldLineNum: 8, newLineNum: 9},
                ],
                startOldLine: 5,
                startNewLine: 5,
            },
        ];

        // Only one hunk — gaps map should be empty
        expect(computeHunkGaps(hunks).size).toBe(0);
    });

    it("accounts for added lines not consuming old line numbers", () => {
        const hunks = [
            {
                header: "@@ -1,1 +1,3 @@",
                lines: [
                    {type: "context" as const, content: "a", oldLineNum: 1, newLineNum: 1},
                    {type: "added" as const, content: "b", newLineNum: 2},
                    {type: "added" as const, content: "c", newLineNum: 3},
                ],
                startOldLine: 1,
                startNewLine: 1,
            },
            {
                header: "@@ -5,1 +7,1 @@",
                lines: [
                    {type: "context" as const, content: "d", oldLineNum: 5, newLineNum: 7},
                ],
                startOldLine: 5,
                startNewLine: 7,
            },
        ];

        const gaps = computeHunkGaps(hunks);
        expect(gaps.size).toBe(1);
        expect(gaps.get(0)!.hiddenLineCount).toBe(3); // Lines 2,3,4 are hidden
    });
});

describe("diffHeaderToFilePath", () => {
    it("extracts file path from standard header", () => {
        expect(diffHeaderToFilePath("diff --git a/src/main.ts b/src/main.ts")).toBe("src/main.ts");
    });

    it("extracts new path from rename header", () => {
        expect(diffHeaderToFilePath("diff --git a/old/path.ts b/new/path.ts")).toBe("new/path.ts");
    });

    it("returns raw header when format does not match", () => {
        expect(diffHeaderToFilePath("not a diff header")).toBe("not a diff header");
    });

    it("handles paths with spaces", () => {
        expect(diffHeaderToFilePath("diff --git a/my file.ts b/my file.ts")).toBe("my file.ts");
    });

    it("handles deeply nested paths", () => {
        expect(diffHeaderToFilePath("diff --git a/a/b/c/d/e.ts b/a/b/c/d/e.ts")).toBe("a/b/c/d/e.ts");
    });

    it("returns (untitled) for null header", () => {
        expect(diffHeaderToFilePath(null)).toBe("(untitled)");
    });

    it("returns raw string for empty string", () => {
        expect(diffHeaderToFilePath("")).toBe("");
    });

    it("handles path with special characters", () => {
        expect(diffHeaderToFilePath("diff --git a/src/[utils].ts b/src/[utils].ts")).toBe("src/[utils].ts");
    });

    it("handles path with dots", () => {
        expect(diffHeaderToFilePath("diff --git a/src/file.test.spec.ts b/src/file.test.spec.ts")).toBe("src/file.test.spec.ts");
    });

    it("handles single filename (no directory)", () => {
        expect(diffHeaderToFilePath("diff --git a/README.md b/README.md")).toBe("README.md");
    });
});
