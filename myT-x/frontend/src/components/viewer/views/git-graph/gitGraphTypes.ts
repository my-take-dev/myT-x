declare const shortCommitHashBrand: unique symbol;
declare const fullCommitHashBrand: unique symbol;

export type ShortCommitHash = string & { readonly [shortCommitHashBrand]: "ShortCommitHash" };
export type FullCommitHash = string & { readonly [fullCommitHashBrand]: "FullCommitHash" };

/**
 * Frontend view of a git-graph commit with branded hash types.
 * Produced by normalizeGitGraphCommit() from the raw backend payload (GitGraphCommitRaw).
 * The branded ShortCommitHash / FullCommitHash types prevent hash-kind mix-ups at compile time.
 */
export type GitGraphCommit = Omit<GitGraphCommitRaw, "hash" | "full_hash" | "parents"> & {
    readonly hash: ShortCommitHash;
    readonly full_hash: FullCommitHash;
    readonly parents: readonly FullCommitHash[] | null;
};

/**
 * @internal Raw backend payload before branding.
 * Not exported — use {@link GitGraphCommit} (branded) in application and test code.
 */
interface GitGraphCommitRaw {
    readonly hash: string;
    readonly full_hash: string;
    readonly parents: readonly string[] | null;
    readonly subject: string;
    readonly author_name: string;
    readonly author_date: string;
    /** Branch/tag ref names pointing to this commit, or null if none. */
    readonly refs: readonly string[] | null;
}

function toShortCommitHash(hash: string): ShortCommitHash {
    return hash as ShortCommitHash;
}

function toFullCommitHash(hash: string): FullCommitHash {
    return hash as FullCommitHash;
}

/**
 * Convert a raw backend commit into a branded-hash GitGraphCommit.
 *
 * - `hash` is branded as ShortCommitHash, `full_hash` as FullCommitHash.
 * - `parents` strings are each branded as FullCommitHash; null parents are preserved as null.
 */
export function normalizeGitGraphCommit(raw: GitGraphCommitRaw): GitGraphCommit {
    return {
        ...raw,
        hash: toShortCommitHash(raw.hash),
        full_hash: toFullCommitHash(raw.full_hash),
        parents: raw.parents ? raw.parents.map(toFullCommitHash) : null,
    };
}

export function normalizeGitGraphCommits(raw: readonly GitGraphCommitRaw[]): readonly GitGraphCommit[] {
    return raw.map(normalizeGitGraphCommit);
}

/** Backend GitStatusResult returned by DevPanelGitStatus. */
export interface GitStatusResult {
    readonly branch: string;
    readonly modified: readonly string[] | null;
    readonly staged: readonly string[] | null;
    readonly untracked: readonly string[] | null;
    readonly ahead: number;
    readonly behind: number;
}

/** Lane assignment for a single commit in the graph. */
export interface LaneAssignment {
    readonly commitHash: FullCommitHash;
    readonly lane: number;
    readonly connections: readonly ParentConnection[];
    /** Number of active lanes at this commit's row (used to size the SVG width). */
    readonly activeLaneCount: number;
}

/** Connection line from a commit to one of its parents. */
export interface ParentConnection {
    readonly fromLane: number;
    readonly toLane: number;
    /**
     * Invariant (enforced by {@link computeLanes} in laneComputation.ts):
     * - "straight"    => fromLane === toLane
     * - "merge-left"  => toLane < fromLane
     * - "merge-right" => toLane > fromLane
     *
     * Precondition: parentLane !== lane for merge connections, because the
     * first parent occupies `lane` and any case where allocateOrFindLane would
     * return `lane` (e.g. duplicate parent hash) is skipped by the guard in
     * computeLanes. This guarantees strict inequality for merge types in practice.
     */
    readonly type: "straight" | "merge-left" | "merge-right";
}
