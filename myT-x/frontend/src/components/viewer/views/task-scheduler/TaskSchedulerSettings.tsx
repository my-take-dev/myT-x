import {useState, useCallback} from "react";
import {useI18n} from "../../../../i18n";
import {config} from "../../../../../wailsjs/go/models";
import type {TaskSchedulerSettings, MessageTemplate} from "./useTaskScheduler";
import {
    blockNonNumericKeys,
    preExecFieldBounds,
    readNumberInput,
    resolveInitialPreExecIdleTimeout,
    resolveInitialPreExecResetDelay,
    resolveInitialPreExecTargetMode,
} from "./taskSchedulerConfigForm";

interface TaskSchedulerSettingsProps {
    initialSettings: TaskSchedulerSettings | null;
    onSave: (settings: TaskSchedulerSettings) => Promise<boolean>;
    onBack: () => void;
}

interface TemplateDraft extends MessageTemplate {
    id: string;
}

interface TemplateEditState {
    mode: "add" | "edit";
    templateID: string | null;
    name: string;
    message: string;
}

let nextTemplateDraftCounter = 0;

function newTemplateDraftID(): string {
    if (typeof globalThis.crypto?.randomUUID === "function") {
        return globalThis.crypto.randomUUID();
    }

    nextTemplateDraftCounter += 1;
    return `task-scheduler-template-${nextTemplateDraftCounter}`;
}

function createTemplateDraft(template?: Partial<MessageTemplate>): TemplateDraft {
    return {
        id: newTemplateDraftID(),
        name: template?.name ?? "",
        message: template?.message ?? "",
    };
}

export function TaskSchedulerSettingsPanel({
    initialSettings,
    onSave,
    onBack,
}: TaskSchedulerSettingsProps) {
    const {language, t} = useI18n();
    const tr = (key: string, jaText: string, enText: string) =>
        t(key, language === "ja" ? jaText : enText);

    const [submitting, setSubmitting] = useState(false);

    // Pre-exec settings
    const [preExecResetDelay, setPreExecResetDelay] = useState(
        resolveInitialPreExecResetDelay(initialSettings?.pre_exec_reset_delay_s),
    );
    const [preExecIdleTimeout, setPreExecIdleTimeout] = useState(
        resolveInitialPreExecIdleTimeout(initialSettings?.pre_exec_idle_timeout_s),
    );
    const [preExecTargetMode, setPreExecTargetMode] = useState(
        resolveInitialPreExecTargetMode(initialSettings?.pre_exec_target_mode),
    );

    // Message templates
    const [templates, setTemplates] = useState<TemplateDraft[]>(
        () => (initialSettings?.message_templates ?? []).map((template) => createTemplateDraft(template)),
    );
    const [editing, setEditing] = useState<TemplateEditState | null>(null);

    const handleAddTemplate = useCallback(() => {
        setEditing({mode: "add", templateID: null, name: "", message: ""});
    }, []);

    const handleEditTemplate = useCallback((templateID: string) => {
        const tmpl = templates.find((template) => template.id === templateID);
        if (!tmpl) return;
        setEditing({mode: "edit", templateID, name: tmpl.name, message: tmpl.message});
    }, [templates]);

    const handleDeleteTemplate = useCallback((templateID: string) => {
        setTemplates((prev) => prev.filter((template) => template.id !== templateID));
    }, []);

    const handleSaveTemplate = useCallback(() => {
        if (!editing) return;
        const name = editing.name.trim();
        const message = editing.message.trim();
        if (name === "" || message === "") return;

        const entry = {name, message};
        if (editing.mode === "add") {
            setTemplates((prev) => [...prev, createTemplateDraft(entry)]);
        } else {
            setTemplates((prev) =>
                prev.map((template) => (
                    template.id === editing.templateID
                        ? {...template, ...entry}
                        : template
                )),
            );
        }
        setEditing(null);
    }, [editing]);

    const handleCancelTemplate = useCallback(() => {
        setEditing(null);
    }, []);

    const canSaveTemplate =
        editing !== null &&
        editing.name.trim() !== "" &&
        editing.message.trim() !== "";

    const handleSave = useCallback(async () => {
        if (submitting) return;
        setSubmitting(true);
        try {
            // Re-apply resolve as a safety net: ensures the submitted values are
            // within valid bounds even if local state was somehow corrupted.
            const settings = new config.TaskSchedulerConfig({
                pre_exec_reset_delay_s: resolveInitialPreExecResetDelay(preExecResetDelay),
                pre_exec_idle_timeout_s: resolveInitialPreExecIdleTimeout(preExecIdleTimeout),
                pre_exec_target_mode: preExecTargetMode,
                message_templates: templates.map(({name, message}) => ({name, message})),
            });
            const ok = await onSave(settings);
            if (ok) {
                onBack();
            }
        } finally {
            setSubmitting(false);
        }
    }, [submitting, onSave, onBack, preExecResetDelay, preExecIdleTimeout, preExecTargetMode, templates]);

    return (
        <div className="task-scheduler-form">
            <button type="button" className="task-scheduler-back-btn" onClick={onBack}>
                {"\u2190 "}{tr("viewer.taskScheduler.back", "\u623b\u308b", "Back")}
            </button>

            <h3 className="task-scheduler-config-title">
                {tr("viewer.taskScheduler.settingsTitle", "\u30b9\u30b1\u30b8\u30e5\u30fc\u30e9\u8a2d\u5b9a", "Scheduler Settings")}
            </h3>

            {/* Pre-exec timing settings */}
            <div className="task-scheduler-config-section">
                <div className="task-scheduler-config-group">
                    <label className="task-scheduler-config-field">
                        <span className="task-scheduler-config-label">
                            {tr(
                                "viewer.taskScheduler.preExecResetDelay",
                                "/new \u5f8c\u306e\u5f85\u6a5f\u6642\u9593 (\u79d2)",
                                "Wait after /new (seconds)",
                            )}
                        </span>
                        <input
                            type="text"
                            inputMode="numeric"
                            pattern="[0-9]*"
                            value={preExecResetDelay}
                            onChange={(event) => {
                                setPreExecResetDelay((prev) =>
                                    readNumberInput(
                                        event.target.value,
                                        prev,
                                        preExecFieldBounds.resetDelay.min,
                                        preExecFieldBounds.resetDelay.max,
                                    ),
                                );
                            }}
                            onKeyDown={blockNonNumericKeys}
                            className="task-scheduler-config-input"
                        />
                    </label>

                    <label className="task-scheduler-config-field">
                        <span className="task-scheduler-config-label">
                            {tr(
                                "viewer.taskScheduler.preExecIdleTimeout",
                                "\u30a2\u30a4\u30c9\u30eb\u5f85\u6a5f\u30bf\u30a4\u30e0\u30a2\u30a6\u30c8 (\u79d2)",
                                "Idle wait timeout (seconds)",
                            )}
                        </span>
                        <input
                            type="text"
                            inputMode="numeric"
                            pattern="[0-9]*"
                            value={preExecIdleTimeout}
                            onChange={(event) => {
                                setPreExecIdleTimeout((prev) =>
                                    readNumberInput(
                                        event.target.value,
                                        prev,
                                        preExecFieldBounds.idleTimeout.min,
                                        preExecFieldBounds.idleTimeout.max,
                                    ),
                                );
                            }}
                            onKeyDown={blockNonNumericKeys}
                            className="task-scheduler-config-input"
                        />
                    </label>
                </div>

                <div className="task-scheduler-config-group">
                    <div className="task-scheduler-config-label">
                        {tr(
                            "viewer.taskScheduler.preExecTarget",
                            "\u5bfe\u8c61\u30da\u30a4\u30f3",
                            "Target panes",
                        )}
                    </div>
                    <label className="task-scheduler-radio-label">
                        <input
                            type="radio"
                            name="task-scheduler-settings-target"
                            checked={preExecTargetMode === "task_panes"}
                            onChange={() => setPreExecTargetMode("task_panes")}
                        />
                        <span>
                            {tr(
                                "viewer.taskScheduler.preExecTaskPanes",
                                "\u30bf\u30b9\u30af\u3067\u4f7f\u7528\u3059\u308b\u30da\u30a4\u30f3\u306e\u307f",
                                "Only panes used by tasks",
                            )}
                        </span>
                    </label>
                    <label className="task-scheduler-radio-label">
                        <input
                            type="radio"
                            name="task-scheduler-settings-target"
                            checked={preExecTargetMode === "all_panes"}
                            onChange={() => setPreExecTargetMode("all_panes")}
                        />
                        <span>
                            {tr(
                                "viewer.taskScheduler.preExecAllPanes",
                                "\u30bb\u30c3\u30b7\u30e7\u30f3\u5185\u306e\u5168\u30da\u30a4\u30f3",
                                "All panes in the session",
                            )}
                        </span>
                    </label>
                </div>
            </div>

            {/* Message templates */}
            <h3 className="task-scheduler-config-title">
                {tr("viewer.taskScheduler.messageTemplates", "\u30e1\u30c3\u30bb\u30fc\u30b8\u30c6\u30f3\u30d7\u30ec\u30fc\u30c8", "Message Templates")}
            </h3>

            <div className="task-scheduler-template-list">
                {templates.length === 0 && !editing && (
                    <div className="task-scheduler-template-empty">
                        {tr("viewer.taskScheduler.noTemplates", "\u30c6\u30f3\u30d7\u30ec\u30fc\u30c8\u306a\u3057", "No templates")}
                    </div>
                )}

                {templates.map((tmpl) => (
                    <div key={tmpl.id} className="task-scheduler-template-item">
                        <div className="task-scheduler-template-item-header">
                            <span className="task-scheduler-template-item-name">{tmpl.name}</span>
                            <div className="task-scheduler-card-actions">
                                <button
                                    type="button"
                                    className="task-scheduler-edit-card-btn"
                                    onClick={() => handleEditTemplate(tmpl.id)}
                                    disabled={editing !== null}
                                >
                                    {tr("viewer.taskScheduler.edit", "\u7de8\u96c6", "Edit")}
                                </button>
                                <button
                                    type="button"
                                    className="task-scheduler-delete-card-btn"
                                    onClick={() => handleDeleteTemplate(tmpl.id)}
                                    disabled={editing !== null}
                                >
                                    {tr("viewer.taskScheduler.remove", "\u524a\u9664", "Remove")}
                                </button>
                            </div>
                        </div>
                        <div className="task-scheduler-template-item-message">{tmpl.message}</div>
                    </div>
                ))}

                {editing && (
                    <div className="task-scheduler-template-form">
                        <input
                            className="form-input"
                            type="text"
                            value={editing.name}
                            onChange={(e) =>
                                setEditing((prev) => prev && {...prev, name: e.target.value})
                            }
                            placeholder={tr("viewer.taskScheduler.templateName", "\u30c6\u30f3\u30d7\u30ec\u30fc\u30c8\u540d", "Template name")}
                        />
                        <textarea
                            className="task-scheduler-textarea"
                            value={editing.message}
                            onChange={(e) =>
                                setEditing((prev) => prev && {...prev, message: e.target.value})
                            }
                            placeholder={tr("viewer.taskScheduler.templateMessage", "\u30e1\u30c3\u30bb\u30fc\u30b8\u672c\u6587", "Message body")}
                        />
                        <div className="task-scheduler-template-form-actions">
                            <button
                                type="button"
                                className="task-scheduler-template-save-btn"
                                onClick={handleSaveTemplate}
                                disabled={!canSaveTemplate}
                            >
                                {editing.mode === "add"
                                    ? tr("viewer.taskScheduler.add", "\u8ffd\u52a0", "Add")
                                    : tr("viewer.taskScheduler.update", "\u66f4\u65b0", "Update")}
                            </button>
                            <button
                                type="button"
                                className="task-scheduler-template-cancel-btn"
                                onClick={handleCancelTemplate}
                            >
                                {tr("viewer.taskScheduler.cancel", "\u30ad\u30e3\u30f3\u30bb\u30eb", "Cancel")}
                            </button>
                        </div>
                    </div>
                )}

                {!editing && (
                    <button
                        type="button"
                        className="task-scheduler-template-add-btn"
                        onClick={handleAddTemplate}
                    >
                        + {tr("viewer.taskScheduler.addTemplate", "\u30c6\u30f3\u30d7\u30ec\u30fc\u30c8\u8ffd\u52a0", "Add Template")}
                    </button>
                )}
            </div>

            <button
                type="button"
                className="task-scheduler-submit-btn"
                onClick={() => void handleSave()}
                disabled={submitting}
            >
                {tr("viewer.taskScheduler.saveSettings", "\u4fdd\u5b58", "Save")}
            </button>
        </div>
    );
}
