import {describe, expect, it} from "vitest";
import type {DiffReviewComment} from "../../../../stores/diffReviewStore";
import {buildReviewMarkdown, buildReviewSendMarkdown} from "./diffReviewMarkdown";

function createComment(overrides: Partial<DiffReviewComment> = {}): DiffReviewComment {
    return {
        id: "1",
        sessionKey: "session:1",
        filePath: "src/auth.ts",
        startLineNum: 42,
        startLineType: "added",
        endLineNum: 42,
        endLineType: "added",
        lineContent: "const token = getToken();",
        commentText: "Add null check",
        ...overrides,
    };
}

describe("buildReviewMarkdown", () => {
    it("returns empty string for empty array", () => {
        expect(buildReviewMarkdown([])).toBe("");
    });

    it("formats single added-line comment", () => {
        const comments: DiffReviewComment[] = [createComment()];
        const result = buildReviewMarkdown(comments);
        expect(result).toBe(
            "# Code Review Comments\n\n---\n\n" +
            "## `src/auth.ts` (L+42)\n```ts\nconst token = getToken();\n```\n> Add null check",
        );
    });

    it("formats single removed-line comment", () => {
        const comments: DiffReviewComment[] = [createComment({
            id: "2",
            filePath: "src/old.ts",
            startLineNum: 10,
            startLineType: "removed",
            endLineNum: 10,
            endLineType: "removed",
            lineContent: "delete this;",
            commentText: "Why removed?",
        })];
        const result = buildReviewMarkdown(comments);
        expect(result).toContain("(L-10)");
        expect(result).toContain("```ts\ndelete this;\n```");
        expect(result).toContain("> Why removed?");
    });

    it("formats context-line comment with no prefix", () => {
        const comments: DiffReviewComment[] = [createComment({
            id: "3",
            filePath: "main.go",
            startLineNum: 5,
            startLineType: "context",
            endLineNum: 5,
            endLineType: "context",
            lineContent: "func main() {",
            commentText: "Consider renaming",
        })];
        const result = buildReviewMarkdown(comments);
        expect(result).toContain("(L5)");
        expect(result).toContain("```go\nfunc main() {\n```");
    });

    it("groups multiple comments by file", () => {
        const comments: DiffReviewComment[] = [
            createComment({filePath: "a.ts", startLineNum: 1, endLineNum: 1, lineContent: "line1", commentText: "c1"}),
            createComment({
                id: "2",
                filePath: "b.ts",
                startLineNum: 2,
                startLineType: "context",
                endLineNum: 2,
                endLineType: "context",
                lineContent: "line2",
                commentText: "c2",
            }),
            createComment({
                id: "3",
                filePath: "a.ts",
                startLineNum: 5,
                startLineType: "removed",
                endLineNum: 5,
                endLineType: "removed",
                lineContent: "line3",
                commentText: "c3",
            }),
        ];
        const result = buildReviewMarkdown(comments);
        const sections = result.split("\n\n---\n\n");
        // Map groups by file: a.ts(2 comments) then b.ts(1 comment) = header + 3 sections
        expect(sections.length).toBe(4);
        expect(sections[1]).toContain("`a.ts` (L+1)");
        expect(sections[2]).toContain("`a.ts` (L-5)");
        expect(sections[3]).toContain("`b.ts` (L2)");
    });

    it("converts multi-line commentText to blockquote", () => {
        const comments: DiffReviewComment[] = [createComment({
            id: "4",
            filePath: "x.ts",
            startLineNum: 1,
            endLineNum: 1,
            lineContent: "code",
            commentText: "line one\nline two\nline three",
        })];
        const result = buildReviewMarkdown(comments);
        expect(result).toContain("> line one\n> line two\n> line three");
    });

    it("formats multi-line ranges with a combined code block", () => {
        const comments: DiffReviewComment[] = [createComment({
            startLineNum: 7,
            startLineType: "context",
            endLineNum: 9,
            endLineType: "context",
            lineContent: "first line\nsecond line\nthird line",
        })];
        const result = buildReviewMarkdown(comments);
        expect(result).toContain("(L7 to L9)");
        expect(result).toContain("```ts\nfirst line\nsecond line\nthird line\n```");
    });

    it("produces hand-calculated output for known input", () => {
        const comments: DiffReviewComment[] = [createComment({
            id: "10",
            filePath: "pkg/util.go",
            startLineNum: 99,
            startLineType: "context",
            endLineNum: 99,
            endLineType: "context",
            lineContent: "return nil",
            commentText: "Should return error",
        })];
        const expected =
            "# Code Review Comments\n\n---\n\n" +
            "## `pkg/util.go` (L99)\n" +
            "```go\n" +
            "return nil\n" +
            "```\n" +
            "> Should return error";
        expect(buildReviewMarkdown(comments)).toBe(expected);
    });

    it("uses a longer fence when line content already contains triple backticks", () => {
        const comments: DiffReviewComment[] = [createComment({
            id: "11",
            filePath: "README.md",
            startLineNum: 7,
            startLineType: "context",
            endLineNum: 7,
            endLineType: "context",
            lineContent: "```ts",
            commentText: "Fence should remain intact",
        })];

        const result = buildReviewMarkdown(comments);

        expect(result).toContain("````md\n```ts\n````");
        expect(result).toContain("> Fence should remain intact");
    });

    it("preserves renamed file provenance in the section header", () => {
        const comments: DiffReviewComment[] = [createComment({
            filePath: "src/new-name.ts",
            oldFilePath: "src/old-name.ts",
        })];

        const result = buildReviewMarkdown(comments);

        expect(result).toContain("`src/old-name.ts -> src/new-name.ts` (L+42)");
    });

    it("uses explicit old and new labels for mixed-type ranges", () => {
        const comments: DiffReviewComment[] = [createComment({
            startLineNum: 10,
            startLineType: "removed",
            endLineNum: 14,
            endLineType: "added",
            lineContent: "old line\nnew line",
        })];

        const result = buildReviewMarkdown(comments);

        expect(result).toContain("(old L10 to new L14)");
    });
});

describe("buildReviewSendMarkdown", () => {
    it("keeps existing review markdown when the message is empty", () => {
        const comments: DiffReviewComment[] = [createComment()];

        expect(buildReviewSendMarkdown({message: "  ", comments})).toBe(buildReviewMarkdown(comments));
    });

    it("prepends a non-empty message before review comments", () => {
        const comments: DiffReviewComment[] = [createComment({commentText: "Review this"})];

        const result = buildReviewSendMarkdown({message: "Please focus on safety.", comments});

        expect(result).toContain("# Overall Comment\n\nPlease focus on safety.\n\n---\n\n# Code Review Comments");
        expect(result).toContain("> Review this");
    });

    it("preserves a user-authored top-level heading inside the overall comment section", () => {
        const result = buildReviewSendMarkdown({message: "# Important\n\nDo not rewrite my heading.", comments: []});

        expect(result).toBe("# Overall Comment\n\n# Important\n\nDo not rewrite my heading.");
    });

    it("allows message-only sends", () => {
        expect(buildReviewSendMarkdown({message: "Please review the overall diff.", comments: []})).toBe(
            "# Overall Comment\n\nPlease review the overall diff.",
        );
    });

    it("trims whitespace-only overall messages before deciding whether to add the heading", () => {
        expect(buildReviewSendMarkdown({message: "\n\t  \r\n", comments: []})).toBe("");
    });

    it("returns empty text for a completely empty payload", () => {
        expect(buildReviewSendMarkdown({message: " ", comments: []})).toBe("");
    });
});
