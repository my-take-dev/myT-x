import {describe, expect, it} from "vitest";
import {toErrorMessage} from "../src/utils/errorUtils";

const FALLBACK = "Unknown error";

describe("toErrorMessage", () => {
    it.each([
        {input: new Error("boom"), expected: "boom", desc: "Error with message"},
        {input: new Error(""), expected: FALLBACK, desc: "Error with empty message"},
        {input: new Error("   "), expected: FALLBACK, desc: "Error with whitespace-only message"},
        {input: "raw error string", expected: "raw error string", desc: "plain string"},
        {input: "   ", expected: FALLBACK, desc: "whitespace-only string"},
        {input: null, expected: FALLBACK, desc: "null"},
        {input: undefined, expected: FALLBACK, desc: "undefined"},
        {input: 42, expected: FALLBACK, desc: "number"},
        {input: {}, expected: FALLBACK, desc: "plain object"},
    ])("$desc → $expected", ({input, expected}) => {
        expect(toErrorMessage(input, FALLBACK)).toBe(expected);
    });

    it("uses custom fallback string", () => {
        expect(toErrorMessage(null, "custom fallback")).toBe("custom fallback");
    });

    it("returns Error.message even when fallback differs", () => {
        expect(toErrorMessage(new Error("specific"), "ignored")).toBe("specific");
    });

    it("returns plain string even when fallback differs", () => {
        expect(toErrorMessage("plain text", "ignored")).toBe("plain text");
    });

    it("returns fallback for boolean", () => {
        expect(toErrorMessage(true, FALLBACK)).toBe(FALLBACK);
    });

    it("returns fallback for symbol", () => {
        expect(toErrorMessage(Symbol("err"), FALLBACK)).toBe(FALLBACK);
    });

    it("returns fallback for array", () => {
        expect(toErrorMessage(["a", "b"], FALLBACK)).toBe(FALLBACK);
    });
});
