import type {FullCommitHash, GitGraphCommit, LaneAssignment, ParentConnection} from "./gitGraphTypes";

/**
 * Find the lane index for a hash if it already occupies a lane,
 * or allocate a new lane (reusing a null slot if available, otherwise appending).
 */
function allocateOrFindLane(activeLanes: (FullCommitHash | null)[], hash: FullCommitHash): number {
    const existing = activeLanes.indexOf(hash);
    if (existing !== -1) {
        return existing;
    }
    const nullSlot = activeLanes.indexOf(null);
    if (nullSlot !== -1) {
        activeLanes[nullSlot] = hash;
        return nullSlot;
    }
    activeLanes.push(hash);
    return activeLanes.length - 1;
}

/**
 * Computes lane assignments for a list of commits (newest first).
 * Each commit is assigned a lane (column) and connections to its parents.
 *
 * Algorithm:
 * 1. activeLanes[] tracks which commit hash each lane is waiting for.
 * 2. For each commit, find its lane in activeLanes (or allocate a new one).
 * 3. First parent inherits the same lane; additional parents reuse their tracked lane or are allocated a new one if not yet tracked.
 * 4. Compact trailing nulls from activeLanes.
 */
export function computeLanes(commits: readonly GitGraphCommit[]): LaneAssignment[] {
    const activeLanes: (FullCommitHash | null)[] = [];
    const assignments: LaneAssignment[] = [];

    for (const commit of commits) {
        // Find the lane for this commit using full hash for accurate matching,
        // or allocate a new lane if not yet tracked.
        const lane = allocateOrFindLane(activeLanes, commit.full_hash);

        const connections: ParentConnection[] = [];
        const parents = commit.parents ?? [];

        if (parents.length === 0) {
            // Root commit: release lane.
            activeLanes[lane] = null;
        } else {
            // First parent: stays on same lane.
            activeLanes[lane] = parents[0];
            connections.push({
                fromLane: lane,
                toLane: lane,
                type: "straight",
            });

            // Additional parents: merge lines.
            for (let i = 1; i < parents.length; i++) {
                const parentHash = parents[i];
                const parentLane = allocateOrFindLane(activeLanes, parentHash);

                // parentLane === lane should not happen in well-formed git history:
                // lane is occupied by parents[0] (set above), so allocateOrFindLane
                // returns lane only if parentHash === parents[0] (duplicate parent hash).
                // Guard against malformed input to prevent an invalid "merge-right"
                // connection where fromLane === toLane.
                if (parentLane === lane) {
                    // Invariant violation: duplicate parent hash or malformed input.
                    // Error-level because this indicates corrupted git data, but
                    // recovery is possible (skip the duplicate connection).
                    console.error(`[lane-computation] invariant violation: parentLane === lane (${lane}) for merge parent ${parentHash}`);
                    continue;
                }
                connections.push({
                    fromLane: lane,
                    toLane: parentLane,
                    type: parentLane < lane ? "merge-left" : "merge-right",
                });
            }
        }

        // Compact trailing nulls.
        while (activeLanes.length > 0 && activeLanes[activeLanes.length - 1] === null) {
            activeLanes.pop();
        }

        assignments.push({
            commitHash: commit.full_hash,
            lane,
            connections,
            activeLaneCount: activeLanes.length,
        });
    }

    return assignments;
}
