/**
 * C-4 regression test: Verify that parseSingleFileDiff's catch block always
 * produces a non-empty `message` field, regardless of what type of value is thrown.
 *
 * Covers: `throw "string"`, `throw null`, `throw undefined`, `throw new Error("")`,
 * and `throw new Error("msg")`.
 */
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";

// Mock splitLines to inject controlled throw types into parseSingleFileDiff's try block.
const splitLinesMock = vi.hoisted(() => vi.fn<(text: string) => string[]>());

vi.mock("../src/utils/textLines", () => ({
    splitLines: splitLinesMock,
}));

import {parseSingleFileDiff} from "../src/utils/diffParser";

describe("parseSingleFileDiff – non-Error throw handling (C-4)", () => {
    beforeEach(() => {
        splitLinesMock.mockReset();
        vi.spyOn(console, "error").mockImplementation(() => {});
        vi.spyOn(console, "warn").mockImplementation(() => {});
    });

    afterEach(() => {
        vi.restoreAllMocks();
    });

    it("returns non-empty fallback message when a string is thrown", () => {
        splitLinesMock.mockImplementation(() => {
            throw "string error";
        });

        const result = parseSingleFileDiff("any input");
        expect(result).toMatchObject({ status: "error", message: "Failed to parse diff." });
    });

    it("returns non-empty fallback message when null is thrown", () => {
        splitLinesMock.mockImplementation(() => {
            throw null;
        });

        const result = parseSingleFileDiff("any input");
        expect(result).toMatchObject({ status: "error", message: "Failed to parse diff." });
    });

    it("returns non-empty fallback message when undefined is thrown", () => {
        splitLinesMock.mockImplementation(() => {
            throw undefined;
        });

        const result = parseSingleFileDiff("any input");
        expect(result).toMatchObject({ status: "error", message: "Failed to parse diff." });
    });

    it("returns fallback message when Error with empty message is thrown", () => {
        splitLinesMock.mockImplementation(() => {
            throw new Error("");
        });

        const result = parseSingleFileDiff("any input");
        expect(result).toMatchObject({ status: "error", message: "Failed to parse diff." });
    });

    it("returns generic fallback message even when Error has a specific message", () => {
        splitLinesMock.mockImplementation(() => {
            throw new Error("specific parse failure");
        });

        // Technical error details are logged via console.error but not exposed to the UI.
        const result = parseSingleFileDiff("any input");
        expect(result).toMatchObject({ status: "error", message: "Failed to parse diff." });
    });

    it("returns non-empty fallback message when a number is thrown", () => {
        splitLinesMock.mockImplementation(() => {
            throw 42;
        });

        const result = parseSingleFileDiff("any input");
        expect(result).toMatchObject({ status: "error", message: "Failed to parse diff." });
    });
});
