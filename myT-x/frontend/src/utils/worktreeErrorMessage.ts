import type {UILanguage} from "../i18n";
import {toErrorMessage} from "./errorUtils";

type TranslationParams = Record<string, string | number>;

type TranslateFn = (key: string, defaultText: string, params?: TranslationParams) => string;

interface WorktreeErrorI18n {
    readonly language: UILanguage;
    readonly t: TranslateFn;
}

function formatTemplate(template: string, params?: TranslationParams): string {
    if (!params) {
        return template;
    }
    return template.replace(/\{(\w+)}/g, (_, key: string) => {
        const value = params[key];
        return value === undefined ? "" : String(value);
    });
}

function localize(
    ctx: WorktreeErrorI18n,
    key: string,
    jaText: string,
    enText: string,
    params?: TranslationParams,
): string {
    if (ctx.language === "en") {
        return formatTemplate(enText, params);
    }
    return ctx.t(key, jaText, params);
}

function detailAfterPrefix(message: string, prefix: string): string {
    return message.slice(prefix.length).trim();
}

// formatWorktreeErrorMessage localizes known worktree/session errors while
// preserving unknown backend detail as-is until the backend gains structured
// error codes for this flow.
export function formatWorktreeErrorMessage(
    err: unknown,
    ctx: WorktreeErrorI18n,
    fallbackJa: string,
    fallbackEn: string,
): string {
    const raw = toErrorMessage(err, ctx.language === "en" ? fallbackEn : fallbackJa).trim();
    if (raw === "") {
        return ctx.language === "en" ? fallbackEn : fallbackJa;
    }

    if (raw === "session name is required") {
        return localize(
            ctx,
            "worktree.error.sessionNameRequired",
            "セッション名を入力してください。",
            "Session name is required.",
        );
    }
    if (raw === "branch name is required for new worktree creation") {
        return localize(
            ctx,
            "worktree.error.branchNameRequired",
            "新しい worktree を作成するにはブランチ名が必要です。",
            "Branch name is required to create a new worktree.",
        );
    }
    if (raw.startsWith("invalid branch name:")) {
        return localize(
            ctx,
            "worktree.error.branchNameInvalid",
            "ブランチ名の形式が不正です。英数字、.、_、-、/ を使用し、先頭に .、-、/ は使わないでください。",
            "Invalid branch name. Use letters, numbers, ., _, -, and /, and do not start with ., -, or /.",
        );
    }
    if (raw.startsWith("pull before worktree creation failed:")) {
        const detail = detailAfterPrefix(raw, "pull before worktree creation failed:");
        return localize(
            ctx,
            "worktree.error.pullBeforeFailed",
            "worktree 作成前の pull に失敗しました: {error}",
            "Pull before worktree creation failed: {error}",
            {error: detail || raw},
        );
    }
    if (raw === "worktree feature is disabled in config") {
        return localize(
            ctx,
            "worktree.error.featureDisabled",
            "worktree 機能は設定で無効化されています。",
            "The worktree feature is disabled in the configuration.",
        );
    }
    if (raw.startsWith("not a git repository:")) {
        const path = detailAfterPrefix(raw, "not a git repository:");
        return localize(
            ctx,
            "worktree.error.notGitRepository",
            "選択したフォルダは Git リポジトリではありません: {path}",
            "The selected folder is not a Git repository: {path}",
            {path: path || raw},
        );
    }
    if (raw.startsWith("failed to detect current branch:")) {
        const detail = detailAfterPrefix(raw, "failed to detect current branch:");
        return localize(
            ctx,
            "worktree.error.currentBranchDetectionFailed",
            "現在のブランチを判定できませんでした: {error}",
            "Failed to detect the current branch: {error}",
            {error: detail || raw},
        );
    }
    if (raw.startsWith("failed to check HEAD state:")) {
        const detail = detailAfterPrefix(raw, "failed to check HEAD state:");
        return localize(
            ctx,
            "worktree.error.headStateCheckFailed",
            "HEAD の状態を確認できませんでした: {error}",
            "Failed to check the HEAD state: {error}",
            {error: detail || raw},
        );
    }
    if (raw.startsWith("failed to create worktree:")) {
        const detail = detailAfterPrefix(raw, "failed to create worktree:");
        return localize(
            ctx,
            "worktree.error.createFailed",
            "worktree の作成に失敗しました: {error}",
            "Failed to create the worktree: {error}",
            {error: detail || raw},
        );
    }
    if (raw.startsWith("failed to create branch:")) {
        const detail = detailAfterPrefix(raw, "failed to create branch:");
        return localize(
            ctx,
            "worktree.error.promoteCreateBranchFailed",
            "ブランチの作成に失敗しました: {error}",
            "Failed to create the branch: {error}",
            {error: detail || raw},
        );
    }
    if (raw.includes(" is not a detached worktree")) {
        return localize(
            ctx,
            "worktree.error.notDetached",
            "このセッションは detached worktree ではないため、ブランチへ昇格できません。",
            "This session is not a detached worktree, so it cannot be promoted to a branch.",
        );
    }

    return raw;
}
