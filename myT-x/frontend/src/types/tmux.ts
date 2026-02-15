import type { config as wailsConfig } from "../../wailsjs/go/models";

export type AppConfigWorktree = Pick<
  wailsConfig.WorktreeConfig,
  "enabled" | "force_cleanup" | "setup_scripts" | "copy_files"
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

export type AppConfig = AppConfigBase & {
  worktree: AppConfigWorktree;
  agent_model?: AppConfigAgentModel;
  pane_env?: Record<string, string>;
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
  created_at: string;
  is_idle: boolean;
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
