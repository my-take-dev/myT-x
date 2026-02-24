import type {FormDispatch, FormState} from "./types";
import {ShortcutInput} from "./ShortcutInput";
import {VIEWER_SHORTCUTS} from "../viewer/viewerShortcutDefinitions";

interface ViewerShortcutSettingsProps {
    s: FormState;
    dispatch: FormDispatch;
}

export function ViewerShortcutSettings({s, dispatch}: ViewerShortcutSettingsProps) {
    return (
        <div className="settings-section">
            <div className="settings-section-title">ビューアーショートカット</div>
            <span className="settings-desc" style={{marginBottom: 8, display: "block"}}>
                右サイドバーのビューを開閉するショートカットキー
            </span>

            {VIEWER_SHORTCUTS.map((viewerShortcut) => {
                const errorKey = `viewer_shortcut_${viewerShortcut.viewId}`;
                const inputID = `viewer-shortcut-${viewerShortcut.viewId}`;
                return (
                    <div className="form-group" key={viewerShortcut.viewId}>
                        <label className="shortcut-label" htmlFor={inputID}>{viewerShortcut.label}</label>
                        <ShortcutInput
                            id={inputID}
                            value={s.viewerShortcuts[viewerShortcut.viewId] || ""}
                            onChange={(value) =>
                                dispatch({type: "UPDATE_VIEWER_SHORTCUT", viewId: viewerShortcut.viewId, value})
                            }
                            placeholder={viewerShortcut.defaultShortcut}
                            ariaLabel={`${viewerShortcut.label} viewer shortcut`}
                        />
                        {s.validationErrors[errorKey] && (
                            <span className="settings-field-error">{s.validationErrors[errorKey]}</span>
                        )}
                    </div>
                );
            })}
        </div>
    );
}
