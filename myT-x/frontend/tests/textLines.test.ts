import {describe, expect, it} from "vitest";
import {detectLineEnding, LINE_ENDING_CRLF, LINE_ENDING_LF, splitLines} from "../src/utils/textLines";

describe("splitLines", () => {
    it("preserves trailing empty lines", () => {
        expect(splitLines("")).toEqual([""]);
        expect(splitLines("a\nb\n")).toEqual(["a", "b", ""]);
    });

    it("handles CRLF and LF consistently", () => {
        expect(splitLines("a\r\nb\r\nc")).toEqual(["a", "b", "c"]);
        expect(splitLines("a\nb\nc")).toEqual(["a", "b", "c"]);
    });

    it("keeps empty middle rows", () => {
        expect(splitLines("a\n\nb")).toEqual(["a", "", "b"]);
        expect(splitLines("a\r\n\r\nb")).toEqual(["a", "", "b"]);
    });

    // ── LF line endings ──

    describe("LF line endings", () => {
        it("splits simple LF text", () => {
            expect(splitLines("a\nb\nc")).toEqual(["a", "b", "c"]);
        });

        it("handles trailing LF", () => {
            expect(splitLines("a\n")).toEqual(["a", ""]);
        });

        it("handles leading LF", () => {
            expect(splitLines("\na")).toEqual(["", "a"]);
        });
    });

    // ── CRLF line endings ──

    describe("CRLF line endings", () => {
        it("splits simple CRLF text", () => {
            expect(splitLines("a\r\nb\r\nc")).toEqual(["a", "b", "c"]);
        });

        it("handles trailing CRLF", () => {
            expect(splitLines("a\r\n")).toEqual(["a", ""]);
        });

        it("handles leading CRLF", () => {
            expect(splitLines("\r\na")).toEqual(["", "a"]);
        });
    });

    // ── CR-only line endings ──
    // NOTE: splitLines uses /\r?\n/ regex, so CR-only does NOT split.
    // This is intentional — CR-only is a legacy format.

    describe("CR-only line endings (not split)", () => {
        it("treats CR-only text as a single line", () => {
            expect(splitLines("a\rb\rc")).toEqual(["a\rb\rc"]);
        });

        it("preserves CR characters within single line", () => {
            const result = splitLines("line1\rline2");
            expect(result).toHaveLength(1);
            expect(result[0]).toBe("line1\rline2");
        });
    });

    // ── Mixed line endings ──

    describe("mixed line endings", () => {
        it("splits mixed CRLF and LF", () => {
            expect(splitLines("a\r\nb\nc\r\nd")).toEqual(["a", "b", "c", "d"]);
        });

        it("splits LF then CRLF", () => {
            expect(splitLines("a\nb\r\nc")).toEqual(["a", "b", "c"]);
        });
    });

    // ── Empty string ──

    describe("empty string", () => {
        it("returns array with one empty string", () => {
            expect(splitLines("")).toEqual([""]);
        });
    });

    // ── Single line (no ending) ──

    describe("single line without line ending", () => {
        it("returns single-element array", () => {
            expect(splitLines("hello")).toEqual(["hello"]);
        });

        it("handles single character", () => {
            expect(splitLines("x")).toEqual(["x"]);
        });

        it("handles whitespace-only content", () => {
            expect(splitLines("   ")).toEqual(["   "]);
        });
    });

    // ── Trailing newline preservation ──

    describe("trailing newline preservation", () => {
        it("'a\\n' produces ['a', '']", () => {
            expect(splitLines("a\n")).toEqual(["a", ""]);
        });

        it("'a\\r\\n' produces ['a', '']", () => {
            expect(splitLines("a\r\n")).toEqual(["a", ""]);
        });

        it("'a\\nb\\n' produces ['a', 'b', '']", () => {
            expect(splitLines("a\nb\n")).toEqual(["a", "b", ""]);
        });
    });

    // ── Multiple consecutive newlines ──

    describe("multiple consecutive newlines", () => {
        it("handles two consecutive LF", () => {
            expect(splitLines("a\n\nb")).toEqual(["a", "", "b"]);
        });

        it("handles three consecutive LF", () => {
            expect(splitLines("a\n\n\nb")).toEqual(["a", "", "", "b"]);
        });

        it("handles two consecutive CRLF", () => {
            expect(splitLines("a\r\n\r\nb")).toEqual(["a", "", "b"]);
        });

        it("handles three consecutive CRLF", () => {
            expect(splitLines("a\r\n\r\n\r\nb")).toEqual(["a", "", "", "b"]);
        });

        it("handles only newlines", () => {
            expect(splitLines("\n\n\n")).toEqual(["", "", "", ""]);
        });

        it("handles only CRLF newlines", () => {
            expect(splitLines("\r\n\r\n\r\n")).toEqual(["", "", "", ""]);
        });
    });
});

describe("detectLineEnding", () => {
    it("detects CRLF when first newline is preceded by carriage return", () => {
        expect(detectLineEnding("a\r\nb\r\nc")).toBe(LINE_ENDING_CRLF);
    });

    it("detects LF in regular unix text", () => {
        expect(detectLineEnding("a\nb\nc")).toBe(LINE_ENDING_LF);
    });

    it("falls back to LF when line ending is not detectable", () => {
        expect(detectLineEnding("single-line")).toBe(LINE_ENDING_LF);
        expect(detectLineEnding("\nstarts-with-lf")).toBe(LINE_ENDING_LF);
    });

    it("falls back to LF for CR-only files (legacy Mac format)", () => {
        // CR-only line endings are intentionally unsupported.
        expect(detectLineEnding("line1\rline2\rline3")).toBe(LINE_ENDING_LF);
    });

    it("detects line ending from the first newline in mixed-ending files", () => {
        // CRLF first, then LF — detects CRLF from the first occurrence.
        expect(detectLineEnding("a\r\nb\nc")).toBe(LINE_ENDING_CRLF);
        // LF first, then CRLF — detects LF from the first occurrence.
        expect(detectLineEnding("a\nb\r\nc")).toBe(LINE_ENDING_LF);
    });

    it("returns LF when file starts with a newline character", () => {
        expect(detectLineEnding("\nabc")).toBe(LINE_ENDING_LF);
        expect(detectLineEnding("\n")).toBe(LINE_ENDING_LF);
    });

    // ── Pure LF content ──

    describe("pure LF content", () => {
        it("detects LF from multi-line text", () => {
            expect(detectLineEnding("line1\nline2\nline3")).toBe(LINE_ENDING_LF);
        });

        it("detects LF from single newline", () => {
            expect(detectLineEnding("a\n")).toBe(LINE_ENDING_LF);
        });
    });

    // ── Pure CRLF content ──

    describe("pure CRLF content", () => {
        it("detects CRLF from multi-line text", () => {
            expect(detectLineEnding("line1\r\nline2\r\nline3")).toBe(LINE_ENDING_CRLF);
        });

        it("detects CRLF from single newline pair", () => {
            expect(detectLineEnding("a\r\n")).toBe(LINE_ENDING_CRLF);
        });
    });

    // ── Empty string ──

    describe("empty string", () => {
        it("returns LF for empty string", () => {
            expect(detectLineEnding("")).toBe(LINE_ENDING_LF);
        });
    });

    // ── No line endings ──

    describe("no line endings", () => {
        it("returns LF for text without any newline", () => {
            expect(detectLineEnding("no newlines here")).toBe(LINE_ENDING_LF);
        });

        it("returns LF for single character", () => {
            expect(detectLineEnding("x")).toBe(LINE_ENDING_LF);
        });

        it("returns LF for text with only CR (no LF)", () => {
            // CR alone does not count as a line ending for detection purposes
            expect(detectLineEnding("a\rb\rc")).toBe(LINE_ENDING_LF);
        });
    });

    // ── Edge: CRLF at position 0 ──

    describe("CRLF at start of string", () => {
        it("detects CRLF when string starts with \\r\\n", () => {
            expect(detectLineEnding("\r\nabc")).toBe(LINE_ENDING_CRLF);
        });
    });
});
