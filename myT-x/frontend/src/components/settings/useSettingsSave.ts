import {useCallback} from "react";
import {config} from "../../../wailsjs/go/models";
import {api} from "../../api";
import {useNotificationStore} from "../../stores/notificationStore";
import {serializeViewerSidebarMode} from "../../utils/viewerSidebarMode";
import {normalizeViewerShortcutConfig} from "../viewer/viewerShortcutDefinitions";
import {normalizeShortcut} from "../viewer/viewerShortcutUtils";
import {EFFORT_LEVEL_KEY} from "./constants";
import {useSettingsI18n} from "./settingsI18n";
import {
    validateAgentModelSettings,
    validateAutoStartSettings,
    validateClaudeEnvSettings,
    validateDefaultSessionDir,
    validateGlobalHotkey,
    validatePaneEnvSettings,
    validatePrefixShortcut,
    validateViewerShortcuts,
    validateWorktreeCopyPathSettings,
} from "./settingsValidation";
import type {FormDispatch, FormState, SettingsCategory} from "./types";
import type {AppConfigMessageTemplate, AppConfigTaskScheduler, WailsConfigInput} from "../../types/tmux";

type StrictMessageTemplatePayload = {[K in keyof config.MessageTemplate]-?: config.MessageTemplate[K]};
type StrictTaskSchedulerPayload = Omit<AppConfigTaskScheduler, "message_templates"> & {
    message_templates: StrictMessageTemplatePayload[];
};

function buildMessageTemplatePayload(template: AppConfigMessageTemplate): StrictMessageTemplatePayload {
    return {
        name: template.name,
        message: template.message,
    };
}

function buildTaskSchedulerPayload(taskScheduler: AppConfigTaskScheduler): StrictTaskSchedulerPayload {
    return {
        pre_exec_reset_delay_s: taskScheduler.pre_exec_reset_delay_s,
        pre_exec_idle_timeout_s: taskScheduler.pre_exec_idle_timeout_s,
        pre_exec_target_mode: taskScheduler.pre_exec_target_mode,
        message_templates: taskScheduler.message_templates?.map(buildMessageTemplatePayload) ?? [],
    };
}

export function buildSettingsSavePayload(s: FormState): WailsConfigInput {
    const filteredOverrides = s.overrides.filter(
        (ov) => ov.name.trim() || ov.model.trim(),
    );

    const hasAgent = s.agentFrom.trim() || s.agentTo.trim() || filteredOverrides.length > 0;

    const paneEnv: Record<string, string> = {};
    const effortLevel = s.effortLevel.trim();
    if (effortLevel) {
        paneEnv[EFFORT_LEVEL_KEY] = effortLevel;
    }
    for (const entry of s.paneEnvEntries) {
        const k = entry.key.trim();
        const v = entry.value.trim();
        if (k && v && k !== EFFORT_LEVEL_KEY) {
            paneEnv[k] = v;
        }
    }

    const claudeEnvVars: Record<string, string> = {};
    for (const entry of s.claudeEnvEntries) {
        const k = entry.key.trim();
        const v = entry.value.trim();
        if (k && v) {
            claudeEnvVars[k] = v;
        }
    }
    const hasClaudeEnv = Object.keys(claudeEnvVars).length > 0 || s.claudeEnvDefaultEnabled;

    return {
        shell: s.shell,
        prefix: s.prefix,
        keys: s.keys,
        quake_mode: s.quakeMode,
        global_hotkey: s.globalHotkey,
        auto_start: s.autoStart
            .map((entry) => ({
                name: entry.name.trim(),
                command: entry.command.trim(),
                args: (entry.args ?? "").trim(),
            }))
            .filter((entry) => entry.command),
        websocket_port: s.websocketPort,
        worktree: {
            enabled: s.wtEnabled,
            force_cleanup: s.wtForceCleanup,
            setup_scripts: s.wtSetupScripts.filter((v) => v.trim()),
            setup_script_timeout_seconds: s.wtSetupScriptTimeoutSeconds,
            copy_files: s.wtCopyFiles.filter((v) => v.trim()),
            copy_dirs: s.wtCopyDirs.filter((v) => v.trim()),
        },
        // SaveConfig is full-overwrite, so explicit empty MCP collections must
        // be preserved after the config load establishes that the user really
        // has zero MCP servers.
        mcp_servers: s.mcpServersLoaded
            ? s.mcpServers.map((server) => ({
                id: server.id,
                name: server.name,
                description: server.description,
                kind: server.kind,
                command: server.command,
                args: server.args ? [...server.args] : undefined,
                env: server.env ? {...server.env} : undefined,
                enabled: server.enabled,
                usage_sample: server.usage_sample,
                config_params: server.config_params?.map((param) => ({
                    key: param.key,
                    label: param.label,
                    default_value: param.default_value,
                    description: param.description,
                })),
            }))
            : undefined,
        agent_model: hasAgent
            ? {
                from: s.agentFrom.trim(),
                to: s.agentTo.trim(),
                overrides: filteredOverrides.map((ov) => ({
                    name: ov.name.trim(),
                    model: ov.model.trim(),
                })),
            }
            : undefined,
        pane_env: Object.keys(paneEnv).length > 0 ? paneEnv : undefined,
        pane_env_default_enabled: s.paneEnvDefaultEnabled,
        claude_env: hasClaudeEnv
            ? {
                default_enabled: s.claudeEnvDefaultEnabled,
                vars: Object.keys(claudeEnvVars).length > 0 ? claudeEnvVars : undefined,
            }
            : undefined,
        task_scheduler: s.taskScheduler
            ? buildTaskSchedulerPayload(s.taskScheduler)
            : undefined,
        viewer_sidebar_mode: serializeViewerSidebarMode(s.viewerSidebarMode),
        chat_overlay_percentage: s.chatOverlayPercentage,
        default_session_dir: s.defaultSessionDir.trim() || undefined,
        viewer_shortcuts: (() => {
            const filtered = Object.fromEntries(
                Object.entries(normalizeViewerShortcutConfig(s.viewerShortcuts))
                    .map(([key, value]) => [key, normalizeShortcut(value.trim())] as const)
                    .filter(([, value]) => value !== ""),
            );
            return Object.keys(filtered).length > 0 ? filtered : undefined;
        })(),
    };
}

export function selectSettingsValidationCategory(errors: Record<string, string>): SettingsCategory | null {
    const keys = Object.keys(errors);
    if (keys.some((k) => k.startsWith("agent") || k.startsWith("override"))) {
        return "agent-model";
    }
    if (keys.some((k) => k.startsWith("claude_env"))) {
        return "claude-env";
    }
    if (keys.some((k) => k.startsWith("pane_env"))) {
        return "pane-env";
    }
    if (keys.some((k) => k.startsWith("wt_copy_"))) {
        return "worktree";
    }
    if (keys.some((k) => k === "default_session_dir" || k === "prefix" || k === "global_hotkey")) {
        return "general";
    }
    if (keys.some((k) => k.startsWith("auto_start"))) {
        return "auto-start";
    }
    if (keys.some((k) => k.startsWith("viewer_shortcut_"))) {
        return "keybinds";
    }
    return null;
}

export function useSettingsSave(
    s: FormState,
    dispatch: FormDispatch,
    onClose: () => void,
): { readonly handleSave: () => Promise<void> } {
    const {t} = useSettingsI18n();

    const handleSave = useCallback(async () => {
        if (s.loadFailed) {
            dispatch({
                type: "SET_FIELD",
                field: "error",
                value: t(
                    "settings.modal.error.configLoadFailedCannotSave",
                    "設定の読み込みに失敗しているため保存できません。",
                    "Cannot save because config loading failed.",
                ),
            });
            return;
        }
        const errors = {
            ...validateAgentModelSettings(s.agentFrom, s.agentTo, s.overrides, s.minOverrideNameLen),
            ...validateAutoStartSettings(s.autoStart),
            ...validateClaudeEnvSettings(s.claudeEnvEntries),
            ...validatePaneEnvSettings(s.paneEnvEntries, s.effortLevel),
            ...validateWorktreeCopyPathSettings(s.wtCopyFiles, s.wtCopyDirs),
            ...validatePrefixShortcut(s.prefix),
            ...validateGlobalHotkey(s.globalHotkey, s.quakeMode),
            ...validateViewerShortcuts(s.viewerShortcuts, s.quakeMode ? s.globalHotkey : ""),
            ...validateDefaultSessionDir(s.defaultSessionDir),
        };
        if (Object.keys(errors).length > 0) {
            dispatch({type: "SET_FIELD", field: "validationErrors", value: errors});
            const nextCategory = selectSettingsValidationCategory(errors);
            if (nextCategory !== null) {
                dispatch({type: "SET_FIELD", field: "activeCategory", value: nextCategory});
            }
            return;
        }
        dispatch({type: "START_SAVE"});

        // NOTE: SaveConfig performs full overwrite (not merge), so the payload
        // must round-trip fields that are not editable in this modal.
        // Go-side config.Save marshals the entire Config struct and writes it
        // atomically to disk.
        const payload = buildSettingsSavePayload(s);

        try {
            const cfg = config.Config.createFrom(payload);
            await api.SaveConfig(cfg);
            const addNotification = useNotificationStore.getState().addNotification;
            addNotification(
                t("settings.modal.notification.saved", "設定を保存しました", "Settings saved."),
                "info",
            );
            onClose();
        } catch (err: unknown) {
            dispatch({type: "SET_FIELD", field: "error", value: String(err)});
        } finally {
            dispatch({type: "SET_FIELD", field: "saving", value: false});
        }
    }, [s, dispatch, onClose, t]);

    return {handleSave};
}
