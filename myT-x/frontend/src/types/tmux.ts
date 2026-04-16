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
import type {ViewerSidebarMode} from "../utils/viewerSidebarMode";

type NonFunctionPropertyNames<T> = {
    [K in keyof T]: T[K] extends (...args: any[]) => unknown ? never : K;
}[keyof T];
type DataShape<T> = Pick<T, NonFunctionPropertyNames<T>>;
type ExactKeysMatch<Expected extends object, Actual extends object> =
    [Exclude<keyof Expected, keyof Actual> | Exclude<keyof Actual, keyof Expected>] extends [never] ? true : false;
type AssertTrue<T extends true> = T;

export type AppConfigWorktree = Pick<
    wailsConfig.WorktreeConfig,
    "enabled" | "force_cleanup" | "setup_scripts" | "setup_script_timeout_seconds" | "copy_files" | "copy_dirs"
>;

export type AppConfigAgentModelOverride = Pick<wailsConfig.AgentModelOverride, "name" | "model">;

export type AppConfigAgentModel = Pick<wailsConfig.AgentModel, "from" | "to" | "overrides">;

export type AppConfigMessageTemplate = Pick<wailsConfig.MessageTemplate, "name" | "message">;

export type AppConfigMCPServerConfigParam = Pick<
    wailsConfig.MCPServerConfigParam,
    "key" | "label" | "default_value" | "description"
>;

export type AppConfigMCPServerConfig = Pick<
    wailsConfig.MCPServerConfig,
    "id" | "name" | "description" | "kind" | "command" | "args" | "env" | "enabled" | "usage_sample" | "config_params"
> & {
    config_params?: AppConfigMCPServerConfigParam[];
};

export type AppConfigTaskScheduler = Pick<
    wailsConfig.TaskSchedulerConfig,
    "pre_exec_reset_delay_s" | "pre_exec_idle_timeout_s" | "pre_exec_target_mode" | "message_templates"
> & {
    message_templates?: AppConfigMessageTemplate[];
};

type WailsTaskSchedulerConfigShape = Pick<
    DataShape<wailsConfig.TaskSchedulerConfig>,
    "pre_exec_reset_delay_s" | "pre_exec_idle_timeout_s" | "pre_exec_target_mode" | "message_templates"
>;

type _AppConfigMessageTemplateKeyGuard =
    AssertTrue<ExactKeysMatch<DataShape<wailsConfig.MessageTemplate>, AppConfigMessageTemplate>>;
type _AppConfigTaskSchedulerKeyGuard =
    AssertTrue<ExactKeysMatch<WailsTaskSchedulerConfigShape, AppConfigTaskScheduler>>;

/** Known key binding actions for tmux prefix key sequences. */
export type KnownKeyBinding =
    | "split-vertical"
    | "split-horizontal"
    | "toggle-zoom"
    | "kill-pane"
    | "detach-session";

type AppConfigBase = Pick<
    wailsConfig.Config,
    "shell" | "prefix" | "keys" | "quake_mode" | "global_hotkey" | "viewer_sidebar_mode" | "default_session_dir" | "chat_overlay_percentage" | "websocket_port"
>;

export type AppConfigClaudeEnv = Pick<wailsConfig.ClaudeEnvConfig, "default_enabled" | "vars">;

export type AppConfig = AppConfigBase & {
    worktree: AppConfigWorktree;
    agent_model?: AppConfigAgentModel;
    pane_env?: Record<string, string>;
    pane_env_default_enabled?: boolean;
    claude_env?: AppConfigClaudeEnv;
    task_scheduler?: AppConfigTaskScheduler;
    viewer_shortcuts?: Record<string, string>;
    mcp_servers?: AppConfigMCPServerConfig[];
};

export type WailsConfigInput = {
    shell: AppConfigBase["shell"];
    prefix: AppConfigBase["prefix"];
    keys: AppConfigBase["keys"];
    quake_mode: AppConfigBase["quake_mode"];
    global_hotkey: AppConfigBase["global_hotkey"];
    worktree: AppConfigWorktree;
    agent_model: AppConfigAgentModel | undefined;
    pane_env: Record<string, string> | undefined;
    pane_env_default_enabled: boolean;
    claude_env: AppConfigClaudeEnv | undefined;
    websocket_port: AppConfigBase["websocket_port"];
    viewer_shortcuts: Record<string, string> | undefined;
    viewer_sidebar_mode: ViewerSidebarMode | undefined;
    default_session_dir: string | undefined;
    mcp_servers: AppConfigMCPServerConfig[] | undefined;
    chat_overlay_percentage: number | undefined;
    task_scheduler: AppConfigTaskScheduler | undefined;
};

type WailsConfigInputKeyShape = {
    shell: true;
    prefix: true;
    keys: true;
    quake_mode: true;
    global_hotkey: true;
    worktree: true;
    agent_model: true;
    pane_env: true;
    pane_env_default_enabled: true;
    claude_env: true;
    websocket_port: true;
    viewer_shortcuts: true;
    viewer_sidebar_mode: true;
    default_session_dir: true;
    mcp_servers: true;
    chat_overlay_percentage: true;
    task_scheduler: true;
};

type _WailsConfigInputKeyGuard =
    AssertTrue<ExactKeysMatch<WailsConfigInputKeyShape, Record<keyof WailsConfigInput, true>>>;

const CONFIG_KEY_GUARDS = [
    true as _AppConfigMessageTemplateKeyGuard,
    true as _AppConfigTaskSchedulerKeyGuard,
    true as _WailsConfigInputKeyGuard,
] as const;
void CONFIG_KEY_GUARDS;

export interface ParsedConfigUpdatedEvent {
    config: AppConfig;
    version: number | null;
    updated_at_unix_milli: number | null;
}

// Keep backend JSON field names (snake_case) here and map to camelCase in UI state.
export interface ValidationRules {
    min_override_name_len: number;
    min_pre_exec_reset_delay: number;
    max_pre_exec_reset_delay: number;
    min_pre_exec_idle_timeout: number;
    max_pre_exec_idle_timeout: number;
    max_message_templates: number;
    max_template_name_len: number;
    max_template_message_len: number;
    min_single_task_runner_clear_delay: number;
    max_single_task_runner_clear_delay: number;
    min_chat_overlay_percentage: number;
    max_chat_overlay_percentage: number;
    default_chat_overlay_percentage: number;
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
