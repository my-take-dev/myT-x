import {useCallback} from "react";
import {config} from "../../../wailsjs/go/models";
import {api} from "../../api";
import {useNotificationStore} from "../../stores/notificationStore";
import {serializeViewerSidebarMode} from "../../utils/viewerSidebarMode";
import {normalizeShortcut} from "../viewer/viewerShortcutUtils";
import {EFFORT_LEVEL_KEY} from "./constants";
import {useSettingsI18n} from "./settingsI18n";
import {
    validateAgentModelSettings,
    validateClaudeEnvSettings,
    validateDefaultSessionDir,
    validatePaneEnvSettings,
    validateViewerShortcuts,
    validateWorktreeCopyPathSettings,
} from "./settingsValidation";
import type {FormDispatch, FormState} from "./types";
import type {WailsConfigInput} from "../../types/tmux";

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
            ...validateClaudeEnvSettings(s.claudeEnvEntries),
            ...validatePaneEnvSettings(s.paneEnvEntries, s.effortLevel),
            ...validateWorktreeCopyPathSettings(s.wtCopyFiles, s.wtCopyDirs),
            ...validateViewerShortcuts(s.viewerShortcuts, s.quakeMode ? s.globalHotkey : ""),
            ...validateDefaultSessionDir(s.defaultSessionDir),
        };
        if (Object.keys(errors).length > 0) {
            dispatch({type: "SET_FIELD", field: "validationErrors", value: errors});
            if (Object.keys(errors).some((k) => k.startsWith("agent") || k.startsWith("override"))) {
                dispatch({type: "SET_FIELD", field: "activeCategory", value: "agent-model"});
            } else if (Object.keys(errors).some((k) => k.startsWith("claude_env"))) {
                dispatch({type: "SET_FIELD", field: "activeCategory", value: "claude-env"});
            } else if (Object.keys(errors).some((k) => k.startsWith("pane_env"))) {
                dispatch({type: "SET_FIELD", field: "activeCategory", value: "pane-env"});
            } else if (Object.keys(errors).some((k) => k.startsWith("wt_copy_"))) {
                dispatch({type: "SET_FIELD", field: "activeCategory", value: "worktree"});
            } else if (Object.keys(errors).some((k) => k === "default_session_dir")) {
                dispatch({type: "SET_FIELD", field: "activeCategory", value: "general"});
            } else if (Object.keys(errors).some((k) => k.startsWith("viewer_shortcut_"))) {
                dispatch({type: "SET_FIELD", field: "activeCategory", value: "keybinds"});
            }
            return;
        }
        dispatch({type: "START_SAVE"});

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
            if (k && v && k !== EFFORT_LEVEL_KEY) paneEnv[k] = v;
        }

        const claudeEnvVars: Record<string, string> = {};
        for (const entry of s.claudeEnvEntries) {
            const k = entry.key.trim();
            const v = entry.value.trim();
            if (k && v) claudeEnvVars[k] = v;
        }
        const hasClaudeEnv = Object.keys(claudeEnvVars).length > 0 || s.claudeEnvDefaultEnabled;

        // NOTE: SaveConfig performs full overwrite (not merge), so omitting
        // claude_env / pane_env when empty correctly clears any previously saved
        // configuration. Go-side config.Save marshals the entire Config struct
        // and writes it atomically to disk.
        const payload: WailsConfigInput = {
            shell: s.shell,
            prefix: s.prefix,
            keys: s.keys,
            quake_mode: s.quakeMode,
            global_hotkey: s.globalHotkey,
            worktree: {
                enabled: s.wtEnabled,
                force_cleanup: s.wtForceCleanup,
                setup_scripts: s.wtSetupScripts.filter((v) => v.trim()),
                copy_files: s.wtCopyFiles.filter((v) => v.trim()),
                copy_dirs: s.wtCopyDirs.filter((v) => v.trim()),
            },
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
            viewer_sidebar_mode: serializeViewerSidebarMode(s.viewerSidebarMode),
            chat_overlay_percentage: s.chatOverlayPercentage,
            default_session_dir: s.defaultSessionDir.trim() || undefined,
            viewer_shortcuts: (() => {
                const filtered = Object.fromEntries(
                    Object.entries(s.viewerShortcuts)
                        .map(([key, value]) => [key, normalizeShortcut(value.trim())] as const)
                        .filter(([, value]) => value !== ""),
                );
                return Object.keys(filtered).length > 0 ? filtered : undefined;
            })(),
        };

        try {
            const cfg = config.Config.createFrom(payload);
            await api.SaveConfig(cfg);
            const addNotification = useNotificationStore.getState().addNotification;
            addNotification(
                t("settings.modal.notification.saved", "設定を保存しました", "Settings saved."),
                "info",
            );
            onClose();
        } catch (err) {
            dispatch({type: "SET_FIELD", field: "error", value: String(err)});
        } finally {
            dispatch({type: "SET_FIELD", field: "saving", value: false});
        }
    }, [s, dispatch, onClose, t]);

    return {handleSave};
}
