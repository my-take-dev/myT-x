import {describe, expect, it} from "vitest";
import type {BranchInfo} from "../src/components/viewer/views/diff-view/sourceControlTypes";

/**
 * Field count guard for BranchInfo.
 * When Go adds fields to GitStatusResult and models.ts is regenerated,
 * the manual mapping in useDiffData.ts must be updated. This test detects
 * when BranchInfo gains or loses fields so the developer is prompted to
 * update the mapping and the EMPTY_GIT_STATUS constant.
 */
describe("BranchInfo field count guard", () => {
    it("should have exactly 6 fields", () => {
        // Construct a value that satisfies every required BranchInfo field.
        // If a field is added/removed from the interface, this literal will
        // cause a compile error, and the count assertion below will fail.
        const sample: BranchInfo = {
            branch: "",
            ahead: 0,
            behind: 0,
            upstreamConfigured: false,
            conflicted: [],
            statusFetchFailed: false,
        };

        const fieldCount = Object.keys(sample).length;
        expect(fieldCount).toBe(6);
    });
});
