import {describe, expect, it} from "vitest";
import {normalizeGitGraphCommit, normalizeGitGraphCommits} from "../src/components/viewer/views/git-graph/gitGraphTypes";

/** Helper to create a minimal raw commit object matching the backend payload shape. */
function makeRaw(overrides: Record<string, unknown> = {}) {
    return {
        hash: "abc1234",
        full_hash: "abc1234567890abcdef1234567890abcdef123456",
        parents: null,
        subject: "test commit",
        author_name: "Test Author",
        author_date: "2026-01-01T00:00:00Z",
        refs: null,
        ...overrides,
    };
}

describe("normalizeGitGraphCommit", () => {
    it("preserves parents: null as null", () => {
        const raw = makeRaw({parents: null});
        const result = normalizeGitGraphCommit(raw);
        expect(result.parents).toBeNull();
    });

    it("converts parents string array to FullCommitHash[]", () => {
        const parentHash = "def4567890abcdef1234567890abcdef12345678";
        const raw = makeRaw({parents: [parentHash]});
        const result = normalizeGitGraphCommit(raw);
        expect(result.parents).toEqual([parentHash]);
        expect(result.parents).not.toBeNull();
        expect(result.parents!).toHaveLength(1);
    });

    it("brands hash as ShortCommitHash", () => {
        const raw = makeRaw({hash: "abcd123"});
        const result = normalizeGitGraphCommit(raw);
        // The branded value is still the same string at runtime
        expect(result.hash).toBe("abcd123");
    });

    it("brands full_hash as FullCommitHash", () => {
        const fullHash = "abcdef1234567890abcdef1234567890abcdef12";
        const raw = makeRaw({full_hash: fullHash});
        const result = normalizeGitGraphCommit(raw);
        expect(result.full_hash).toBe(fullHash);
    });

    it("preserves all non-hash fields unchanged", () => {
        const raw = makeRaw({
            subject: "feat: add feature",
            author_name: "Alice",
            author_date: "2026-03-01T12:00:00Z",
            refs: ["HEAD", "main"],
        });
        const result = normalizeGitGraphCommit(raw);
        expect(result.subject).toBe("feat: add feature");
        expect(result.author_name).toBe("Alice");
        expect(result.author_date).toBe("2026-03-01T12:00:00Z");
        expect(result.refs).toEqual(["HEAD", "main"]);
    });

    it("handles multiple parents (merge commit)", () => {
        const p1 = "1111111111111111111111111111111111111111";
        const p2 = "2222222222222222222222222222222222222222";
        const raw = makeRaw({parents: [p1, p2]});
        const result = normalizeGitGraphCommit(raw);
        expect(result.parents).toEqual([p1, p2]);
        expect(result.parents!).toHaveLength(2);
    });

    it("handles empty parents array", () => {
        const raw = makeRaw({parents: []});
        const result = normalizeGitGraphCommit(raw);
        // Empty array is truthy, so parents.map runs and returns []
        expect(result.parents).toEqual([]);
        expect(result.parents!).toHaveLength(0);
    });

    it("preserves refs: null", () => {
        const raw = makeRaw({refs: null});
        const result = normalizeGitGraphCommit(raw);
        expect(result.refs).toBeNull();
    });
});

describe("normalizeGitGraphCommits", () => {
    it("returns empty array for empty input", () => {
        expect(normalizeGitGraphCommits([])).toEqual([]);
    });

    it("normalizes a single commit", () => {
        const raw = [makeRaw()];
        const result = normalizeGitGraphCommits(raw);
        expect(result).toHaveLength(1);
        expect(result[0].hash).toBe("abc1234");
    });

    it("normalizes multiple commits with mixed parents", () => {
        const p1 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa";
        const p2 = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb";
        const raws: ReturnType<typeof makeRaw>[] = [
            makeRaw({hash: "aaa", full_hash: p1, parents: null, subject: "initial"}),
            makeRaw({hash: "bbb", full_hash: p2, parents: [p1], subject: "second"}),
            makeRaw({
                hash: "ccc",
                full_hash: "cccccccccccccccccccccccccccccccccccccccc",
                parents: [p1, p2],
                subject: "merge",
            }),
        ];

        const result = normalizeGitGraphCommits(raws);
        expect(result).toHaveLength(3);

        // First commit: no parents
        expect(result[0].hash).toBe("aaa");
        expect(result[0].parents).toBeNull();

        // Second commit: one parent
        expect(result[1].hash).toBe("bbb");
        expect(result[1].parents).toEqual([p1]);

        // Third commit: merge with two parents
        expect(result[2].hash).toBe("ccc");
        expect(result[2].parents).toEqual([p1, p2]);
    });

    it("preserves order of input commits", () => {
        const raws: ReturnType<typeof makeRaw>[] = [
            makeRaw({hash: "first", subject: "A"}),
            makeRaw({hash: "second", subject: "B"}),
            makeRaw({hash: "third", subject: "C"}),
        ];

        const result = normalizeGitGraphCommits(raws);
        expect(result.map(c => c.hash)).toEqual(["first", "second", "third"]);
        expect(result.map(c => c.subject)).toEqual(["A", "B", "C"]);
    });
});
