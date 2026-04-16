export interface RefLike<T> {
    current: T;
}

export function matchesCapturedSessionKey(capturedSessionKey: string, currentSessionKey: string): boolean {
    return capturedSessionKey === currentSessionKey;
}

export function shouldIgnoreSessionMutation(
    capturedSessionKey: string,
    isMountedRef: RefLike<boolean>,
    latestSessionKeyRef: RefLike<string>,
): boolean {
    return !isMountedRef.current || !matchesCapturedSessionKey(capturedSessionKey, latestSessionKeyRef.current);
}

export function shouldIgnoreSessionRequest(
    capturedSessionKey: string,
    requestToken: number,
    isMountedRef: RefLike<boolean>,
    latestSessionKeyRef: RefLike<string>,
    latestRequestTokenRef: RefLike<number>,
): boolean {
    return shouldIgnoreSessionMutation(capturedSessionKey, isMountedRef, latestSessionKeyRef)
        || latestRequestTokenRef.current !== requestToken;
}

export function shouldSkipSessionMutationRequest(
    capturedSessionKey: string,
    hasResolvedSessionKeyRef: RefLike<boolean>,
): boolean {
    return !hasResolvedSessionKeyRef.current || capturedSessionKey === "";
}
