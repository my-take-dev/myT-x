import {describe, expect, it} from "vitest";
import {sanitizeSessionName, suggestSessionName} from "./sanitizeSessionName";

describe("sanitizeSessionName", () => {
    it("matches the backend sanitizer for common inputs", () => {
        expect(sanitizeSessionName("my-session")).toBe("my-session");
        expect(sanitizeSessionName("a.b:c")).toBe("a-b-c");
        expect(sanitizeSessionName("feature/login")).toBe("feature/login");
        expect(sanitizeSessionName("---alpha...beta:::")).toBe("alpha-beta");
        expect(sanitizeSessionName("   ")).toBe("   ");
    });

    it("applies the requested fallback when sanitizing to an empty name", () => {
        expect(suggestSessionName("", "session")).toBe("session");
        expect(suggestSessionName("...", "worktree-session")).toBe("worktree-session");
    });
});
