import type {WorkingDiffResult} from "./diffViewTypes";

const FIELD_SEPARATOR = "\u001f";
const FILE_SEPARATOR = "\u0000";

function compareNullableStrings(left: string, right: string): number {
    return left.localeCompare(right);
}

function hashText(text: string): string {
    let hash = 2166136261;
    for (let i = 0; i < text.length; i++) {
        hash ^= text.charCodeAt(i);
        hash = Math.imul(hash, 16777619);
    }
    return (hash >>> 0).toString(16);
}

export function buildDiffReviewGenerationKey(diffResult: WorkingDiffResult | null): string {
    if (diffResult == null) return "";

    const fileSegments = [...(diffResult.files ?? [])]
        .sort((left, right) => {
            const pathCompare = compareNullableStrings(left.path, right.path);
            if (pathCompare !== 0) {
                return pathCompare;
            }
            const oldPathCompare = compareNullableStrings(left.old_path, right.old_path);
            if (oldPathCompare !== 0) {
                return oldPathCompare;
            }
            return compareNullableStrings(left.status, right.status);
        })
        .map((file) =>
        [
            file.path,
            file.old_path,
            file.status,
            String(file.additions),
            String(file.deletions),
            hashText(file.diff),
        ].join(FIELD_SEPARATOR),
    );

    return [
        String(diffResult.total_added ?? 0),
        String(diffResult.total_deleted ?? 0),
        diffResult.truncated ? "1" : "0",
        ...fileSegments,
    ].join(FILE_SEPARATOR);
}

export function shouldResetDiffReviewState(
    previousSessionKey: string,
    previousGenerationKey: string,
    nextSessionKey: string,
    nextGenerationKey: string,
): boolean {
    if (previousSessionKey === "" || previousSessionKey !== nextSessionKey) {
        return false;
    }
    if (previousGenerationKey === "" || nextGenerationKey === "") {
        return false;
    }

    return previousGenerationKey !== nextGenerationKey;
}
