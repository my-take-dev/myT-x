import {useState, useCallback, useRef, useEffect, type ChangeEvent} from "react";
import type {PaneSnapshot} from "../../../../types/tmux";
import {
    SCHEDULER_INFINITE_COUNT,
    clampSchedulerMaxCount,
    isSchedulerMaxCountValid,
} from "./usePaneScheduler";
import type {SchedulerEditDraft, SchedulerStartValues, SchedulerTemplate} from "./usePaneScheduler";

interface SchedulerFormProps {
    availablePanes: PaneSnapshot[];
    initialDraft?: SchedulerEditDraft | null;
    templates: SchedulerTemplate[];
    onStart: (values: SchedulerStartValues) => Promise<void>;
    onBack: () => void;
    onSaveTemplate: (tmpl: SchedulerTemplate) => Promise<void>;
    onDeleteTemplate: (title: string) => Promise<void>;
    submitLabel?: string;
}

const MANUAL_INPUT = "__manual__";
const SCHEDULER_MIN_INPUT = 0;

export function SchedulerForm({
                                  availablePanes,
                                  initialDraft,
                                  templates,
                                  onStart,
                                  onBack,
                                  onSaveTemplate,
                                  onDeleteTemplate,
                                  submitLabel = "Start",
                              }: SchedulerFormProps) {
    const [selectedTemplate, setSelectedTemplate] = useState(MANUAL_INPUT);
    const [title, setTitle] = useState(initialDraft?.title ?? "");
    const [paneID, setPaneID] = useState(initialDraft?.paneID ?? "");
    const [message, setMessage] = useState(initialDraft?.message ?? "");
    const [intervalMinutes, setIntervalMinutes] = useState(initialDraft?.intervalMinutes ?? 5);
    const [maxCount, setMaxCount] = useState(initialDraft?.maxCount ?? 1);
    const [submitting, setSubmitting] = useState(false);
    const [saving, setSaving] = useState(false);

    const textareaRef = useRef<HTMLTextAreaElement>(null);

    const isFormValid =
        title.trim() !== "" &&
        paneID !== "" &&
        message !== "" &&
        intervalMinutes >= 1 &&
        isSchedulerMaxCountValid(maxCount);

    const isTemplateValid =
        title.trim() !== "" &&
        message !== "" &&
        intervalMinutes >= 1 &&
        isSchedulerMaxCountValid(maxCount);

    // Auto-resize textarea on input.
    const handleMessageChange = useCallback((e: ChangeEvent<HTMLTextAreaElement>) => {
        setMessage(e.target.value);
        const el = e.target;
        el.style.height = "auto";
        el.style.height = `${Math.min(el.scrollHeight, 300)}px`;
    }, []);

    // Reset textarea height when message is cleared.
    useEffect(() => {
        if (message === "" && textareaRef.current) {
            textareaRef.current.style.height = "auto";
        }
    }, [message]);

    useEffect(() => {
        setSelectedTemplate(MANUAL_INPUT);
        setTitle(initialDraft?.title ?? "");
        setPaneID(initialDraft?.paneID ?? "");
        setMessage(initialDraft?.message ?? "");
        setIntervalMinutes(initialDraft?.intervalMinutes ?? 5);
        setMaxCount(initialDraft?.maxCount ?? 1);
        if (textareaRef.current) {
            textareaRef.current.style.height = "auto";
            requestAnimationFrame(() => {
                if (textareaRef.current) {
                    textareaRef.current.style.height = `${Math.min(textareaRef.current.scrollHeight, 300)}px`;
                }
            });
        }
    }, [initialDraft]);

    const handleTemplateChange = useCallback(
        (e: ChangeEvent<HTMLSelectElement>) => {
            const value = e.target.value;
            setSelectedTemplate(value);
            if (value === MANUAL_INPUT) return;
            const tmpl = templates.find((t) => t.title === value);
            if (!tmpl) return;
            setTitle(tmpl.title);
            setMessage(tmpl.message);
            setIntervalMinutes(tmpl.interval_minutes);
            setMaxCount(tmpl.max_count);
            // Reset textarea height for new content.
            if (textareaRef.current) {
                textareaRef.current.style.height = "auto";
                requestAnimationFrame(() => {
                    if (textareaRef.current) {
                        textareaRef.current.style.height = `${Math.min(textareaRef.current.scrollHeight, 300)}px`;
                    }
                });
            }
        },
        [templates],
    );

    const handleSubmit = useCallback(async () => {
        if (!isFormValid || submitting) return;
        setSubmitting(true);
        try {
            await onStart({
                title: title.trim(),
                paneID,
                message,
                intervalMinutes,
                maxCount,
            });
            onBack();
        } catch (e) {
            // Wails API error is handled in the parent hook.
            console.warn("[pane-scheduler] handleSubmit catch", e);
        } finally {
            setSubmitting(false);
        }
    }, [isFormValid, submitting, title, paneID, message, intervalMinutes, maxCount, onStart, onBack]);

    const handleSaveTemplate = useCallback(async () => {
        if (!isTemplateValid || saving) return;
        setSaving(true);
        try {
            await onSaveTemplate({
                title: title.trim(),
                message,
                interval_minutes: intervalMinutes,
                max_count: maxCount,
            });
            setSelectedTemplate(title.trim());
        } catch (e) {
            // Wails API error is handled in the parent hook.
            console.warn("[pane-scheduler] handleSaveTemplate catch", e);
        } finally {
            setSaving(false);
        }
    }, [isTemplateValid, saving, title, message, intervalMinutes, maxCount, onSaveTemplate]);

    const handleDeleteTemplate = useCallback(async () => {
        if (selectedTemplate === MANUAL_INPUT) return;
        try {
            await onDeleteTemplate(selectedTemplate);
            setSelectedTemplate(MANUAL_INPUT);
        } catch (e) {
            // Wails API error is handled in the parent hook.
            console.warn("[pane-scheduler] handleDeleteTemplate catch", e);
        }
    }, [selectedTemplate, onDeleteTemplate]);

    return (
        <div className="pane-scheduler-form">
            <button
                type="button"
                className="pane-scheduler-back-btn"
                onClick={onBack}
            >
                &larr; Back
            </button>

            <div className="pane-scheduler-template-row">
                <div className="form-group">
                    <label className="form-label">Template</label>
                    <select
                        className="form-select"
                        value={selectedTemplate}
                        onChange={handleTemplateChange}
                    >
                        <option value={MANUAL_INPUT}>-- Manual input --</option>
                        {templates.map((t) => (
                            <option key={t.title} value={t.title}>
                                {t.title}
                            </option>
                        ))}
                    </select>
                </div>
                {selectedTemplate !== MANUAL_INPUT && (
                    <button
                        type="button"
                        className="pane-scheduler-delete-tmpl-btn"
                        onClick={handleDeleteTemplate}
                        title="Delete this template"
                    >
                        Delete
                    </button>
                )}
            </div>

            <div className="form-group">
                <label className="form-label">Title</label>
                <input
                    className="form-input"
                    type="text"
                    value={title}
                    onChange={(e) => setTitle(e.target.value)}
                    placeholder="Scheduler name..."
                />
            </div>

            <div className="form-group">
                <label className="form-label">Target Pane</label>
                <select
                    className="form-select"
                    value={paneID}
                    onChange={(e) => setPaneID(e.target.value)}
                >
                    <option value="">Select pane...</option>
                    {availablePanes.map((p) => (
                        <option key={p.id} value={p.id}>
                            {p.id}{p.title ? ` (${p.title})` : ""}
                        </option>
                    ))}
                </select>
            </div>

            <div className="form-group">
                <label className="form-label">Message</label>
                <textarea
                    ref={textareaRef}
                    className="form-input pane-scheduler-textarea"
                    value={message}
                    onChange={handleMessageChange}
                    placeholder="Message to send..."
                />
            </div>

            <div className="pane-scheduler-row">
                <div className="form-group pane-scheduler-half">
                    <label className="form-label">Interval (min)</label>
                    <input
                        className="form-input"
                        type="number"
                        min={1}
                        value={intervalMinutes}
                        onChange={(e) => setIntervalMinutes(Math.max(1, Number(e.target.value)))}
                    />
                </div>
                <div className="form-group pane-scheduler-half">
                    <label className="form-label">Count ({SCHEDULER_INFINITE_COUNT}=&infin;)</label>
                    <input
                        className="form-input"
                        type="number"
                        min={SCHEDULER_MIN_INPUT}
                        value={maxCount}
                        onChange={(e) => {
                            setMaxCount(clampSchedulerMaxCount(Number(e.target.value)));
                        }}
                    />
                </div>
            </div>

            <div className="pane-scheduler-btn-row">
                <button
                    type="button"
                    className="pane-scheduler-start-btn"
                    disabled={!isFormValid || submitting}
                    onClick={handleSubmit}
                >
                    {submitting ? (submitLabel === "Start" ? "Starting..." : "Applying...") : submitLabel}
                </button>
                <button
                    type="button"
                    className="pane-scheduler-save-btn"
                    disabled={!isTemplateValid || saving}
                    onClick={handleSaveTemplate}
                >
                    {saving ? "Saving..." : "Save as Template"}
                </button>
            </div>
        </div>
    );
}
