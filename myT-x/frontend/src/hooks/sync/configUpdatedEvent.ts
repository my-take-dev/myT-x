import type {AppConfig, ParsedConfigUpdatedEvent} from "../../types/tmux";
import {asObject} from "../../utils/typeGuards";

function isAppConfigPayload(payload: unknown): payload is AppConfig {
    const cfg = asObject<Record<string, unknown>>(payload);
    if (!cfg) {
        return false;
    }
    if (typeof cfg.shell !== "string" || cfg.shell.trim() === "") {
        return false;
    }
    if (typeof cfg.prefix !== "string" || cfg.prefix.trim() === "") {
        return false;
    }
    if (typeof cfg.quake_mode !== "boolean") {
        return false;
    }
    if (typeof cfg.global_hotkey !== "string") {
        return false;
    }

    const keys = asObject<Record<string, unknown>>(cfg.keys);
    if (!keys) {
        return false;
    }
    for (const value of Object.values(keys)) {
        if (typeof value !== "string") {
            return false;
        }
    }

    const worktree = asObject<Record<string, unknown>>(cfg.worktree);
    if (!worktree) {
        return false;
    }
    if (typeof worktree.enabled !== "boolean" || typeof worktree.force_cleanup !== "boolean") {
        return false;
    }
    if (
        worktree.setup_script_timeout_seconds !== undefined &&
        (typeof worktree.setup_script_timeout_seconds !== "number" || worktree.setup_script_timeout_seconds <= 0)
    ) {
        return false;
    }
    return true;
}

/**
 * Parse a config:updated payload with synthetic-version compatibility.
 *
 * Older payload shapes may omit the nested `config` object or the `version`
 * field. Callers should treat `version: null` as "apply in arrival order".
 */
export function parseConfigUpdatedPayload(payload: unknown): ParsedConfigUpdatedEvent | null {
    const event = asObject<Record<string, unknown>>(payload);
    if (!event) {
        return null;
    }

    const nestedConfig = asObject<Record<string, unknown>>(event.config);
    const rawVersion = event.version;
    let version: number | null = null;
    if (typeof rawVersion === "number" && Number.isSafeInteger(rawVersion) && rawVersion > 0) {
        version = rawVersion;
    }
    const rawUpdatedAt = event.updated_at_unix_milli;
    let updatedAtUnixMilli: number | null = null;
    if (typeof rawUpdatedAt === "number" && Number.isSafeInteger(rawUpdatedAt) && rawUpdatedAt > 0) {
        updatedAtUnixMilli = rawUpdatedAt;
    }

    if (nestedConfig) {
        if (!isAppConfigPayload(nestedConfig)) {
            return null;
        }
        return {config: nestedConfig, version, updated_at_unix_milli: updatedAtUnixMilli};
    }

    if (!isAppConfigPayload(event)) {
        return null;
    }

    const flatEvent = event as Record<string, unknown>;
    const {version: _version, updated_at_unix_milli: _updatedAtUnixMilli, ...flatConfig} = flatEvent;
    return {config: flatConfig as AppConfig, version, updated_at_unix_milli: updatedAtUnixMilli};
}
