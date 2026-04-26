const DIFF_REVIEW_SCOPE_SEPARATOR = "::";
const DIFF_REVIEW_LINE_SEPARATOR = "\u001f";

export function buildDiffReviewSessionKey(sessionID: number): string {
    return `session:${sessionID}`;
}

export function buildScopedDiffReviewPrefix(sessionKey: string): string {
    return sessionKey === "" ? "" : `${sessionKey}${DIFF_REVIEW_SCOPE_SEPARATOR}`;
}

export function buildScopedDiffReviewKey(sessionKey: string, key: string): string {
    const prefix = buildScopedDiffReviewPrefix(sessionKey);
    return prefix === "" ? key : `${prefix}${key}`;
}

export function buildDiffReviewDraftKey(sessionKey: string, filePath: string, lineKey: string): string {
    return buildScopedDiffReviewKey(sessionKey, `${filePath}${DIFF_REVIEW_LINE_SEPARATOR}${lineKey}`);
}
