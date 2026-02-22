// NOTE: This file contains hand-written TypeScript types for the application.
// Some types below (AppConfigWorktree, AppConfigAgentModel, etc.) are derived
// from the auto-generated wailsjs/go/models.ts via `Pick<>` to maintain a
// single source of truth for config shapes.
//
// WARNING: This type must stay in sync with auto-generated models.ts.
//
// DRIFT RISK: Types that are NOT derived via Pick (SessionSnapshot,
// WindowSnapshot, PaneSnapshot, LayoutNode, SessionWorktreeInfo) are
// duplicated from models.ts by hand. If the corresponding Go structs
// change without regenerating models.ts, or if models.ts is regenerated
// without updating this file, the two definitions will silently diverge
// and cause runtime type errors.
//
// KNOWN DRIFT (I-23): models.ts auto-generates `created_at: any` for
// Go's time.Time fields because the Wails generator cannot infer the
// serialized type. This file intentionally narrows it to `string` because
// encoding/json serializes time.Time as an RFC 3339 / ISO 8601 string.
// If models.ts is regenerated and the `any` type reappears, it is expected
// and does NOT indicate a real drift -- this file's `string` is the
// correct narrowed type.
//
// SYNC STRATEGY:
//   1. Config types: Use `Pick<>` from models.ts (already done for AppConfig*).
//   2. Snapshot types: Manual sync required. When a Go struct in
//      internal/tmux/types.go changes, update BOTH models.ts (via `wails generate`)
//      AND the interfaces below.
//   3. Periodic check: Run `diff` between the tmux namespace in models.ts and
//      the interfaces below to catch silent divergence:
//      Unix (bash):
//        grep -A20 'export class SessionSnapshot' wailsjs/go/models.ts
//        grep -A20 'export interface SessionSnapshot' src/types/tmux.ts
//      PowerShell:
//        Select-String -Path wailsjs/go/models.ts -Pattern 'export class SessionSnapshot' -Context 0,20
//        Select-String -Path src/types/tmux.ts -Pattern 'export interface SessionSnapshot' -Context 0,20
//
// Rule: whenever a Go struct that maps to a type in this file is modified,
// BOTH this file AND wailsjs/go/models.ts must be updated together.

import type {config as wailsConfig} from "../../wailsjs/go/models";

export type AppConfigWorktree = Pick<
    wailsConfig.WorktreeConfig,
    "enabled" | "force_cleanup" | "setup_scripts" | "copy_files" | "copy_dirs"
>;

export type AppConfigAgentModelOverride = Pick<wailsConfig.AgentModelOverride, "name" | "model">;

export type AppConfigAgentModel = Pick<wailsConfig.AgentModel, "from" | "to" | "overrides">;

/** Known key binding actions for tmux prefix key sequences. */
export type KnownKeyBinding =
    | "split-vertical"
    | "split-horizontal"
    | "toggle-zoom"
    | "kill-pane"
    | "detach-session";

type AppConfigBase = Pick<wailsConfig.Config, "shell" | "prefix" | "keys" | "quake_mode" | "global_hotkey">;

export type AppConfigClaudeEnv = Pick<wailsConfig.ClaudeEnvConfig, "default_enabled" | "vars">;

export type AppConfig = AppConfigBase & {
    worktree: AppConfigWorktree;
    agent_model?: AppConfigAgentModel;
    pane_env?: Record<string, string>;
    pane_env_default_enabled?: boolean;
    claude_env?: AppConfigClaudeEnv;
};

export type WailsConfigInput = AppConfig;

export interface ParsedConfigUpdatedEvent {
    config: AppConfig;
    version: number | null;
    updated_at_unix_milli: number | null;
}

// Keep backend JSON field names (snake_case) here and map to camelCase in UI state.
export interface ValidationRules {
    min_override_name_len: number;
}

export interface LayoutNode {
    type: string;
    direction?: string;
    ratio?: number;
    pane_id: number;
    children?: LayoutNode[];
}

export interface PaneSnapshot {
    id: string;
    index: number;
    title?: string;
    active: boolean;
    width: number;
    height: number;
}

export interface WindowSnapshot {
    id: number;
    name: string;
    layout?: LayoutNode;
    active_pane: number;
    panes: PaneSnapshot[];
}

export interface SessionSnapshot {
    id: number;
    name: string;
    // Go type: time.Time — serialized as RFC 3339 / ISO 8601 string by encoding/json.
    // Must match models.ts SessionSnapshot.created_at: string.
    created_at: string;
    is_idle: boolean;
    active_window_id: number;
    // Backend omits false via omitempty, so undefined means false.
    is_agent_team?: boolean;
    windows: WindowSnapshot[];
    worktree?: SessionWorktreeInfo;
    root_path?: string;
}

export interface SessionWorktreeInfo {
    // Backend uses omitempty for string fields, so empty values may be omitted.
    path?: string;
    repo_path?: string;
    branch_name?: string;
    base_branch?: string;
    is_detached: boolean;
}

export interface SessionSnapshotDelta {
    upserts: SessionSnapshot[];
    removed: string[];
}
