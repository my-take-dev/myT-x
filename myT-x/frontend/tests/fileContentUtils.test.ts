import {describe, expect, it} from "vitest";
import {isSelectAllShortcut, parseLineIndex} from "../src/components/viewer/views/file-tree/fileContentUtils";

describe("fileContentUtils", () => {
    describe("isSelectAllShortcut", () => {
        const makeEvent = (overrides: Partial<{
            ctrlKey: boolean;
            metaKey: boolean;
            altKey: boolean;
            key: string;
        }> = {}) =>
            ({
                ctrlKey: false,
                metaKey: false,
                altKey: false,
                key: "a",
                ...overrides,
            }) as unknown as React.KeyboardEvent<HTMLDivElement>;

        it.each([
            {name: "Ctrl+A", event: {ctrlKey: true, key: "a"}},
            {name: "Cmd+A", event: {metaKey: true, key: "a"}},
            {name: "Ctrl+A (uppercase)", event: {ctrlKey: true, key: "A"}},
        ])("returns true for $name", ({event}) => {
            expect(isSelectAllShortcut(makeEvent(event))).toBe(true);
        });

        it.each([
            {name: "just A", event: {key: "a"}},
            {name: "Alt+A", event: {altKey: true, key: "a"}},
            {name: "Ctrl+Alt+A", event: {ctrlKey: true, altKey: true, key: "a"}},
            {name: "Ctrl+B", event: {ctrlKey: true, key: "b"}},
            {name: "Ctrl+C", event: {ctrlKey: true, key: "c"}},
        ])("returns false for $name", ({event}) => {
            expect(isSelectAllShortcut(makeEvent(event))).toBe(false);
        });
    });

    describe("parseLineIndex", () => {
        const makeEl = (dataLineIndex?: string) => {
            if (dataLineIndex === undefined) return null;
            return {dataset: {lineIndex: dataLineIndex}} as unknown as HTMLElement;
        };

        it.each([
            {name: "valid index 0", input: "0", expected: 0},
            {name: "valid index 42", input: "42", expected: 42},
            {name: "valid large index", input: "99999", expected: 99999},
        ])("returns $expected for $name", ({input, expected}) => {
            expect(parseLineIndex(makeEl(input))).toBe(expected);
        });

        it.each([
            {name: "null element", input: undefined, factory: () => null as HTMLElement | null},
            {name: "empty string", input: ""},
            {name: "negative number", input: "-1"},
            {name: "float", input: "1.5"},
            {name: "NaN string", input: "abc"},
        ])("returns null for $name", (testCase) => {
            const el = "factory" in testCase
                ? (testCase.factory as () => HTMLElement | null)()
                : makeEl(testCase.input as string);
            expect(parseLineIndex(el)).toBeNull();
        });
    });
});
