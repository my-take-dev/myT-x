import {describe, expect, it} from "vitest";
import {asArray, asObject} from "../src/utils/typeGuards";

describe("typeGuards", () => {
    describe("asObject", () => {
        it("returns object for plain object payload", () => {
            const payload = {a: 1};
            expect(asObject<typeof payload>(payload)).toBe(payload);
        });

        it("returns null for null, primitives, and arrays", () => {
            expect(asObject(null)).toBeNull();
            expect(asObject(undefined)).toBeNull();
            expect(asObject(1)).toBeNull();
            expect(asObject("x")).toBeNull();
            expect(asObject([1, 2, 3])).toBeNull();
        });
    });

    describe("asArray", () => {
        it("returns array payload", () => {
            const payload = [1, 2, 3];
            expect(asArray<number>(payload)).toBe(payload);
        });

        it("returns null for non-array payloads", () => {
            expect(asArray(null)).toBeNull();
            expect(asArray(undefined)).toBeNull();
            expect(asArray({})).toBeNull();
            expect(asArray("x")).toBeNull();
        });
    });
});
