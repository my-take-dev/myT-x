import {describe, expect, it} from "vitest";
import {extractSelectedText, type SelectionSpan} from "../src/utils/selectionUtils";
import {LINE_ENDING_CRLF, LINE_ENDING_LF} from "../src/utils/textLines";

describe("extractSelectedText", () => {
    // ── Empty / degenerate inputs ──

    describe("empty lines array", () => {
        it("returns empty string for empty lines", () => {
            const span: SelectionSpan = {startLineIndex: 0, endLineIndex: 0, startOffset: 0, endOffset: 0};
            expect(extractSelectedText([], span, LINE_ENDING_LF)).toBe("");
        });

        it("returns empty string for empty lines with non-zero span", () => {
            const span: SelectionSpan = {startLineIndex: 0, endLineIndex: 5, startOffset: 0, endOffset: 10};
            expect(extractSelectedText([], span, LINE_ENDING_LF)).toBe("");
        });
    });

    // ── Single line selection ──

    describe("single line selection", () => {
        const lines = ["hello world"];

        it("selects entire line", () => {
            const span: SelectionSpan = {startLineIndex: 0, endLineIndex: 0, startOffset: 0, endOffset: 11};
            expect(extractSelectedText(lines, span, LINE_ENDING_LF)).toBe("hello world");
        });

        it("selects substring", () => {
            const span: SelectionSpan = {startLineIndex: 0, endLineIndex: 0, startOffset: 6, endOffset: 11};
            expect(extractSelectedText(lines, span, LINE_ENDING_LF)).toBe("world");
        });

        it("selects nothing when offsets are equal", () => {
            const span: SelectionSpan = {startLineIndex: 0, endLineIndex: 0, startOffset: 3, endOffset: 3};
            expect(extractSelectedText(lines, span, LINE_ENDING_LF)).toBe("");
        });

        it("handles reverse direction drag (right-to-left) on same line", () => {
            // endOffset < startOffset: user dragged from right to left
            const span: SelectionSpan = {startLineIndex: 0, endLineIndex: 0, startOffset: 11, endOffset: 6};
            expect(extractSelectedText(lines, span, LINE_ENDING_LF)).toBe("world");
        });

        it("handles same-line reverse offset (right-to-left selection)", () => {
            const lines = ["Hello, World!"];
            // startOffset=12 > endOffset=5, so should extract from offset 5 to 12
            const span: SelectionSpan = {startLineIndex: 0, endLineIndex: 0, startOffset: 12, endOffset: 5};
            expect(extractSelectedText(lines, span, LINE_ENDING_LF)).toBe(", World");
        });

        it("selects first character only", () => {
            const span: SelectionSpan = {startLineIndex: 0, endLineIndex: 0, startOffset: 0, endOffset: 1};
            expect(extractSelectedText(lines, span, LINE_ENDING_LF)).toBe("h");
        });

        it("selects last character only", () => {
            const span: SelectionSpan = {startLineIndex: 0, endLineIndex: 0, startOffset: 10, endOffset: 11};
            expect(extractSelectedText(lines, span, LINE_ENDING_LF)).toBe("d");
        });
    });

    // ── Multi-line selection ──

    describe("multi-line selection", () => {
        const lines = ["first line", "second line", "third line", "fourth line"];

        it("selects across two lines", () => {
            const span: SelectionSpan = {startLineIndex: 0, endLineIndex: 1, startOffset: 6, endOffset: 6};
            expect(extractSelectedText(lines, span, LINE_ENDING_LF)).toBe("line\nsecond");
        });

        it("selects across three lines (includes full middle line)", () => {
            const span: SelectionSpan = {startLineIndex: 0, endLineIndex: 2, startOffset: 6, endOffset: 5};
            expect(extractSelectedText(lines, span, LINE_ENDING_LF)).toBe("line\nsecond line\nthird");
        });

        it("selects all lines from start to end", () => {
            const span: SelectionSpan = {startLineIndex: 0, endLineIndex: 3, startOffset: 0, endOffset: 11};
            expect(extractSelectedText(lines, span, LINE_ENDING_LF)).toBe("first line\nsecond line\nthird line\nfourth line");
        });

        it("respects CRLF line ending", () => {
            const span: SelectionSpan = {startLineIndex: 0, endLineIndex: 1, startOffset: 6, endOffset: 6};
            expect(extractSelectedText(lines, span, LINE_ENDING_CRLF)).toBe("line\r\nsecond");
        });
    });

    // ── Reverse (bottom-to-top) multi-line drag selection ──

    describe("reverse direction drag selection (bottom-to-top)", () => {
        const lines = ["first line", "second line", "third line"];

        it("normalizes reverse multi-line selection", () => {
            // User drags from line 2 to line 0 (bottom-to-top)
            const span: SelectionSpan = {startLineIndex: 2, endLineIndex: 0, startOffset: 5, endOffset: 6};
            // After normalization: startLine=0, endLine=2, startOffset=6, endOffset=5
            expect(extractSelectedText(lines, span, LINE_ENDING_LF)).toBe("line\nsecond line\nthird");
        });

        it("normalizes reverse two-line selection", () => {
            const span: SelectionSpan = {startLineIndex: 1, endLineIndex: 0, startOffset: 3, endOffset: 0};
            // After normalization: startLine=0, endLine=1, startOffset=0, endOffset=3
            expect(extractSelectedText(lines, span, LINE_ENDING_LF)).toBe("first line\nsec");
        });
    });

    // ── Boundary: line indices beyond bounds ──

    describe("out-of-bounds line indices", () => {
        const lines = ["alpha", "beta", "gamma"];

        it("clamps negative startLineIndex to 0", () => {
            const span: SelectionSpan = {startLineIndex: -1, endLineIndex: 0, startOffset: 0, endOffset: 3};
            expect(extractSelectedText(lines, span, LINE_ENDING_LF)).toBe("alp");
        });

        it("clamps negative startOffset to 0", () => {
            const span: SelectionSpan = {startLineIndex: 0, endLineIndex: 0, startOffset: -5, endOffset: 3};
            expect(extractSelectedText(lines, span, LINE_ENDING_LF)).toBe("alp");
        });

        it("clamps startLineIndex beyond length", () => {
            const span: SelectionSpan = {startLineIndex: 100, endLineIndex: 100, startOffset: 0, endOffset: 5};
            // Both clamped to lines.length-1 = 2 (gamma)
            expect(extractSelectedText(lines, span, LINE_ENDING_LF)).toBe("gamma");
        });

        it("clamps endLineIndex beyond length", () => {
            const span: SelectionSpan = {startLineIndex: 0, endLineIndex: 100, startOffset: 3, endOffset: 100};
            // endIndex clamped to 2, endOffset clamped to codePointLength("gamma") = 5
            expect(extractSelectedText(lines, span, LINE_ENDING_LF)).toBe("ha\nbeta\ngamma");
        });

        it("clamps negative-like zero start", () => {
            // Math.max(0, ...) ensures negative indices become 0
            const span: SelectionSpan = {startLineIndex: 0, endLineIndex: 0, startOffset: 0, endOffset: 3};
            expect(extractSelectedText(lines, span, LINE_ENDING_LF)).toBe("alp");
        });
    });

    // ── Boundary: offset clamping ──

    describe("offset clamping", () => {
        const lines = ["short"];

        it("clamps endOffset beyond line length", () => {
            const span: SelectionSpan = {startLineIndex: 0, endLineIndex: 0, startOffset: 0, endOffset: 100};
            expect(extractSelectedText(lines, span, LINE_ENDING_LF)).toBe("short");
        });

        it("clamps startOffset beyond line length to produce empty", () => {
            const span: SelectionSpan = {startLineIndex: 0, endLineIndex: 0, startOffset: 100, endOffset: 100};
            // Both clamped to 5 → lo=5, hi=5 → empty
            expect(extractSelectedText(lines, span, LINE_ENDING_LF)).toBe("");
        });
    });

    // ── Unicode / emoji selection ──

    describe("Unicode emoji selection", () => {
        const lines = ["hello\u{1F600}world", "\u{1F601}\u{1F602}\u{1F603}"];

        it("selects emoji character on single line", () => {
            // "hello" = 5 code points, emoji = 1 code point, "world" = 5 code points
            const span: SelectionSpan = {startLineIndex: 0, endLineIndex: 0, startOffset: 5, endOffset: 6};
            expect(extractSelectedText(lines, span, LINE_ENDING_LF)).toBe("\u{1F600}");
        });

        it("selects across emoji boundary", () => {
            const span: SelectionSpan = {startLineIndex: 0, endLineIndex: 0, startOffset: 4, endOffset: 7};
            expect(extractSelectedText(lines, span, LINE_ENDING_LF)).toBe("o\u{1F600}w");
        });

        it("selects emoji-only line", () => {
            const span: SelectionSpan = {startLineIndex: 1, endLineIndex: 1, startOffset: 0, endOffset: 3};
            expect(extractSelectedText(lines, span, LINE_ENDING_LF)).toBe("\u{1F601}\u{1F602}\u{1F603}");
        });

        it("selects across lines with emoji", () => {
            const span: SelectionSpan = {startLineIndex: 0, endLineIndex: 1, startOffset: 5, endOffset: 2};
            expect(extractSelectedText(lines, span, LINE_ENDING_LF)).toBe("\u{1F600}world\n\u{1F601}\u{1F602}");
        });

        it("reverse drag across emoji lines", () => {
            const span: SelectionSpan = {startLineIndex: 1, endLineIndex: 0, startOffset: 1, endOffset: 10};
            // Normalized: startLine=0 offset=10, endLine=1 offset=1
            expect(extractSelectedText(lines, span, LINE_ENDING_LF)).toBe("d\n\u{1F601}");
        });
    });

    // ── Empty lines ──

    describe("empty lines", () => {
        const lines = ["", "content", ""];

        it("selects from empty line to content line", () => {
            const span: SelectionSpan = {startLineIndex: 0, endLineIndex: 1, startOffset: 0, endOffset: 7};
            expect(extractSelectedText(lines, span, LINE_ENDING_LF)).toBe("\ncontent");
        });

        it("selects from content line to empty line", () => {
            const span: SelectionSpan = {startLineIndex: 1, endLineIndex: 2, startOffset: 0, endOffset: 0};
            expect(extractSelectedText(lines, span, LINE_ENDING_LF)).toBe("content\n");
        });

        it("selects across all with empty lines", () => {
            const span: SelectionSpan = {startLineIndex: 0, endLineIndex: 2, startOffset: 0, endOffset: 0};
            expect(extractSelectedText(lines, span, LINE_ENDING_LF)).toBe("\ncontent\n");
        });

        it("selects within empty line returns empty string", () => {
            const span: SelectionSpan = {startLineIndex: 0, endLineIndex: 0, startOffset: 0, endOffset: 0};
            expect(extractSelectedText(lines, span, LINE_ENDING_LF)).toBe("");
        });
    });

    // ── CJK characters ──

    describe("CJK characters", () => {
        const lines = ["\u4F60\u597D\u4E16\u754C"];  // 你好世界

        it("selects CJK substring", () => {
            const span: SelectionSpan = {startLineIndex: 0, endLineIndex: 0, startOffset: 1, endOffset: 3};
            expect(extractSelectedText(lines, span, LINE_ENDING_LF)).toBe("\u597D\u4E16");
        });

        it("selects entire CJK line", () => {
            const span: SelectionSpan = {startLineIndex: 0, endLineIndex: 0, startOffset: 0, endOffset: 4};
            expect(extractSelectedText(lines, span, LINE_ENDING_LF)).toBe("\u4F60\u597D\u4E16\u754C");
        });

        it("selects first CJK character only", () => {
            const span: SelectionSpan = {startLineIndex: 0, endLineIndex: 0, startOffset: 0, endOffset: 1};
            expect(extractSelectedText(lines, span, LINE_ENDING_LF)).toBe("\u4F60");
        });

        it("selects last CJK character only", () => {
            const span: SelectionSpan = {startLineIndex: 0, endLineIndex: 0, startOffset: 3, endOffset: 4};
            expect(extractSelectedText(lines, span, LINE_ENDING_LF)).toBe("\u754C");
        });
    });

    // ── CJK supplementary plane characters (surrogate pairs) ──

    describe("CJK supplementary plane (surrogate pairs)", () => {
        // U+20000 (CJK Unified Ideographs Extension B) — requires surrogate pair in UTF-16
        const lines = ["\u{20000}\u{20001}\u{20002}"];

        it("selects single CJK supplementary character", () => {
            const span: SelectionSpan = {startLineIndex: 0, endLineIndex: 0, startOffset: 0, endOffset: 1};
            expect(extractSelectedText(lines, span, LINE_ENDING_LF)).toBe("\u{20000}");
        });

        it("selects middle CJK supplementary character", () => {
            const span: SelectionSpan = {startLineIndex: 0, endLineIndex: 0, startOffset: 1, endOffset: 2};
            expect(extractSelectedText(lines, span, LINE_ENDING_LF)).toBe("\u{20001}");
        });

        it("selects all CJK supplementary characters", () => {
            const span: SelectionSpan = {startLineIndex: 0, endLineIndex: 0, startOffset: 0, endOffset: 3};
            expect(extractSelectedText(lines, span, LINE_ENDING_LF)).toBe("\u{20000}\u{20001}\u{20002}");
        });

        it("reverse drag across CJK supplementary characters", () => {
            const span: SelectionSpan = {startLineIndex: 0, endLineIndex: 0, startOffset: 3, endOffset: 1};
            expect(extractSelectedText(lines, span, LINE_ENDING_LF)).toBe("\u{20001}\u{20002}");
        });
    });

    // ── Mixed surrogate pairs with ASCII across lines ──

    describe("mixed surrogate pairs across multiple lines", () => {
        const lines = ["abc\u{1F600}", "\u{20000}def", "\u{1F601}xyz\u{1F602}"];

        it("selects across line boundary from emoji to CJK supplementary", () => {
            const span: SelectionSpan = {startLineIndex: 0, endLineIndex: 1, startOffset: 3, endOffset: 2};
            expect(extractSelectedText(lines, span, LINE_ENDING_LF)).toBe("\u{1F600}\n\u{20000}d");
        });

        it("selects all three mixed lines", () => {
            const span: SelectionSpan = {startLineIndex: 0, endLineIndex: 2, startOffset: 0, endOffset: 5};
            expect(extractSelectedText(lines, span, LINE_ENDING_LF))
                .toBe("abc\u{1F600}\n\u{20000}def\n\u{1F601}xyz\u{1F602}");
        });

        it("reverse drag across mixed lines", () => {
            const span: SelectionSpan = {startLineIndex: 2, endLineIndex: 1, startOffset: 1, endOffset: 1};
            // Normalized: startLine=1, endLine=2, startOffset=1, endOffset=1
            expect(extractSelectedText(lines, span, LINE_ENDING_LF)).toBe("def\n\u{1F601}");
        });
    });

    // ── Selection at exact boundaries (start of line, end of line) ──

    describe("selection at exact line boundaries", () => {
        const lines = ["first", "second", "third"];

        it("selects from start of first line to start of second line (zero-width end)", () => {
            const span: SelectionSpan = {startLineIndex: 0, endLineIndex: 1, startOffset: 0, endOffset: 0};
            expect(extractSelectedText(lines, span, LINE_ENDING_LF)).toBe("first\n");
        });

        it("selects from end of first line to end of second line", () => {
            const span: SelectionSpan = {startLineIndex: 0, endLineIndex: 1, startOffset: 5, endOffset: 6};
            expect(extractSelectedText(lines, span, LINE_ENDING_LF)).toBe("\nsecond");
        });

        it("selects from end of one line to start of next (just the newline)", () => {
            const span: SelectionSpan = {startLineIndex: 0, endLineIndex: 1, startOffset: 5, endOffset: 0};
            expect(extractSelectedText(lines, span, LINE_ENDING_LF)).toBe("\n");
        });
    });

    // ── Single-element lines array ──

    describe("single-element lines array", () => {
        const lines = ["only"];

        it("selects full content", () => {
            const span: SelectionSpan = {startLineIndex: 0, endLineIndex: 0, startOffset: 0, endOffset: 4};
            expect(extractSelectedText(lines, span, LINE_ENDING_LF)).toBe("only");
        });

        it("multi-line span clamped to single line", () => {
            const span: SelectionSpan = {startLineIndex: 0, endLineIndex: 5, startOffset: 0, endOffset: 100};
            // Both clamped to line 0
            expect(extractSelectedText(lines, span, LINE_ENDING_LF)).toBe("only");
        });
    });
});
