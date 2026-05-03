import type {UsageSource} from "./useUsageDashboard";

export const USAGE_SOURCE_IDS = ["claude", "codex"] as const satisfies ReadonlyArray<UsageSource>;
// The default cap mirrors the available sources. A smaller cap needs a
// visible locked-state message in the dashboard controls.
export const MAX_COMPARISON_SOURCES = USAGE_SOURCE_IDS.length;

export type NonEmptyReadonlyArray<T> = readonly [T, ...T[]];

export function getInitialComparisonSources(): NonEmptyReadonlyArray<UsageSource> {
    return USAGE_SOURCE_IDS;
}

export function getComparisonSourceLockedReason<T extends string>(
    current: NonEmptyReadonlyArray<T>,
    source: T,
    minimumSelectionMessage: string,
): string | null {
    if (current.includes(source) && current.length === 1) {
        return minimumSelectionMessage;
    }
    return null;
}

export function toggleComparisonSourceSelection<T extends string>(
    current: NonEmptyReadonlyArray<T>,
    source: T,
    maxSources: number = MAX_COMPARISON_SOURCES,
): NonEmptyReadonlyArray<T> {
    if (current.includes(source)) {
        if (current.length === 1) return current;
        return toNonEmptyReadonlyArray(current.filter((item) => item !== source), current);
    }
    if (current.length >= maxSources) return current;
    return [current[0], ...current.slice(1), source];
}

function toNonEmptyReadonlyArray<T>(
    items: ReadonlyArray<T>,
    fallback: NonEmptyReadonlyArray<T>,
): NonEmptyReadonlyArray<T> {
    if (items.length === 0) {
        return fallback;
    }
    return [items[0] as T, ...items.slice(1)];
}
