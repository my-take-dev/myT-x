import { useEffect, useLayoutEffect, useReducer, useRef } from "react";
import { config } from "../../wailsjs/go/models";
import { api } from "../api";
import { useEscapeClose } from "../hooks/useEscapeClose";
import { useNotificationStore } from "../stores/notificationStore";
import { GeneralSettings } from "./settings/GeneralSettings";
import { KeybindSettings } from "./settings/KeybindSettings";
import { WorktreeSettings } from "./settings/WorktreeSettings";
import { AgentModelSettings } from "./settings/AgentModelSettings";
import { PaneEnvSettings } from "./settings/PaneEnvSettings";
import { EFFORT_LEVEL_KEY, MIN_OVERRIDE_NAME_LEN_FALLBACK } from "./settings/constants";
import type { SettingsCategory } from "./settings/types";
import { INITIAL_FORM, formReducer } from "./settings/settingsReducer";
import { validateAgentModelSettings, validatePaneEnvSettings } from "./settings/settingsValidation";
import type { WailsConfigInput } from "../types/tmux";

interface SettingsModalProps {
  open: boolean;
  onClose: () => void;
}

// Settings category definitions.
const SETTINGS_CATEGORIES: { id: SettingsCategory; label: string }[] = [
  { id: "general", label: "基本設定" },
  { id: "keybinds", label: "キーバインド" },
  { id: "worktree", label: "Worktree" },
  { id: "agent-model", label: "Agent Model" },
  { id: "pane-env", label: "環境変数" },
];

function getFocusableElements(root: HTMLElement | null): HTMLElement[] {
  if (!root) {
    return [];
  }
  return Array.from(
    root.querySelectorAll<HTMLElement>(
      'button:not([disabled]), input:not([disabled]), textarea:not([disabled]), select:not([disabled]), a[href], [tabindex]:not([tabindex="-1"])',
    ),
  ).filter((el) => !el.hasAttribute("disabled") && el.getAttribute("aria-hidden") !== "true");
}

// Settings modal root component.

export function SettingsModal({ open, onClose }: SettingsModalProps) {
  const [s, dispatch] = useReducer(formReducer, INITIAL_FORM);
  const panelRef = useRef<HTMLDivElement | null>(null);
  const previouslyFocusedRef = useRef<HTMLElement | null>(null);
  const prevOpenForResetRef = useRef(false);
  const prevOpenForFocusRef = useRef(false);

  useEscapeClose(open && !s.saving, onClose);

  useLayoutEffect(() => {
    if (open && !prevOpenForResetRef.current) {
      dispatch({ type: "RESET_FOR_LOAD" });
    }
    prevOpenForResetRef.current = open;
  }, [open]);

  // モーダル開時にconfig読み込み
  useEffect(() => {
    if (!open) return;
    let cancelled = false;

    Promise.all([
      api.GetConfig(),
      api.GetAllowedShells(),
      api.GetValidationRules().catch(() => null),
    ])
      .then(([cfg, shells, rules]) => {
        if (cancelled) return;

        dispatch({ type: "LOAD_CONFIG", config: cfg, shells });
        const minOverrideNameLen =
          typeof rules?.min_override_name_len === "number" &&
          Number.isFinite(rules.min_override_name_len)
            ? Math.max(1, Math.trunc(rules.min_override_name_len))
            : MIN_OVERRIDE_NAME_LEN_FALLBACK;
        dispatch({ type: "SET_FIELD", field: "minOverrideNameLen", value: minOverrideNameLen });
      })
      .catch((reason) => {
        if (cancelled) return;
        dispatch({ type: "SET_FIELD", field: "error", value: String(reason) });
        dispatch({ type: "SET_FIELD", field: "loadFailed", value: true });
      })
      .finally(() => {
        if (!cancelled) {
          dispatch({ type: "SET_FIELD", field: "loading", value: false });
        }
      });

    return () => {
      cancelled = true;
    };
  }, [open]);

  useEffect(() => {
    let raf = 0;
    if (open) {
      previouslyFocusedRef.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
      raf = requestAnimationFrame(() => {
        const focusables = getFocusableElements(panelRef.current);
        if (focusables.length > 0) {
          focusables[0]?.focus();
          return;
        }
        panelRef.current?.focus();
      });
    } else if (prevOpenForFocusRef.current) {
      previouslyFocusedRef.current?.focus();
    }
    prevOpenForFocusRef.current = open;

    return () => {
      if (raf !== 0) {
        cancelAnimationFrame(raf);
      }
    };
  }, [open]);

  const handlePanelKeyDown = (event: React.KeyboardEvent<HTMLDivElement>) => {
    if (event.key !== "Tab") {
      return;
    }
    const focusables = getFocusableElements(panelRef.current);
    if (focusables.length === 0) {
      event.preventDefault();
      return;
    }

    const first = focusables[0];
    const last = focusables[focusables.length - 1];
    const active = document.activeElement;

    if (event.shiftKey) {
      if (active === first || active === panelRef.current) {
        event.preventDefault();
        last?.focus();
      }
      return;
    }
    if (active === last) {
      event.preventDefault();
      first?.focus();
    }
  };

  const handleCategoryKeyDown = (event: React.KeyboardEvent<HTMLButtonElement>, category: SettingsCategory) => {
    const currentIndex = SETTINGS_CATEGORIES.findIndex((item) => item.id === category);
    if (currentIndex < 0) {
      return;
    }

    let nextIndex = currentIndex;
    switch (event.key) {
      case "ArrowRight":
      case "ArrowDown":
        nextIndex = (currentIndex + 1) % SETTINGS_CATEGORIES.length;
        break;
      case "ArrowLeft":
      case "ArrowUp":
        nextIndex = (currentIndex - 1 + SETTINGS_CATEGORIES.length) % SETTINGS_CATEGORIES.length;
        break;
      case "Home":
        nextIndex = 0;
        break;
      case "End":
        nextIndex = SETTINGS_CATEGORIES.length - 1;
        break;
      default:
        return;
    }

    event.preventDefault();
    const nextCategory = SETTINGS_CATEGORIES[nextIndex]!.id;
    dispatch({ type: "SET_FIELD", field: "activeCategory", value: nextCategory });
    const nextTab = document.getElementById(`settings-tab-${nextCategory}`);
    if (nextTab instanceof HTMLElement) {
      nextTab.focus();
    }
  };

  const categoryPanels: Record<SettingsCategory, () => JSX.Element> = {
    general: () => <GeneralSettings s={s} dispatch={dispatch} />,
    keybinds: () => <KeybindSettings s={s} dispatch={dispatch} />,
    worktree: () => <WorktreeSettings s={s} dispatch={dispatch} />,
    "agent-model": () => <AgentModelSettings s={s} dispatch={dispatch} />,
    "pane-env": () => <PaneEnvSettings s={s} dispatch={dispatch} />,
  };

  const renderCategoryPanel = (category: SettingsCategory) => {
    return categoryPanels[category]();
  };

  const handleSave = async () => {
    if (s.loadFailed) {
      dispatch({ type: "SET_FIELD", field: "error", value: "Cannot save because config loading failed." });
      return;
    }
    const errors = {
      ...validateAgentModelSettings(s.agentFrom, s.agentTo, s.overrides, s.minOverrideNameLen),
      ...validatePaneEnvSettings(s.paneEnvEntries, s.effortLevel),
    };
    if (Object.keys(errors).length > 0) {
      dispatch({ type: "SET_FIELD", field: "validationErrors", value: errors });
      if (Object.keys(errors).some((k) => k.startsWith("agent") || k.startsWith("override"))) {
        dispatch({ type: "SET_FIELD", field: "activeCategory", value: "agent-model" });
      } else if (Object.keys(errors).some((k) => k.startsWith("pane_env"))) {
        dispatch({ type: "SET_FIELD", field: "activeCategory", value: "pane-env" });
      }
      return;
    }
    dispatch({ type: "START_SAVE" });

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
    };

    try {
      const cfg = config.Config.createFrom(payload);
      await api.SaveConfig(cfg);
      const addNotification = useNotificationStore.getState().addNotification;
      addNotification("設定を保存しました", "info");
      onClose();
    } catch (err) {
      dispatch({ type: "SET_FIELD", field: "error", value: String(err) });
    } finally {
      dispatch({ type: "SET_FIELD", field: "saving", value: false });
    }
  };

  if (!open) return null;

  return (
    <div
      className="modal-overlay"
      role="presentation"
      onClick={(e) => e.target === e.currentTarget && !s.saving && onClose()}
    >
      <div
        ref={panelRef}
        className="modal-panel settings-panel"
        role="dialog"
        aria-modal="true"
        aria-labelledby="settings-modal-title"
        onKeyDown={handlePanelKeyDown}
        tabIndex={-1}
      >
        <div className="modal-header">
          <h2 id="settings-modal-title">設定(config.yaml)</h2>
        </div>

        {s.loading ? (
          <div className="modal-loading">設定を読み込み中...</div>
        ) : (
          <div className="settings-layout">
            <nav className="settings-sidebar" role="tablist" aria-label="Settings categories">
              {SETTINGS_CATEGORIES.map((cat) => {
                const tabID = `settings-tab-${cat.id}`;
                const panelID = `settings-panel-${cat.id}`;
                return (
                  <button
                    key={cat.id}
                    id={tabID}
                    role="tab"
                    aria-selected={s.activeCategory === cat.id}
                    aria-controls={panelID}
                    tabIndex={s.activeCategory === cat.id ? 0 : -1}
                    className={`settings-sidebar-item ${s.activeCategory === cat.id ? "active" : ""}`}
                    onClick={() => dispatch({ type: "SET_FIELD", field: "activeCategory", value: cat.id })}
                    onKeyDown={(event) => handleCategoryKeyDown(event, cat.id)}
                  >
                    {cat.label}
                  </button>
                );
              })}
            </nav>

            <div className="settings-body">
              {SETTINGS_CATEGORIES.map((cat) => {
                const isActive = s.activeCategory === cat.id;
                return (
                  <div
                    key={cat.id}
                    id={`settings-panel-${cat.id}`}
                    role="tabpanel"
                    aria-labelledby={`settings-tab-${cat.id}`}
                    hidden={!isActive}
                  >
                    {isActive ? renderCategoryPanel(cat.id) : null}
                  </div>
                );
              })}
            </div>
          </div>
        )}

        <div className="settings-footer">
          {s.error ? (
            <span className="settings-error">{s.error}</span>
          ) : (
            <span className="settings-restart-note">
              ※ 一部設定はアプリ再起動後に反映されます        </span>
          )}
          <button className="modal-btn" onClick={onClose} disabled={s.saving}>
            キャンセル
          </button>
          <button
            className="modal-btn primary"
            onClick={handleSave}
            disabled={s.loading || s.saving || s.loadFailed}
          >
            {s.saving ? "\u4fdd\u5b58\u4e2d..." : "\u4fdd\u5b58"}
          </button>
        </div>
      </div>
    </div>
  );
}
