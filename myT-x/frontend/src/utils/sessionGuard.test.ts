import {describe, expect, it} from "vitest";
import {
    matchesCapturedSessionKey,
    shouldIgnoreSessionMutation,
    shouldIgnoreSessionRequest,
    shouldSkipSessionMutationRequest,
    type RefLike,
} from "./sessionGuard";

function ref<T>(current: T): RefLike<T> {
    return {current};
}

describe("sessionGuard", () => {
    it("matchesCapturedSessionKey returns true only for the same session key", () => {
        expect(matchesCapturedSessionKey("alpha:1", "alpha:1")).toBe(true);
        expect(matchesCapturedSessionKey("alpha:1", "beta:2")).toBe(false);
    });

    it("shouldIgnoreSessionMutation rejects unmounted handlers", () => {
        expect(shouldIgnoreSessionMutation("alpha:1", ref(false), ref("alpha:1"))).toBe(true);
    });

    it("shouldIgnoreSessionMutation rejects stale session results", () => {
        expect(shouldIgnoreSessionMutation("alpha:1", ref(true), ref("beta:2"))).toBe(true);
    });

    it("shouldIgnoreSessionMutation accepts mounted handlers for the active session", () => {
        expect(shouldIgnoreSessionMutation("alpha:1", ref(true), ref("alpha:1"))).toBe(false);
    });

    it("shouldIgnoreSessionRequest rejects stale request tokens", () => {
        expect(shouldIgnoreSessionRequest("alpha:1", 1, ref(true), ref("alpha:1"), ref(2))).toBe(true);
    });

    it("shouldIgnoreSessionRequest accepts the latest mounted request", () => {
        expect(shouldIgnoreSessionRequest("alpha:1", 2, ref(true), ref("alpha:1"), ref(2))).toBe(false);
    });

    it("shouldSkipSessionMutationRequest skips unresolved session keys", () => {
        expect(shouldSkipSessionMutationRequest("", ref(false))).toBe(true);
        expect(shouldSkipSessionMutationRequest("", ref(true))).toBe(true);
    });

    it("shouldSkipSessionMutationRequest accepts resolved non-empty session keys", () => {
        expect(shouldSkipSessionMutationRequest("alpha:1", ref(true))).toBe(false);
    });
});
