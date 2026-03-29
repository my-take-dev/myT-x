import {useEffect, useLayoutEffect, useReducer, useRef} from "react";
import {api} from "../api";
import {useEscapeClose} from "../hooks/useEscapeClose";
import {logFrontendEventSafe} from "../utils/logFrontendEventSafe";
import {MIN_OVERRIDE_NAME_LEN_FALLBACK} from "./settings/constants";
import {INITIAL_FORM, formReducer} from "./settings/settingsReducer";
import {useSettingsI18n} from "./settings/settingsI18n";
import {SettingsTabs} from "./settings/SettingsTabs";
import {useSettingsSave} from "./settings/useSettingsSave";

interface SettingsModalProps {
    open: boolean;
    onClose: () => void;
}

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

export function SettingsModal({open, onClose}: SettingsModalProps) {
    const {t} = useSettingsI18n();
    const [s, dispatch] = useReducer(formReducer, INITIAL_FORM);
    const panelRef = useRef<HTMLDivElement | null>(null);
    const previouslyFocusedRef = useRef<HTMLElement | null>(null);
    const prevOpenForResetRef = useRef(false);
    const prevOpenForFocusRef = useRef(false);

    const {handleSave} = useSettingsSave(s, dispatch, onClose);

    useEscapeClose(open && !s.saving, onClose);

    useLayoutEffect(() => {
        if (open && !prevOpenForResetRef.current) {
            dispatch({type: "RESET_FOR_LOAD"});
        }
        prevOpenForResetRef.current = open;
    }, [open]);

    // Load config when modal opens.
    useEffect(() => {
        if (!open) return;
        let cancelled = false;

        Promise.all([
            api.GetConfig(),
            api.GetAllowedShells(),
            api.GetValidationRules().catch((err: unknown) => {
                console.warn("[settings] GetValidationRules failed (non-fatal)", err);
                logFrontendEventSafe("warn", `GetValidationRules failed: ${err instanceof Error ? err.message : String(err)}`, "SettingsModal");
                return null;
            }),
        ])
            .then(([cfg, shells, rules]) => {
                if (cancelled) return;

                dispatch({type: "LOAD_CONFIG", config: cfg, shells});
                const minOverrideNameLen =
                    typeof rules?.min_override_name_len === "number" &&
                    Number.isFinite(rules.min_override_name_len)
                        ? Math.max(1, Math.trunc(rules.min_override_name_len))
                        : MIN_OVERRIDE_NAME_LEN_FALLBACK;
                dispatch({type: "SET_FIELD", field: "minOverrideNameLen", value: minOverrideNameLen});
            })
            .catch((reason) => {
                if (cancelled) return;
                dispatch({type: "SET_FIELD", field: "error", value: String(reason)});
                dispatch({type: "SET_FIELD", field: "loadFailed", value: true});
            })
            .finally(() => {
                if (!cancelled) {
                    dispatch({type: "SET_FIELD", field: "loading", value: false});
                }
            });

        return () => {
            cancelled = true;
        };
    }, [open]);

    // Focus management: trap focus inside modal.
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
                    <h2 id="settings-modal-title">{t("settings.modal.title", "設定(config.yaml)", "Settings (config.yaml)")}</h2>
                </div>

                {s.loading ? (
                    <div className="modal-loading">{t("settings.modal.loading", "設定を読み込み中...", "Loading settings...")}</div>
                ) : (
                    <SettingsTabs s={s} dispatch={dispatch}/>
                )}

                <div className="settings-footer">
                    {s.error ? (
                        <span className="settings-error">{s.error}</span>
                    ) : (
                        <span className="settings-restart-note">
                            {t(
                                "settings.modal.restartNote",
                                "※ 一部設定はアプリ再起動後に反映されます",
                                "Some settings are applied after app restart.",
                            )}
                        </span>
                    )}
                    <button className="modal-btn" onClick={onClose} disabled={s.saving}>
                        {t("common.cancel", "キャンセル", "Cancel")}
                    </button>
                    <button
                        className="modal-btn primary"
                        onClick={handleSave}
                        disabled={s.loading || s.saving || s.loadFailed}
                    >
                        {s.saving
                            ? t("common.saving", "保存中...", "Saving...")
                            : t("common.save", "保存", "Save")}
                    </button>
                </div>
            </div>
        </div>
    );
}
