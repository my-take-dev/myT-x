import {describe, expect, it} from "vitest";
import {codePointLength, sliceByCodePoints} from "../src/utils/codePointUtils";

describe("codePointLength", () => {
    const cases: { name: string; input: string; expected: number }[] = [
        {name: "empty string", input: "", expected: 0},
        {name: "ASCII only", input: "hello", expected: 5},
        {name: "single emoji (surrogate pair)", input: "\u{1F600}", expected: 1},
        {name: "multiple emoji", input: "\u{1F600}\u{1F601}\u{1F602}", expected: 3},
        {name: "mixed ASCII and emoji", input: "hi\u{1F600}!", expected: 4},
        {name: "single ASCII character", input: "a", expected: 1},
        {name: "string with spaces", input: "a b c", expected: 5},
        {name: "CJK characters", input: "\u4F60\u597D", expected: 2},
        {name: "flag emoji (two code points)", input: "\u{1F1FA}\u{1F1F8}", expected: 2},
    ];

    it.each(cases)("$name: codePointLength($input) === $expected", ({input, expected}) => {
        expect(codePointLength(input)).toBe(expected);
    });
});

describe("sliceByCodePoints", () => {
    const cases: { name: string; input: string; start: number; end?: number; expected: string }[] = [
        // ASCII strings
        {name: "full ASCII slice from start", input: "hello", start: 0, end: 5, expected: "hello"},
        {name: "ASCII substring", input: "hello", start: 1, end: 4, expected: "ell"},
        {name: "ASCII from start=0", input: "hello", start: 0, end: 2, expected: "he"},
        {name: "ASCII to end (no end param)", input: "hello", start: 3, expected: "lo"},
        {name: "ASCII start equals end", input: "hello", start: 2, end: 2, expected: ""},

        // Emoji (surrogate pairs)
        {name: "single emoji slice", input: "\u{1F600}\u{1F601}\u{1F602}", start: 0, end: 1, expected: "\u{1F600}"},
        {name: "middle emoji", input: "\u{1F600}\u{1F601}\u{1F602}", start: 1, end: 2, expected: "\u{1F601}"},
        {
            name: "all emoji",
            input: "\u{1F600}\u{1F601}\u{1F602}",
            start: 0,
            end: 3,
            expected: "\u{1F600}\u{1F601}\u{1F602}"
        },
        {name: "emoji to end", input: "\u{1F600}\u{1F601}", start: 1, expected: "\u{1F601}"},

        // Mixed ASCII + emoji
        {name: "mixed: ASCII portion", input: "hi\u{1F600}!", start: 0, end: 2, expected: "hi"},
        {name: "mixed: emoji portion", input: "hi\u{1F600}!", start: 2, end: 3, expected: "\u{1F600}"},
        {name: "mixed: after emoji", input: "hi\u{1F600}!", start: 3, end: 4, expected: "!"},
        {name: "mixed: span across emoji", input: "hi\u{1F600}!", start: 1, end: 4, expected: "i\u{1F600}!"},

        // Empty string
        {name: "empty string with start=0", input: "", start: 0, expected: ""},
        {name: "empty string with start=0 end=0", input: "", start: 0, end: 0, expected: ""},

        // Edge cases
        {name: "start=0 end=length", input: "abc", start: 0, end: 3, expected: "abc"},
        {name: "end beyond length", input: "abc", start: 1, end: 100, expected: "bc"},
        {name: "start at last code point", input: "abc", start: 2, end: 3, expected: "c"},
        {name: "start beyond length", input: "abc", start: 5, expected: ""},
        {name: "start beyond length with end", input: "abc", start: 5, end: 10, expected: ""},
        {name: "end less than start returns empty", input: "hello", start: 3, end: 1, expected: ""},
        {name: "negative start is clamped to 0", input: "hello", start: -3, end: 2, expected: "he"},
        {name: "negative start without end returns full string", input: "hello", start: -1, expected: "hello"},

        // Early return path: start=0, end=undefined → returns value directly
        {name: "full string shortcut (start=0, no end)", input: "hello", start: 0, expected: "hello"},
        {name: "full emoji string shortcut", input: "\u{1F600}\u{1F601}", start: 0, expected: "\u{1F600}\u{1F601}"},
        {name: "full CJK string shortcut", input: "\u4F60\u597D\u4E16\u754C", start: 0, expected: "\u4F60\u597D\u4E16\u754C"},
        {name: "single char shortcut", input: "x", start: 0, expected: "x"},
        {name: "empty string shortcut (start=0, no end)", input: "", start: 0, expected: ""},
        {name: "mixed ASCII+emoji shortcut", input: "hi\u{1F600}!", start: 0, expected: "hi\u{1F600}!"},
        {name: "string with spaces shortcut", input: "a b c", start: 0, expected: "a b c"},
    ];

    it.each(cases)("$name", ({input, start, end, expected}) => {
        expect(sliceByCodePoints(input, start, end)).toBe(expected);
    });

    it("start=0 no end returns the same string reference (identity shortcut)", () => {
        const original = "hello\u{1F600}world";
        const result = sliceByCodePoints(original, 0);
        // toBe uses Object.is internally — verifies reference identity, not just value equality.
        // The early return path `if (start === 0 && end === undefined) return value;`
        // returns the original reference without allocating a new string.
        expect(result).toBe(original);
    });

    it("treats ZWJ sequences as multiple code points (code-point-based slicing)", () => {
        // Family emoji is composed from multiple code points joined by ZWJ.
        const family = "👨‍👩‍👧‍👦";
        expect(codePointLength(family)).toBeGreaterThan(1);
        expect(sliceByCodePoints(family, 0, 1).length).toBeGreaterThan(0);
    });
});
