import {describe, expect, it, vi} from "vitest";
import {computeLanes} from "../src/components/viewer/views/git-graph/laneComputation";
import type {FullCommitHash, GitGraphCommit, LaneAssignment, ParentConnection} from "../src/components/viewer/views/git-graph/gitGraphTypes";

// ---------------------------------------------------------------------------
// Test helper: create a minimal GitGraphCommit with sensible defaults.
// ---------------------------------------------------------------------------

function makeCommit(
    fullHash: string,
    parents: string[] | null,
    overrides: Partial<Omit<GitGraphCommit, "full_hash" | "parents">> = {},
): GitGraphCommit {
    return {
        hash: fullHash.slice(0, 7) as GitGraphCommit["hash"],
        full_hash: fullHash as FullCommitHash,
        parents: parents ? parents.map(p => p as FullCommitHash) : null,
        subject: overrides.subject ?? `commit ${fullHash.slice(0, 7)}`,
        author_name: overrides.author_name ?? "test",
        author_date: overrides.author_date ?? "2026-01-01T00:00:00Z",
        refs: overrides.refs ?? null,
    };
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("computeLanes", () => {
    // ── 1. Empty array ──

    it("returns empty array for empty input", () => {
        expect(computeLanes([])).toEqual([]);
    });

    // ── 2. Single root commit ──

    describe("single root commit", () => {
        it.each([
            {label: "parents: null", parents: null},
            {label: "parents: []", parents: [] as string[]},
        ])("assigns lane 0 with no connections ($label)", ({parents}) => {
            const commits = [makeCommit("aaa0000000000000000000000000000000000001", parents)];
            const result = computeLanes(commits);

            expect(result).toHaveLength(1);
            expect(result[0].lane).toBe(0);
            expect(result[0].connections).toEqual([]);
            expect(result[0].commitHash).toBe("aaa0000000000000000000000000000000000001");
        });

        it("releases the lane (activeLaneCount becomes 0 after compaction)", () => {
            const commits = [makeCommit("aaa0000000000000000000000000000000000001", null)];
            const result = computeLanes(commits);

            expect(result[0].activeLaneCount).toBe(0);
        });
    });

    // ── 3. Linear chain ──

    describe("linear chain (A -> B, newest first)", () => {
        const hashA = "aaa0000000000000000000000000000000000001";
        const hashB = "bbb0000000000000000000000000000000000002";
        const commits = [
            makeCommit(hashA, [hashB]),     // newest: A has parent B
            makeCommit(hashB, null),         // oldest: B is root
        ];

        it("assigns both commits to lane 0", () => {
            const result = computeLanes(commits);
            expect(result).toHaveLength(2);
            expect(result[0].lane).toBe(0);
            expect(result[1].lane).toBe(0);
        });

        it("creates a straight connection for the first commit", () => {
            const result = computeLanes(commits);
            expect(result[0].connections).toEqual([
                {fromLane: 0, toLane: 0, type: "straight"},
            ]);
        });

        it("root commit has no connections", () => {
            const result = computeLanes(commits);
            expect(result[1].connections).toEqual([]);
        });

        it("activeLaneCount is 1 for A and 0 for B (root compacted)", () => {
            const result = computeLanes(commits);
            expect(result[0].activeLaneCount).toBe(1);
            expect(result[1].activeLaneCount).toBe(0);
        });
    });

    // ── 4. Merge commit ──

    describe("merge commit (C has 2 parents A and B)", () => {
        //  C (merge) -> parents [A, B]
        //  A (root)
        //  B (root)
        // Newest first: C, A, B
        const hashA = "aaa0000000000000000000000000000000000001";
        const hashB = "bbb0000000000000000000000000000000000002";
        const hashC = "ccc0000000000000000000000000000000000003";

        const commits = [
            makeCommit(hashC, [hashA, hashB]),  // merge commit
            makeCommit(hashA, null),             // first parent (root)
            makeCommit(hashB, null),             // second parent (root)
        ];

        it("assigns merge commit to lane 0", () => {
            const result = computeLanes(commits);
            expect(result[0].lane).toBe(0);
        });

        it("creates straight + merge connection for the merge commit", () => {
            const result = computeLanes(commits);
            const connections = result[0].connections;

            expect(connections).toHaveLength(2);
            // First parent gets straight connection on same lane
            expect(connections[0]).toEqual({fromLane: 0, toLane: 0, type: "straight"});
            // Second parent gets allocated to a new lane (lane 1) -> merge-right
            expect(connections[1]).toEqual({fromLane: 0, toLane: 1, type: "merge-right"});
        });

        it("first parent inherits lane 0, second parent gets lane 1", () => {
            const result = computeLanes(commits);
            expect(result[1].lane).toBe(0); // A
            expect(result[2].lane).toBe(1); // B
        });
    });

    // ── 5. Branch divergence ──

    describe("branch divergence (two children of same parent)", () => {
        // History (newest first):
        //   D (on branch, parent=B)  -- first seen, gets lane 0
        //   C (on main, parent=B)    -- not yet tracked, needs new lane
        //   B (root)                 -- tracked from D on lane 0
        const hashB = "bbb0000000000000000000000000000000000002";
        const hashC = "ccc0000000000000000000000000000000000003";
        const hashD = "ddd0000000000000000000000000000000000004";

        const commits = [
            makeCommit(hashD, [hashB]),  // branch tip
            makeCommit(hashC, [hashB]),  // main tip
            makeCommit(hashB, null),     // shared root
        ];

        it("allocates diverging commits to different lanes when parent is already tracked", () => {
            const result = computeLanes(commits);
            // D takes lane 0 (new, first slot)
            expect(result[0].lane).toBe(0);
            // C is not tracked in activeLanes, gets next available slot (lane 1)
            expect(result[1].lane).toBe(1);
        });

        it("both D and C point to parent B as straight connection", () => {
            const result = computeLanes(commits);
            // D: activeLanes[0] = hashB (straight to parent)
            expect(result[0].connections).toEqual([
                {fromLane: 0, toLane: 0, type: "straight"},
            ]);
            // C: activeLanes[1] = hashB (straight to parent)
            expect(result[1].connections).toEqual([
                {fromLane: 1, toLane: 1, type: "straight"},
            ]);
        });

        it("parent B ends up on the lane that was first registered for it (lane 0)", () => {
            const result = computeLanes(commits);
            // B is tracked in activeLanes[0] from D, so it gets lane 0
            expect(result[2].lane).toBe(0);
        });
    });

    // ── 6. Root commit lane release and compaction ──

    describe("root commit lane release and trailing null compaction", () => {
        // History:
        //   E (parent=C)   lane 0
        //   D (parent=C)   lane 1 (new branch)
        //   C (root)       lane 0 (release lane) -> activeLanes compacted
        const hashC = "ccc0000000000000000000000000000000000003";
        const hashD = "ddd0000000000000000000000000000000000004";
        const hashE = "eee0000000000000000000000000000000000005";

        it("compacts trailing null lanes after root commits", () => {
            // Both E and D point to C.
            // E: first commit, gets lane 0, activeLanes = [C]
            // D: not tracked, gets lane 1 (next slot), but C already in lane 0
            //    D sets activeLanes[1] = C, but C is already in lane 0
            //    Actually D's parent C is found at activeLanes[0], so activeLanes[1] = C (duplicate)
            //    Wait - let's trace carefully:
            //
            // After E: activeLanes = [hashC]
            // D arrives: indexOf(hashD) = -1, indexOf(null) = -1 -> lane 1, activeLanes = [hashC, hashD]
            // D's parent is hashC: activeLanes[1] = hashC -> activeLanes = [hashC, hashC]
            // Compact: no trailing nulls, activeLaneCount = 2
            // C arrives: indexOf(hashC) = 0 -> lane 0
            // C is root: activeLanes[0] = null -> activeLanes = [null, hashC]
            // Compact: trailing slot 1 is hashC (not null) -> no compaction, activeLaneCount = 2
            // BUT hashC at index 1 is the duplicate from D.
            // Since C has already been processed, that duplicate never matches again.
            // Actually wait - C is root. After processing C, activeLanes = [null, hashC_dup].
            // The duplicate at index 1 will never be consumed. But compaction only removes trailing nulls.

            // Let's use a simpler scenario:
            // E (parent=C), D (root) on a separate lane
            const hashRoot = "rrr0000000000000000000000000000000000001";
            const hashChild = "ccc0000000000000000000000000000000000002";
            const hashMain = "mmm0000000000000000000000000000000000003";

            // History: mainCommit (parent=hashChild), rootCommit (no parent), childCommit (root)
            // mainCommit: lane 0, activeLanes = [hashChild]
            // rootCommit: not tracked, lane 1, activeLanes = [hashChild, hashRoot]
            //   root -> release lane 1 -> activeLanes = [hashChild, null]
            //   compact trailing nulls -> activeLanes = [hashChild]  (length reduced!)
            // childCommit: indexOf(hashChild) = 0, lane 0, root -> release
            //   activeLanes = [null] -> compact -> activeLanes = []

            const testCommits = [
                makeCommit(hashMain, [hashChild]),
                makeCommit(hashRoot, null),         // root on separate lane
                makeCommit(hashChild, null),         // root
            ];

            const result = computeLanes(testCommits);

            // mainCommit: lane 0, activeLaneCount = 1
            expect(result[0].lane).toBe(0);
            expect(result[0].activeLaneCount).toBe(1);

            // rootCommit: lane 1, but after release + compaction -> activeLaneCount = 1
            expect(result[1].lane).toBe(1);
            expect(result[1].activeLaneCount).toBe(1);

            // childCommit: lane 0, root -> release -> activeLaneCount = 0
            expect(result[2].lane).toBe(0);
            expect(result[2].activeLaneCount).toBe(0);
        });

        it("reuses freed lane slot for next untracked commit", () => {
            // After a root commit frees a lane (non-trailing), the slot becomes null
            // and can be reused by the next untracked commit.
            const h1 = "aaa0000000000000000000000000000000000001";
            const h2 = "bbb0000000000000000000000000000000000002";
            const h3 = "ccc0000000000000000000000000000000000003";
            const h4 = "ddd0000000000000000000000000000000000004";

            // h1 (parent=h3) -> lane 0, activeLanes = [h3]
            // h2 (parent=h4) -> lane 1, activeLanes = [h3, h4]
            // h3 (root)      -> lane 0, release -> activeLanes = [null, h4], compact -> [null, h4]
            //   (non-trailing null not compacted)
            // h4 arrives     -> indexOf(h4) = 1 -> lane 1
            //   h4 is root   -> release lane 1 -> activeLanes = [null, null] -> compact -> []

            const testCommits = [
                makeCommit(h1, [h3]),
                makeCommit(h2, [h4]),
                makeCommit(h3, null),
                makeCommit(h4, null),
            ];

            const result = computeLanes(testCommits);
            expect(result[0].lane).toBe(0);
            expect(result[1].lane).toBe(1);
            expect(result[2].lane).toBe(0);
            // After h3 root release: activeLanes = [null, h4], no trailing null to compact
            expect(result[2].activeLaneCount).toBe(2);
            expect(result[3].lane).toBe(1);
            // After h4 root release: activeLanes = [null, null] -> compact to []
            expect(result[3].activeLaneCount).toBe(0);
        });

        it("allocates new untracked commit into freed (null) slot instead of appending", () => {
            const h1 = "aaa0000000000000000000000000000000000001";
            const h2 = "bbb0000000000000000000000000000000000002";
            const h3 = "ccc0000000000000000000000000000000000003";
            const h4 = "ddd0000000000000000000000000000000000004";
            const h5 = "eee0000000000000000000000000000000000005";

            // h1 (parent=h3) -> lane 0, activeLanes = [h3]
            // h2 (parent=h4) -> lane 1, activeLanes = [h3, h4]
            // h3 (root)      -> lane 0, release -> activeLanes = [null, h4]
            // h5 (parent=h4) -> not tracked, indexOf(null)=0, so reuses lane 0!
            //                   activeLanes = [h4, h4]
            //                   h5's parent h4 already in activeLanes -> stays on same lane 0 as straight
            //                   Actually: first parent h4, activeLanes[0] = h4 (already h4, overwrite same)
            //                   connections: [{fromLane:0, toLane:0, type:"straight"}]

            const testCommits = [
                makeCommit(h1, [h3]),
                makeCommit(h2, [h4]),
                makeCommit(h3, null),
                makeCommit(h5, [h4]),   // should reuse freed lane 0
                makeCommit(h4, null),
            ];

            const result = computeLanes(testCommits);
            // h5 should get lane 0 (reused from freed slot)
            expect(result[3].lane).toBe(0);
        });
    });

    // ── 7. Multiple branches + merges mixed scenario ──

    describe("complex scenario: multiple branches with merge", () => {
        // Realistic git history (newest first):
        //
        //   M (merge: parents=[D, E])    -- merge commit
        //   D (parent=B)                 -- on main
        //   E (parent=C)                 -- on feature branch
        //   B (parent=A)                 -- on main
        //   C (root)                     -- feature root
        //   A (root)                     -- main root
        //
        const hashA = "aaa0000000000000000000000000000000000001";
        const hashB = "bbb0000000000000000000000000000000000002";
        const hashC = "ccc0000000000000000000000000000000000003";
        const hashD = "ddd0000000000000000000000000000000000004";
        const hashE = "eee0000000000000000000000000000000000005";
        const hashM = "mmm0000000000000000000000000000000000006";

        const commits = [
            makeCommit(hashM, [hashD, hashE]),  // merge
            makeCommit(hashD, [hashB]),          // main
            makeCommit(hashE, [hashC]),          // feature
            makeCommit(hashB, [hashA]),          // main
            makeCommit(hashC, null),             // feature root
            makeCommit(hashA, null),             // main root
        ];

        let result: LaneAssignment[];
        // Compute once, assert many
        it("computes correct number of assignments", () => {
            result = computeLanes(commits);
            expect(result).toHaveLength(6);
        });

        it("merge commit is on lane 0 with straight + merge connections", () => {
            result = computeLanes(commits);
            expect(result[0].lane).toBe(0);
            expect(result[0].connections).toHaveLength(2);
            expect(result[0].connections[0].type).toBe("straight");
            expect(result[0].connections[1].type).toBe("merge-right");
        });

        it("all commitHash fields match input hashes", () => {
            result = computeLanes(commits);
            expect(result.map(r => r.commitHash)).toEqual([
                hashM, hashD, hashE, hashB, hashC, hashA,
            ]);
        });

        it("activeLaneCount is non-negative for every row", () => {
            result = computeLanes(commits);
            for (const assignment of result) {
                expect(assignment.activeLaneCount).toBeGreaterThanOrEqual(0);
            }
        });

        it("connections only reference lanes within activeLaneCount range at the time", () => {
            result = computeLanes(commits);
            // For each row, the max lane referenced by connections should be < activeLaneCount
            // (or equal to, since activeLaneCount is recorded after allocation).
            for (const assignment of result) {
                for (const conn of assignment.connections) {
                    expect(conn.fromLane).toBeLessThan(assignment.activeLaneCount);
                    expect(conn.toLane).toBeLessThan(assignment.activeLaneCount);
                }
            }
        });

        it("final row (last root) has activeLaneCount 0", () => {
            result = computeLanes(commits);
            expect(result[result.length - 1].activeLaneCount).toBe(0);
        });
    });

    // ── 8. Merge-left connection type ──

    describe("merge-left connection type", () => {
        it("produces merge-left when parent lane is lower than commit lane", () => {
            // Setup: two active lanes, then a merge from lane 1 to parent on lane 0.
            //
            // X (parent=Z)        -> lane 0, activeLanes = [Z]
            // Y (parents=[W, Z])  -> lane 1 (new), first parent W -> activeLanes[1]=W
            //                        second parent Z -> already at lane 0
            //                        connection: merge from lane 1 to lane 0 -> merge-left
            const hashW = "www0000000000000000000000000000000000001";
            const hashX = "xxx0000000000000000000000000000000000002";
            const hashY = "yyy0000000000000000000000000000000000003";
            const hashZ = "zzz0000000000000000000000000000000000004";

            const testCommits = [
                makeCommit(hashX, [hashZ]),
                makeCommit(hashY, [hashW, hashZ]),  // merge: second parent Z is on lane 0
                makeCommit(hashW, null),
                makeCommit(hashZ, null),
            ];

            const result = computeLanes(testCommits);
            // Y is on lane 1; second parent Z is on lane 0 -> merge-left
            const yConnections = result[1].connections;
            expect(yConnections).toHaveLength(2);
            expect(yConnections[0]).toEqual({fromLane: 1, toLane: 1, type: "straight"});
            expect(yConnections[1]).toEqual({fromLane: 1, toLane: 0, type: "merge-left"});
        });
    });

    // ── 9. Merge-right connection type verification ──

    describe("merge-right connection type", () => {
        it("produces merge-right when parent lane is higher than commit lane", () => {
            // M (parents=[A, B]): lane 0, first parent A inherits lane 0,
            // second parent B gets new lane 1 -> merge-right (0 -> 1)
            const hashA = "aaa0000000000000000000000000000000000001";
            const hashB = "bbb0000000000000000000000000000000000002";
            const hashM = "mmm0000000000000000000000000000000000003";

            const testCommits = [
                makeCommit(hashM, [hashA, hashB]),
                makeCommit(hashA, null),
                makeCommit(hashB, null),
            ];

            const result = computeLanes(testCommits);
            const mConnections = result[0].connections;
            expect(mConnections[1]).toEqual({fromLane: 0, toLane: 1, type: "merge-right"});
        });
    });

    // ── 10. Long linear chain (table-driven) ──

    describe("longer linear chain (5 commits)", () => {
        const hashes = [
            "aaa0000000000000000000000000000000000001",
            "bbb0000000000000000000000000000000000002",
            "ccc0000000000000000000000000000000000003",
            "ddd0000000000000000000000000000000000004",
            "eee0000000000000000000000000000000000005",
        ];
        // Each commit's parent is the next in the list (newest first), last is root.
        const commits = hashes.map((h, i) =>
            makeCommit(h, i < hashes.length - 1 ? [hashes[i + 1]] : null),
        );

        it.each(
            hashes.map((h, i) => ({index: i, hash: h.slice(0, 7)})),
        )("commit $index ($hash) is assigned to lane 0", ({index}) => {
            const result = computeLanes(commits);
            expect(result[index].lane).toBe(0);
        });

        it("all non-root commits have exactly one straight connection", () => {
            const result = computeLanes(commits);
            for (let i = 0; i < result.length - 1; i++) {
                expect(result[i].connections).toEqual([
                    {fromLane: 0, toLane: 0, type: "straight"},
                ]);
            }
        });

        it("root commit has no connections", () => {
            const result = computeLanes(commits);
            expect(result[result.length - 1].connections).toEqual([]);
        });
    });

    // ── 11. Octopus merge (3+ parents) ──

    describe("octopus merge (3 parents)", () => {
        const hashP1 = "p110000000000000000000000000000000000001";
        const hashP2 = "p220000000000000000000000000000000000002";
        const hashP3 = "p330000000000000000000000000000000000003";
        const hashO = "ooo0000000000000000000000000000000000004";

        const commits = [
            makeCommit(hashO, [hashP1, hashP2, hashP3]),
            makeCommit(hashP1, null),
            makeCommit(hashP2, null),
            makeCommit(hashP3, null),
        ];

        it("creates 3 connections for octopus merge", () => {
            const result = computeLanes(commits);
            expect(result[0].connections).toHaveLength(3);
        });

        it("first connection is straight, others are merge-right", () => {
            const result = computeLanes(commits);
            expect(result[0].connections[0].type).toBe("straight");
            expect(result[0].connections[1].type).toBe("merge-right");
            expect(result[0].connections[2].type).toBe("merge-right");
        });

        it("each parent is allocated to a distinct lane", () => {
            const result = computeLanes(commits);
            const lanes = result.slice(1).map(r => r.lane);
            const uniqueLanes = new Set(lanes);
            expect(uniqueLanes.size).toBe(3);
        });
    });

    // ── 12. readonly input is not mutated ──

    describe("immutability", () => {
        it("does not mutate the input array", () => {
            const hashA = "aaa0000000000000000000000000000000000001";
            const hashB = "bbb0000000000000000000000000000000000002";
            const commits: readonly GitGraphCommit[] = Object.freeze([
                makeCommit(hashA, [hashB]),
                makeCommit(hashB, null),
            ]);

            // Should not throw even though input is frozen
            expect(() => computeLanes(commits)).not.toThrow();
        });
    });

    // ── 13. parentLane === lane guard (duplicate parent hash) ──

    describe("parentLane === lane guard (malformed input)", () => {
        it("skips connection and logs error when merge parent duplicates first parent hash", () => {
            // M has parents [A, A] — duplicate parent hash.
            // First parent A occupies lane 0 (straight connection).
            // Second parent A: allocateOrFindLane finds A at lane 0 (same as commit lane).
            // The guard should skip this connection and log an error.
            const hashA = "aaa0000000000000000000000000000000000001";
            const hashM = "mmm0000000000000000000000000000000000002";

            const commits = [
                makeCommit(hashM, [hashA, hashA]),  // duplicate parent hash
                makeCommit(hashA, null),
            ];

            const errorSpy = vi.spyOn(console, "error").mockImplementation(() => {});

            const result = computeLanes(commits);

            // M should have only the straight connection (duplicate merge skipped).
            const mAssignment = result.find(a => a.commitHash === hashM);
            expect(mAssignment).toBeDefined();
            expect(mAssignment!.connections).toHaveLength(1);
            expect(mAssignment!.connections[0]).toEqual({fromLane: 0, toLane: 0, type: "straight"});

            // No connection where fromLane === toLane with merge type should exist.
            expect(mAssignment!.connections.every(c =>
                c.type === "straight" || c.fromLane !== c.toLane,
            )).toBe(true);

            // The guard should have logged exactly one error.
            expect(errorSpy).toHaveBeenCalledOnce();
            expect(errorSpy.mock.calls[0][0]).toContain("[lane-computation] invariant violation");

            errorSpy.mockRestore();
        });

        it("does not crash and processes remaining parents after skipping duplicate", () => {
            // Octopus merge with duplicate: M has parents [A, A, B].
            // A is duplicate → skipped. B should still get a merge connection.
            const hashA = "aaa0000000000000000000000000000000000001";
            const hashB = "bbb0000000000000000000000000000000000002";
            const hashM = "mmm0000000000000000000000000000000000003";

            const commits = [
                makeCommit(hashM, [hashA, hashA, hashB]),  // A duplicated, then B
                makeCommit(hashA, null),
                makeCommit(hashB, null),
            ];

            const errorSpy = vi.spyOn(console, "error").mockImplementation(() => {});

            const result = computeLanes(commits);

            const mAssignment = result.find(a => a.commitHash === hashM);
            expect(mAssignment).toBeDefined();
            // straight(A) + skipped(A dup) + merge-right(B) = 2 connections
            expect(mAssignment!.connections).toHaveLength(2);
            expect(mAssignment!.connections[0]).toEqual({fromLane: 0, toLane: 0, type: "straight"});
            expect(mAssignment!.connections[1].type).toBe("merge-right");
            expect(mAssignment!.connections[1].toLane).not.toBe(mAssignment!.connections[1].fromLane);

            // Error logged for the duplicate parent.
            expect(errorSpy).toHaveBeenCalledOnce();

            errorSpy.mockRestore();
        });
    });
});
