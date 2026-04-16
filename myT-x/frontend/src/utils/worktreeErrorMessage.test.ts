import {describe, expect, it} from "vitest";
import {formatWorktreeErrorMessage} from "./worktreeErrorMessage";

const jaContext = {
    language: "ja" as const,
    t: (_key: string, defaultText: string, params?: Record<string, string | number>) =>
        defaultText.replace(/\{(\w+)\}/g, (_, key: string) => String(params?.[key] ?? "")),
};

const enContext = {
    language: "en" as const,
    t: (_key: string, defaultText: string) => defaultText,
};

describe("formatWorktreeErrorMessage", () => {
    it("localizes invalid branch names for new-session flows", () => {
        const result = formatWorktreeErrorMessage(
            "invalid branch name: invalid branch name: feature..broken",
            jaContext,
            "セッションの作成に失敗しました。",
            "Failed to create the session.",
        );

        expect(result).toContain("ブランチ名の形式が不正");
    });

    it("localizes strict pull-before-create failures", () => {
        const result = formatWorktreeErrorMessage(
            "pull before worktree creation failed: fatal: no remote configured",
            enContext,
            "セッションの作成に失敗しました。",
            "Failed to create the session.",
        );

        expect(result).toBe("Pull before worktree creation failed: fatal: no remote configured");
    });

    it("localizes not-a-git-repository errors", () => {
        const result = formatWorktreeErrorMessage(
            "not a git repository: C:/repo",
            enContext,
            "セッションの作成に失敗しました。",
            "Failed to create the session.",
        );

        expect(result).toBe("The selected folder is not a Git repository: C:/repo");
    });

    it("localizes failed-to-check-head-state errors", () => {
        const result = formatWorktreeErrorMessage(
            "failed to check HEAD state: fatal: bad revision",
            jaContext,
            "セッションの作成に失敗しました。",
            "Failed to create the session.",
        );

        expect(result).toBe("HEAD の状態を確認できませんでした: fatal: bad revision");
    });

    it("preserves unknown backend detail", () => {
        const result = formatWorktreeErrorMessage(
            "unexpected backend failure",
            jaContext,
            "セッションの作成に失敗しました。",
            "Failed to create the session.",
        );

        expect(result).toBe("unexpected backend failure");
    });
});
