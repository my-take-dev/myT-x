import {useState, useCallback, useEffect, useMemo, useRef} from "react";
import {useI18n} from "../../../../i18n";
import {api} from "../../../../api";
import {config} from "../../../../../wailsjs/go/models";
import type {ValidationRules} from "../../../../types/tmux";
import type {TaskSchedulerSettings, MessageTemplate} from "./useTaskScheduler";
import {toErrorMessage} from "../../../../utils/errorUtils";
import {
    blockNonNumericKeys,
    getPreExecFieldBounds,
    getTemplateFieldLimits,
    readNumberInput,
    resolveInitialPreExecIdleTimeoutWithRules,
    resolveInitialPreExecResetDelayWithRules,
    resolveInitialPreExecTargetMode,
} from "./taskSchedulerConfigForm";
import {
    PRE_EXEC_TARGET_MODE_ALL_PANES,
    PRE_EXEC_TARGET_MODE_TASK_PANES,
} from "./preExecTargetModes";

interface TaskSchedulerSettingsProps {
    initialSettings: TaskSchedulerSettings | null;
    onSave: (settings: TaskSchedulerSettings) => Promise<boolean>;
    onError: (message: string | null) => void;
    onBack: () => void;
    onDirtyChange?: (dirty: boolean) => void;
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

interface SettingsDraftState {
    preExecResetDelay: number;
    preExecIdleTimeout: number;
    preExecTargetMode: string;
    templates: MessageTemplate[];
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

function createSettingsDraftState(
    settings: TaskSchedulerSettings,
    validationRules: ValidationRules | null,
): SettingsDraftState {
    return {
        preExecResetDelay: resolveInitialPreExecResetDelayWithRules(settings.pre_exec_reset_delay_s, validationRules),
        preExecIdleTimeout: resolveInitialPreExecIdleTimeoutWithRules(settings.pre_exec_idle_timeout_s, validationRules),
        preExecTargetMode: resolveInitialPreExecTargetMode(settings.pre_exec_target_mode),
        templates: (settings.message_templates ?? []).map(({name, message}) => ({name, message})),
    };
}

function createDraftStateFromLocalState(
    preExecResetDelay: number,
    preExecIdleTimeout: number,
    preExecTargetMode: string,
    templates: TemplateDraft[],
): SettingsDraftState {
    return {
        preExecResetDelay,
        preExecIdleTimeout,
        preExecTargetMode,
        templates: templates.map(({name, message}) => ({name, message})),
    };
}

function equalSettingsDraft(left: SettingsDraftState | null, right: SettingsDraftState | null): boolean {
    if (left === right) {
        return true;
    }
    if (left === null || right === null) {
        return false;
    }
    if (
        left.preExecResetDelay !== right.preExecResetDelay ||
        left.preExecIdleTimeout !== right.preExecIdleTimeout ||
        left.preExecTargetMode !== right.preExecTargetMode ||
        left.templates.length !== right.templates.length
    ) {
        return false;
    }

    return left.templates.every((template, index) => (
        template.name === right.templates[index]?.name &&
        template.message === right.templates[index]?.message
    ));
}

function isDuplicateTemplateName(
    templates: TemplateDraft[],
    editing: TemplateEditState | null,
): boolean {
    if (!editing) {
        return false;
    }

    const trimmedName = editing.name.trim();
    if (trimmedName === "") {
        return false;
    }

    return templates.some((template) => (
        template.id !== editing.templateID &&
        template.name.trim() === trimmedName
    ));
}

export function TaskSchedulerSettingsPanel({
                                               initialSettings,
                                               onSave,
                                               onError,
                                               onBack,
                                               onDirtyChange,
                                           }: TaskSchedulerSettingsProps) {
    const {language, t} = useI18n();
    const tr = (key: string, jaText: string, enText: string) =>
        t(key, language === "ja" ? jaText : enText);

    const [submitting, setSubmitting] = useState(false);
    // Incremented when the baseline ref advances during a dirty edit to force
    // re-render and recalculate hasUnsavedChanges against the new baseline.
    const [, setBaselineVersion] = useState(0);
    const [validationRules, setValidationRules] = useState<ValidationRules | null>(null);
    const fieldBounds = getPreExecFieldBounds(validationRules);
    const templateLimits = getTemplateFieldLimits(validationRules);

    // Pre-exec settings
    const [preExecResetDelay, setPreExecResetDelay] = useState(
        resolveInitialPreExecResetDelayWithRules(initialSettings?.pre_exec_reset_delay_s, validationRules),
    );
    const [preExecIdleTimeout, setPreExecIdleTimeout] = useState(
        resolveInitialPreExecIdleTimeoutWithRules(initialSettings?.pre_exec_idle_timeout_s, validationRules),
    );
    const [preExecTargetMode, setPreExecTargetMode] = useState(
        resolveInitialPreExecTargetMode(initialSettings?.pre_exec_target_mode),
    );

    // Message templates
    const [templates, setTemplates] = useState<TemplateDraft[]>(
        () => (initialSettings?.message_templates ?? []).map((template) => createTemplateDraft(template)),
    );
    const [editing, setEditing] = useState<TemplateEditState | null>(null);
    const syncedDraftRef = useRef<SettingsDraftState | null>(
        initialSettings ? createSettingsDraftState(initialSettings, validationRules) : null,
    );
    const currentDraft = useMemo(() => createDraftStateFromLocalState(
        preExecResetDelay,
        preExecIdleTimeout,
        preExecTargetMode,
        templates,
    ), [preExecIdleTimeout, preExecResetDelay, preExecTargetMode, templates]);
    const settingsLoaded = syncedDraftRef.current !== null;
    const hasUnsavedChanges = syncedDraftRef.current !== null
        && !equalSettingsDraft(currentDraft, syncedDraftRef.current);

    // The settings payload arrives asynchronously from the backend, so the form
    // must refresh its local draft state when that source of truth changes.
    useEffect(() => {
        let cancelled = false;
        void api.GetValidationRules()
            .then((rules) => {
                if (!cancelled) {
                    setValidationRules(rules);
                }
            })
            .catch((err: unknown) => {
                console.warn("[task-scheduler] failed to load validation rules, using fallbacks", err);
                if (!cancelled) {
                    onError("Validation rules are unavailable; using fallback limits.");
                }
            });
        return () => {
            cancelled = true;
        };
    }, [onError]);

    useEffect(() => {
        if (initialSettings === null) {
            syncedDraftRef.current = null;
            return;
        }

        const nextDraft = createSettingsDraftState(initialSettings, validationRules);
        // Always advance the baseline so dirty comparison uses the latest
        // backend state, even when the user has in-progress edits.
        // hasUnsavedChanges is computed during render against the PREVIOUS
        // baseline, so it correctly reflects whether the user had pending edits
        // before this settings update arrived.
        syncedDraftRef.current = nextDraft;

        if (hasUnsavedChanges || editing !== null) {
            // Force re-render so hasUnsavedChanges recalculates against the
            // updated baseline. Without this, the ref mutation alone would not
            // trigger a re-render and dirty state would remain stale.
            setBaselineVersion((v) => v + 1);
            return;
        }

        setPreExecResetDelay(nextDraft.preExecResetDelay);
        setPreExecIdleTimeout(nextDraft.preExecIdleTimeout);
        setPreExecTargetMode(nextDraft.preExecTargetMode);
        setTemplates(nextDraft.templates.map((template) => createTemplateDraft(template)));
        setEditing(null);
    }, [editing, hasUnsavedChanges, initialSettings, validationRules]);

    useEffect(() => {
        onDirtyChange?.(hasUnsavedChanges);
    }, [hasUnsavedChanges, onDirtyChange]);

    const handleAddTemplate = useCallback(() => {
        if (templates.length >= templateLimits.maxMessageTemplates) {
            return;
        }
        setEditing({mode: "add", templateID: null, name: "", message: ""});
    }, [templateLimits.maxMessageTemplates, templates.length]);

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
        if (name.length > templateLimits.maxTemplateNameLen || message.length > templateLimits.maxTemplateMessageLen) {
            return;
        }
        if (isDuplicateTemplateName(templates, editing)) {
            return;
        }

        const entry = {name, message};
        if (editing.mode === "add") {
            if (templates.length >= templateLimits.maxMessageTemplates) {
                return;
            }
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
    }, [editing, templateLimits.maxMessageTemplates, templateLimits.maxTemplateMessageLen, templateLimits.maxTemplateNameLen, templates]);

    const handleCancelTemplate = useCallback(() => {
        setEditing(null);
    }, []);

    const hasDuplicateTemplate = isDuplicateTemplateName(templates, editing);
    const canSaveTemplate =
        settingsLoaded &&
        editing !== null &&
        editing.name.trim() !== "" &&
        editing.message.trim() !== "" &&
        editing.name.trim().length <= templateLimits.maxTemplateNameLen &&
        editing.message.trim().length <= templateLimits.maxTemplateMessageLen &&
        !hasDuplicateTemplate;
    const canAddTemplate = settingsLoaded && editing === null && templates.length < templateLimits.maxMessageTemplates;

    const handleSave = useCallback(async () => {
        if (submitting || !settingsLoaded) return;
        onError(null);
        setSubmitting(true);
        try {
            // Re-apply resolve as a safety net: ensures the submitted values are
            // within valid bounds even if local state was somehow corrupted.
            const settings = new config.TaskSchedulerConfig({
                pre_exec_reset_delay_s: resolveInitialPreExecResetDelayWithRules(preExecResetDelay, validationRules),
                pre_exec_idle_timeout_s: resolveInitialPreExecIdleTimeoutWithRules(preExecIdleTimeout, validationRules),
                pre_exec_target_mode: preExecTargetMode,
                message_templates: templates.map(({name, message}) => ({name, message})),
            });
            const ok = await onSave(settings);
            if (ok) {
                onBack();
            }
        } catch (err: unknown) {
            onError(toErrorMessage(err, "Failed to save task scheduler settings"));
        } finally {
            setSubmitting(false);
        }
    }, [submitting, settingsLoaded, onSave, onError, onBack, preExecResetDelay, preExecIdleTimeout, preExecTargetMode, templates, validationRules]);

    return (
        <div className="task-scheduler-form">
            <button type="button" className="task-scheduler-back-btn" onClick={onBack}>
                {"\u2190 "}{tr("viewer.taskScheduler.back", "\u623b\u308b", "Back")}
            </button>

            <h3 className="task-scheduler-config-title">
                {tr("viewer.taskScheduler.settingsTitle", "\u30b9\u30b1\u30b8\u30e5\u30fc\u30e9\u8a2d\u5b9a", "Scheduler Settings")}
            </h3>

            {!settingsLoaded ? (
                <div className="viewer-message">
                    {tr(
                        "viewer.taskScheduler.preExecSettingsLoading",
                        "\u30b9\u30b1\u30b8\u30e5\u30fc\u30e9\u8a2d\u5b9a\u3092\u8aad\u307f\u8fbc\u307f\u4e2d...",
                        "Loading scheduler settings...",
                    )}
                </div>
            ) : (
                <>
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
                                        fieldBounds.resetDelay.min,
                                        fieldBounds.resetDelay.max,
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
                                        fieldBounds.idleTimeout.min,
                                        fieldBounds.idleTimeout.max,
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
                            checked={preExecTargetMode === PRE_EXEC_TARGET_MODE_TASK_PANES}
                            onChange={() => setPreExecTargetMode(PRE_EXEC_TARGET_MODE_TASK_PANES)}
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
                            checked={preExecTargetMode === PRE_EXEC_TARGET_MODE_ALL_PANES}
                            onChange={() => setPreExecTargetMode(PRE_EXEC_TARGET_MODE_ALL_PANES)}
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
                            maxLength={templateLimits.maxTemplateNameLen}
                            placeholder={tr("viewer.taskScheduler.templateName", "\u30c6\u30f3\u30d7\u30ec\u30fc\u30c8\u540d", "Template name")}
                        />
                        <textarea
                            className="task-scheduler-textarea"
                            value={editing.message}
                            onChange={(e) =>
                                setEditing((prev) => prev && {...prev, message: e.target.value})
                            }
                            maxLength={templateLimits.maxTemplateMessageLen}
                            placeholder={tr("viewer.taskScheduler.templateMessage", "\u30e1\u30c3\u30bb\u30fc\u30b8\u672c\u6587", "Message body")}
                        />
                        {hasDuplicateTemplate && (
                            <div className="task-scheduler-template-error">
                                {tr(
                                    "viewer.taskScheduler.templateNameDuplicate",
                                    "\u540c\u540d\u306e\u30c6\u30f3\u30d7\u30ec\u30fc\u30c8\u306f\u4fdd\u5b58\u3067\u304d\u307e\u305b\u3093",
                                    "A template with this name already exists.",
                                )}
                            </div>
                        )}
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
                        disabled={!canAddTemplate}
                    >
                        + {tr("viewer.taskScheduler.addTemplate", "\u30c6\u30f3\u30d7\u30ec\u30fc\u30c8\u8ffd\u52a0", "Add Template")}
                    </button>
                )}
            </div>

                </>
            )}

            <button
                type="button"
                className="task-scheduler-submit-btn"
                onClick={() => void handleSave()}
                disabled={submitting || !settingsLoaded}
            >
                {tr("viewer.taskScheduler.saveSettings", "\u4fdd\u5b58", "Save")}
            </button>
        </div>
    );
}
