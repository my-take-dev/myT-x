import type {ClaudeEnvEntry, OverrideEntry, PaneEnvEntry} from "./types";
import {EFFORT_LEVEL_KEY, MIN_OVERRIDE_NAME_LEN_FALLBACK, VALID_EFFORT_LEVELS} from "./constants";
import {VIEWER_SHORTCUTS} from "../viewer/viewerShortcutDefinitions";
import {getEffectiveViewerShortcut, hasShortcutModifier, normalizeShortcut} from "../viewer/viewerShortcutUtils";
import {translateSettings} from "./settingsI18n";

export function validateViewerShortcuts(
    viewerShortcuts: Record<string, string>,
    globalHotkey: string = "",
): Record<string, string> {
    const errors: Record<string, string> = {};
    const ownersByShortcut = new Map<string, string[]>();
    const normalizedGlobalHotkey = normalizeShortcut(globalHotkey);

    const setShortcutError = (viewId: string, message: string) => {
        const key = `viewer_shortcut_${viewId}`;
        if (!errors[key]) {
            errors[key] = message;
        }
    };

    for (const {viewId, defaultShortcut} of VIEWER_SHORTCUTS) {
        const effectiveShortcut = getEffectiveViewerShortcut(viewerShortcuts[viewId], defaultShortcut);
        if (!effectiveShortcut) {
            continue;
        }
        if (!hasShortcutModifier(effectiveShortcut)) {
            setShortcutError(
                viewId,
                translateSettings(
                    "settings.validation.viewerShortcut.modifierRequired",
                    "修飾キーが必要です",
                    "Modifier key is required.",
                ),
            );
            continue;
        }
        const normalized = normalizeShortcut(effectiveShortcut);
        if (!normalized) {
            continue;
        }
        if (normalizedGlobalHotkey !== "" && normalized === normalizedGlobalHotkey) {
            setShortcutError(
                viewId,
                translateSettings(
                    "settings.validation.viewerShortcut.duplicateWithGlobalHotkey",
                    "グローバルホットキーと重複しています",
                    "Duplicated with global hotkey.",
                ),
            );
            continue;
        }
        const owners = ownersByShortcut.get(normalized);
        if (owners) {
            owners.push(viewId);
        } else {
            ownersByShortcut.set(normalized, [viewId]);
        }
    }

    for (const owners of ownersByShortcut.values()) {
        if (owners.length < 2) {
            continue;
        }
        for (const viewId of owners) {
            setShortcutError(
                viewId,
                translateSettings(
                    "settings.validation.viewerShortcut.duplicateAcrossViews",
                    "他のビューと重複しています",
                    "Duplicated with another view.",
                ),
            );
        }
    }

    return errors;
}

// SYNC: Must match blockedEnvironmentKeys in internal/tmux/command_router_terminal.go
// and TestBlockedEnvironmentKeysCountGuard in command_router_terminal_test.go
/** Environment variable names that must not be overridden (mirrors backend blockedEnvironmentKeys). */
const BLOCKED_ENV_KEYS = new Set([
    "PATH", "PATHEXT", "COMSPEC", "SYSTEMROOT", "WINDIR",
    "SYSTEMDRIVE", "APPDATA", "LOCALAPPDATA", "PSMODULEPATH",
    "TEMP", "TMP", "USERPROFILE",
]);

/** POSIX準拠の環境変数名パターン: 英字またはアンダースコアで始まり、英数字とアンダースコアのみ */
const ENV_VAR_NAME_PATTERN = /^[A-Za-z_][A-Za-z0-9_]*$/;

export function validateAgentModelSettings(
    agentFrom: string,
    agentTo: string,
    overrides: OverrideEntry[],
    minOverrideNameLen = MIN_OVERRIDE_NAME_LEN_FALLBACK,
): Record<string, string> {
    const errors: Record<string, string> = {};

    const from = agentFrom.trim();
    const to = agentTo.trim();
    if ((from && !to) || (!from && to)) {
        errors["agent_model"] = translateSettings(
            "settings.validation.agentModel.fromToRequired",
            "fromとtoは両方同時に指定が必要です",
            "Both from and to must be specified together.",
        );
    }

    overrides.forEach((ov, i) => {
        const name = ov.name.trim();
        const model = ov.model.trim();
        if (!name && !model) {
            return;
        }
        if (!name && model) {
            errors[`override_name_${i}`] = translateSettings(
                "settings.validation.agentModel.overrideNameRequiredWhenModelProvided",
                "モデルが指定されている場合、名前は必須です",
                "Name is required when a model is specified.",
            );
            return;
        }
        const runeLen = [...name].length;
        if (runeLen < minOverrideNameLen) {
            errors[`override_name_${i}`] = translateSettings(
                "settings.validation.agentModel.overrideNameMinLength",
                "名前は{min}文字以上必要です (現在: {current}文字)",
                "Name must be at least {min} characters (current: {current}).",
                {min: minOverrideNameLen, current: runeLen},
            );
        }
        if (name && !model) {
            errors[`override_model_${i}`] = translateSettings(
                "settings.validation.agentModel.overrideModelRequired",
                "モデルは必須です",
                "Model is required.",
            );
        }
    });

    return errors;
}

/** Options for validateEnvEntries to control prefix-specific validation. */
interface EnvEntryValidationOptions {
    /** Whether to reject EFFORT_LEVEL_KEY as a manually entered key. */
    rejectEffortLevelKey?: boolean;
}

/**
 * Shared validation for environment variable entry lists.
 * Both pane-env and claude-env entries share the same shape and rules;
 * only the error key prefix and the EFFORT_LEVEL_KEY check differ.
 *
 * NOTE: The parameter uses a structural subset `{ key, value }` rather than
 * PaneEnvEntry | ClaudeEnvEntry because the `id` field is irrelevant to
 * validation. The canonical type definitions live in ./types.ts.
 */
function validateEnvEntries(
    entries: ReadonlyArray<{ key: string; value: string }>,
    keyPrefix: string,
    options: EnvEntryValidationOptions = {},
): Record<string, string> {
    const errors: Record<string, string> = {};
    const seenKeys = new Set<string>();

    entries.forEach((entry, i) => {
        const key = entry.key.trim();
        const value = entry.value.trim();
        if (!key && !value) {
            return;
        }
        if (!key && value) {
            errors[`${keyPrefix}_key_${i}`] = translateSettings(
                "settings.validation.env.keyRequired",
                "変数名は必須です",
                "Variable name is required.",
            );
            return;
        }
        if (!ENV_VAR_NAME_PATTERN.test(key)) {
            errors[`${keyPrefix}_key_${i}`] = translateSettings(
                "settings.validation.env.keyPatternInvalid",
                "変数名は英字・アンダースコアで始まり、英数字・アンダースコアのみ使用できます",
                "Variable name must start with a letter or underscore, and contain only letters, digits, and underscores.",
            );
            return;
        }
        const upper = key.toUpperCase();
        if (key && !value) {
            errors[`${keyPrefix}_val_${i}`] = translateSettings(
                "settings.validation.env.valueRequired",
                "値は必須です",
                "Value is required.",
            );
            return;
        }
        if (options.rejectEffortLevelKey && upper === EFFORT_LEVEL_KEY) {
            errors[`${keyPrefix}_key_${i}`] = translateSettings(
                "settings.validation.env.useDedicatedEffortLevelField",
                "この変数は上部の専用フィールドで設定してください",
                "Set this variable in the dedicated field above.",
            );
            return;
        }
        if (BLOCKED_ENV_KEYS.has(upper)) {
            errors[`${keyPrefix}_key_${i}`] = translateSettings(
                "settings.validation.env.systemKeyNotAllowed",
                "システム変数は設定できません",
                "System variables cannot be configured.",
            );
            return;
        }
        if (seenKeys.has(upper)) {
            errors[`${keyPrefix}_key_${i}`] = translateSettings(
                "settings.validation.env.duplicateKey",
                "変数名が重複しています",
                "Variable name is duplicated.",
            );
        }
        seenKeys.add(upper);
    });

    return errors;
}

export function validatePaneEnvSettings(
    entries: PaneEnvEntry[],
    effortLevel: string,
): Record<string, string> {
    const errors: Record<string, string> = {};

    if (!VALID_EFFORT_LEVELS.has(effortLevel.trim().toLowerCase())) {
        errors["pane_env_effort"] = translateSettings(
            "settings.validation.paneEnv.effortLevelInvalid",
            "low, medium, high のいずれかを指定してください",
            "Specify one of low, medium, or high.",
        );
    }

    return {...errors, ...validateEnvEntries(entries, "pane_env", {rejectEffortLevelKey: true})};
}

export function validateClaudeEnvSettings(
    entries: ClaudeEnvEntry[],
): Record<string, string> {
    return validateEnvEntries(entries, "claude_env");
}

// NOTE: This pattern intentionally accepts POSIX root "/" in addition to
// Windows drive paths (C:\) and UNC paths (\\). The Go backend
// (config.validateDefaultSessionDir) expands ~ and env vars then checks
// filepath.IsAbs, which rejects bare "/" on Windows. The frontend accepts
// it here to avoid confusing "not absolute" errors for WSL-style paths;
// the backend serves as the authoritative gate.
const ABSOLUTE_SESSION_DIR_PATTERN = /^(?:~(?:[\\/]|$)|%[A-Za-z_][A-Za-z0-9_]*%(?:[\\/]|$)|\$(?:[A-Za-z_][A-Za-z0-9_]*|\{[A-Za-z_][A-Za-z0-9_]*\})(?:[\\/]|$)|[A-Za-z]:[\\/]|[\\/]{2}|\/)/;
const ABSOLUTE_OR_DRIVE_PATH_PATTERN = /^(?:[A-Za-z]:|[\\/]{2}|[\\/])/;

export function validateDefaultSessionDir(rawPath: string): Record<string, string> {
    const path = rawPath.trim();
    if (path === "") {
        return {};
    }
    if (!ABSOLUTE_SESSION_DIR_PATTERN.test(path)) {
        return {
            default_session_dir: translateSettings(
                "settings.validation.defaultSessionDir.absolutePathRequired",
                "絶対パスを指定してください",
                "Specify an absolute path.",
            ),
        };
    }
    return {};
}

export function normalizeRelativePath(path: string): string {
    const segments = path.split("/");
    const stack: string[] = [];

    for (const segment of segments) {
        if (!segment || segment === ".") {
            continue;
        }
        if (segment === "..") {
            if (stack.length === 0 || stack[stack.length - 1] === "..") {
                stack.push(segment);
            } else {
                stack.pop();
            }
            continue;
        }
        stack.push(segment);
    }

    return stack.length === 0 ? "." : stack.join("/");
}

export function isUnsafeWorktreeCopyPath(rawPath: string): boolean {
    const trimmed = rawPath.trim();
    if (!trimmed) {
        return false;
    }
    if (ABSOLUTE_OR_DRIVE_PATH_PATTERN.test(trimmed)) {
        return true;
    }

    const normalized = trimmed.replace(/\\/g, "/");
    const cleaned = normalizeRelativePath(normalized);
    return cleaned === "." || cleaned === ".." || cleaned.startsWith("../");
}

export function validateWorktreeCopyPathSettings(
    copyFiles: string[],
    copyDirs: string[],
): Record<string, string> {
    const errors: Record<string, string> = {};
    const pathErrorMessage = translateSettings(
        "settings.validation.worktreeCopy.relativePathOnly",
        "相対パスのみ指定できます（絶対パス、'.'、'..' は不可）",
        "Only relative paths are allowed (absolute paths, '.', '..' are not allowed).",
    );

    let fileErrorCount = 0;
    copyFiles.forEach((path, i) => {
        if (!path.trim()) {
            return;
        }
        if (isUnsafeWorktreeCopyPath(path)) {
            errors[`wt_copy_files_${i}`] = pathErrorMessage;
            fileErrorCount++;
        }
    });
    if (fileErrorCount > 0) {
        errors["wt_copy_files"] = translateSettings(
            "settings.validation.worktreeCopy.filesInvalidCount",
            "コピーファイルに不正なパスが{count}件あります",
            "Copy files contain {count} invalid path(s).",
            {count: fileErrorCount},
        );
    }

    let dirErrorCount = 0;
    copyDirs.forEach((path, i) => {
        if (!path.trim()) {
            return;
        }
        if (isUnsafeWorktreeCopyPath(path)) {
            errors[`wt_copy_dirs_${i}`] = pathErrorMessage;
            dirErrorCount++;
        }
    });
    if (dirErrorCount > 0) {
        errors["wt_copy_dirs"] = translateSettings(
            "settings.validation.worktreeCopy.dirsInvalidCount",
            "コピーディレクトリに不正なパスが{count}件あります",
            "Copy directories contain {count} invalid path(s).",
            {count: dirErrorCount},
        );
    }

    return errors;
}
