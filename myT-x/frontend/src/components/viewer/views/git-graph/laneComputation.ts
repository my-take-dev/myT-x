import type { GitGraphCommit, LaneAssignment, ParentConnection } from "./gitGraphTypes";

/**
 * Computes lane assignments for a list of commits (newest first).
 * Each commit is assigned a lane (column) and connections to its parents.
 *
 * Algorithm:
 * 1. activeLanes[] tracks which commit hash each lane is waiting for.
 * 2. For each commit, find its lane in activeLanes (or allocate a new one).
 * 3. First parent inherits the same lane; additional parents get new lanes.
 * 4. Compact trailing nulls from activeLanes.
 */
export function computeLanes(commits: GitGraphCommit[]): LaneAssignment[] {
  const activeLanes: (string | null)[] = [];
  const assignments: LaneAssignment[] = [];

  for (const commit of commits) {
    // Find the lane for this commit using full hash for accurate matching.
    let lane = activeLanes.indexOf(commit.full_hash);
    if (lane === -1) {
      // New branch: allocate the first available (null) slot, or append.
      lane = activeLanes.indexOf(null);
      if (lane === -1) {
        lane = activeLanes.length;
        activeLanes.push(commit.full_hash);
      } else {
        activeLanes[lane] = commit.full_hash;
      }
    }

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
        let parentLane = activeLanes.indexOf(parentHash);
        if (parentLane === -1) {
          // Allocate a new lane for this parent.
          parentLane = activeLanes.indexOf(null);
          if (parentLane === -1) {
            parentLane = activeLanes.length;
            activeLanes.push(parentHash);
          } else {
            activeLanes[parentLane] = parentHash;
          }
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
