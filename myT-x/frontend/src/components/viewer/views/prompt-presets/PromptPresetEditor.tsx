import {useI18n} from "../../../../i18n";
import type {PromptPresetDraft} from "./types";

interface PromptPresetEditorProps {
    draft: PromptPresetDraft;
    saving: boolean;
    canSave: boolean;
    activeSession: string | null;
    storageLocked: boolean;
    onChange: (draft: PromptPresetDraft) => void;
    onBack: () => void;
    onSave: () => void;
}

export function PromptPresetEditor({
    draft,
    saving,
    canSave,
    activeSession,
    storageLocked,
    onChange,
    onBack,
    onSave,
}: PromptPresetEditorProps) {
    const {t} = useI18n();

    return (
        <div className="prompt-presets-editor">
            <button type="button" className="prompt-presets-back-btn" onClick={onBack}>
                &larr; {t("viewer.promptPresets.editor.back", "Back")}
            </button>

            <div className="prompt-presets-editor-body">
                <div className="form-group">
                    <label className="form-label">
                        {t("viewer.promptPresets.editor.name", "Preset name")}
                    </label>
                    <input
                        className="form-input"
                        type="text"
                        value={draft.name}
                        onChange={(event) => onChange({...draft, name: event.target.value})}
                        placeholder={t("viewer.promptPresets.editor.namePlaceholder", "TDD implementation")}
                    />
                </div>

                <div className="form-group">
                    <label className="form-label">
                        {t("viewer.promptPresets.editor.storageLocation", "Storage")}
                    </label>
                    <div className="prompt-presets-storage-selector">
                        <label className={`prompt-presets-storage-option${storageLocked ? " disabled" : ""}`}>
                            <input
                                type="radio"
                                name="promptPresetStorageLocation"
                                value="global"
                                checked={draft.storageLocation === "global"}
                                disabled={storageLocked}
                                onChange={() => onChange({
                                    ...draft,
                                    storageLocation: "global",
                                    projectSessionName: null,
                                })}
                            />
                            {t("viewer.promptPresets.editor.storageGlobal", "Global")}
                        </label>
                        <label
                            className={`prompt-presets-storage-option${storageLocked || activeSession === null ? " disabled" : ""}`}
                        >
                            <input
                                type="radio"
                                name="promptPresetStorageLocation"
                                value="project"
                                checked={draft.storageLocation === "project"}
                                disabled={storageLocked || activeSession === null}
                                onChange={() => onChange({
                                    ...draft,
                                    storageLocation: "project",
                                    projectSessionName: activeSession,
                                })}
                            />
                            {t("viewer.promptPresets.editor.storageProject", "Project (.myT-x)")}
                        </label>
                    </div>
                    {storageLocked && (
                        <span className="prompt-presets-editor-note">
                            {t(
                                "viewer.promptPresets.editor.storageLocked",
                                "Storage location cannot be changed for an existing preset.",
                            )}
                        </span>
                    )}
                </div>

                <div className="form-group">
                    <label className="form-label">
                        {t("viewer.promptPresets.editor.body", "Prompt")}
                    </label>
                    <textarea
                        className="form-input prompt-presets-editor-textarea"
                        rows={10}
                        value={draft.body}
                        onChange={(event) => onChange({...draft, body: event.target.value})}
                        placeholder={t(
                            "viewer.promptPresets.editor.bodyPlaceholder",
                            "Implement this with tests first and explain the tradeoffs in English.",
                        )}
                    />
                </div>
            </div>

            <div className="prompt-presets-editor-footer">
                <button
                    type="button"
                    className="prompt-presets-primary-btn"
                    disabled={!canSave || saving}
                    onClick={onSave}
                >
                    {saving
                        ? t("viewer.promptPresets.editor.saving", "Saving...")
                        : t("viewer.promptPresets.editor.save", "Save preset")}
                </button>
            </div>
        </div>
    );
}
