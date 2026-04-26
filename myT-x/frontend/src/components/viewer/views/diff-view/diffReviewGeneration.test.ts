import {describe, expect, it} from "vitest";
import {devpanel} from "../../../../../wailsjs/go/models";
import {buildDiffReviewGenerationKey, shouldResetDiffReviewState} from "./diffReviewGeneration";

describe("diffReviewGeneration", () => {
    it("changes the generation key when diff content changes", () => {
        const before = buildDiffReviewGenerationKey(new devpanel.WorkingDiffResult({
            files: [{path: "a.ts", old_path: "", status: "modified", additions: 1, deletions: 0, diff: "@@ -1 +1 @@\n-old\n+new"}],
            total_added: 1,
            total_deleted: 0,
            truncated: false,
        }));
        const after = buildDiffReviewGenerationKey(new devpanel.WorkingDiffResult({
            files: [{path: "a.ts", old_path: "", status: "modified", additions: 1, deletions: 0, diff: "@@ -1 +1 @@\n-old\n+newer"}],
            total_added: 1,
            total_deleted: 0,
            truncated: false,
        }));

        expect(before).not.toBe(after);
    });

    it("keeps the same generation key when file order changes without content changes", () => {
        const first = buildDiffReviewGenerationKey(new devpanel.WorkingDiffResult({
            files: [
                {path: "b.ts", old_path: "", status: "modified", additions: 1, deletions: 0, diff: "@@ -1 +1 @@\n-old\n+new"},
                {path: "a.ts", old_path: "", status: "modified", additions: 2, deletions: 1, diff: "@@ -2 +2 @@\n-older\n+newer"},
            ],
            total_added: 3,
            total_deleted: 1,
            truncated: false,
        }));
        const second = buildDiffReviewGenerationKey(new devpanel.WorkingDiffResult({
            files: [
                {path: "a.ts", old_path: "", status: "modified", additions: 2, deletions: 1, diff: "@@ -2 +2 @@\n-older\n+newer"},
                {path: "b.ts", old_path: "", status: "modified", additions: 1, deletions: 0, diff: "@@ -1 +1 @@\n-old\n+new"},
            ],
            total_added: 3,
            total_deleted: 1,
            truncated: false,
        }));

        expect(first).toBe(second);
    });

    it("requests reset only when the same session receives a new diff generation", () => {
        expect(shouldResetDiffReviewState("session:1", "gen-1", "session:1", "gen-2")).toBe(true);
        expect(shouldResetDiffReviewState("session:1", "gen-1", "session:1", "gen-1")).toBe(false);
        expect(shouldResetDiffReviewState("session:1", "gen-1", "session:2", "gen-2")).toBe(false);
        expect(shouldResetDiffReviewState("session:1", "", "session:1", "gen-2")).toBe(false);
        expect(shouldResetDiffReviewState("session:1", "gen-1", "session:1", "")).toBe(false);
    });
});
