import {useMemo, useState} from "react";
import {useI18n} from "../../../../i18n";
import {ConfirmDialog} from "../../../ConfirmDialog";
import {useViewerStore} from "../../viewerStore";
import {ViewerPanelShell} from "../shared/ViewerPanelShell";
import {PromptPresetEditor} from "./PromptPresetEditor";
import {usePromptPresets} from "./usePromptPresets";
import type {PromptPreset, PromptPresetDraft} from "./types";
import {
    generatePromptPresetDraftID,
    resolvePromptPresetProjectSessionName,
    toPromptPresetDraft,
    toPromptPresetStorageLocation,
} from "./types";

type Screen = "list" | "editor";
type PendingNavigation = "back" | "close" | null;

interface PendingDeletePreset {
    preset: PromptPreset;
    projectSessionName: string | null;
}

function createEmptyDraft(activeSession: string | null): PromptPresetDraft {
    return {
        id: generatePromptPresetDraftID(),
        name: "",
        body: "",
        order: 0,
        storageLocation: "global",
        projectSessionName: resolvePromptPresetProjectSessionName("global", activeSession),
    };
}

export function isPromptPresetDraftDirty(draft: PromptPresetDraft | null, draftSnapshot: string | null): boolean {
    if (draft === null || draftSnapshot === null) {
        return false;
    }
    return JSON.stringify(draft) !== draftSnapshot;
}

export function PromptPresetsView() {
    const {t} = useI18n();
    const closeView = useViewerStore((state) => state.closeView);
    const {
        presets,
        loading,
        error,
        warning,
        activeSession,
        refresh,
        savePreset,
        deletePreset,
        moveUp,
        moveDown,
        setError,
        setWarning,
    } = usePromptPresets();

    const [screen, setScreen] = useState<Screen>("list");
    const [selectedPresetID, setSelectedPresetID] = useState<string | null>(null);
    const [draft, setDraft] = useState<PromptPresetDraft | null>(null);
    const [saving, setSaving] = useState(false);
    const [storageLocked, setStorageLocked] = useState(false);
    const [pendingDelete, setPendingDelete] = useState<PendingDeletePreset | null>(null);
    const [draftSnapshot, setDraftSnapshot] = useState<string | null>(null);
    const [pendingNavigation, setPendingNavigation] = useState<PendingNavigation>(null);

    const selectedPreset = useMemo(
        () => presets.find((preset) => preset.id === selectedPresetID) ?? null,
        [presets, selectedPresetID],
    );
    const canSave = draft !== null && draft.name.trim() !== "" && draft.body.trim() !== "";
    const isDirty = useMemo(
        () => isPromptPresetDraftDirty(draft, draftSnapshot),
        [draft, draftSnapshot],
    );

    const scopedPresets = useMemo(
        () => ({
            global: presets.filter((preset) => toPromptPresetStorageLocation(preset.storage_location) === "global"),
            project: presets.filter((preset) => toPromptPresetStorageLocation(preset.storage_location) === "project"),
        }),
        [presets],
    );

    const beginNewPreset = () => {
        const nextDraft = createEmptyDraft(activeSession);
        setDraft(nextDraft);
        setDraftSnapshot(JSON.stringify(nextDraft));
        setStorageLocked(false);
        setScreen("editor");
    };

    const beginEditPreset = (preset: PromptPreset) => {
        setSelectedPresetID(preset.id);
        const nextDraft = toPromptPresetDraft(preset, activeSession);
        setDraft(nextDraft);
        setDraftSnapshot(JSON.stringify(nextDraft));
        setStorageLocked(true);
        setScreen("editor");
    };

    const closeEditor = () => {
        setScreen("list");
        setDraft(null);
        setDraftSnapshot(null);
        setStorageLocked(false);
    };

    const handleBack = () => {
        if (isDirty) {
            setPendingNavigation("back");
            return;
        }
        closeEditor();
    };

    const handleClose = () => {
        if (screen === "editor" && isDirty) {
            setPendingNavigation("close");
            return;
        }
        closeView();
    };

    const handleSave = async () => {
        if (draft === null) {
            return;
        }
        setSaving(true);
        try {
            await savePreset(draft);
            closeEditor();
            if (draft.id !== "") {
                setSelectedPresetID(draft.id);
            }
        } catch {
            // The hook already updates the error banner.
        } finally {
            setSaving(false);
        }
    };

    return (
        <>
            <ViewerPanelShell
                className="prompt-presets-view"
                title={t("viewer.promptPresets.title", "Prompt Presets")}
                onClose={handleClose}
                onRefresh={() => {
                    void refresh().catch(() => {
                        // The hook already updated the error banner.
                    });
                }}
            >
                <div className="prompt-presets-body">
                    {error && (
                        <div className="prompt-presets-banner error">
                            <span>{error}</span>
                            <button type="button" onClick={() => setError(null)}>
                                {t("viewer.promptPresets.dismiss", "Dismiss")}
                            </button>
                        </div>
                    )}
                    {!error && warning && (
                        <div className="prompt-presets-banner warning">
                            <span>{warning}</span>
                            <button type="button" onClick={() => setWarning(null)}>
                                {t("viewer.promptPresets.dismiss", "Dismiss")}
                            </button>
                        </div>
                    )}

                    {screen === "list" && (
                        <div className="prompt-presets-list">
                            <div className="prompt-presets-toolbar">
                                <button type="button" className="prompt-presets-primary-btn" onClick={beginNewPreset}>
                                    {t("viewer.promptPresets.list.new", "+ New")}
                                </button>
                            </div>

                            {activeSession === null && (
                                <div className="prompt-presets-inline-note">
                                    {t(
                                        "viewer.promptPresets.list.noActiveSession",
                                        "Project presets become available after you select an active session.",
                                    )}
                                </div>
                            )}

                            {loading ? (
                                <div className="prompt-presets-empty">
                                    {t("viewer.promptPresets.list.loading", "Loading prompt presets...")}
                                </div>
                            ) : presets.length === 0 ? (
                                <div className="prompt-presets-empty">
                                    {t("viewer.promptPresets.list.empty", "No prompt presets saved yet.")}
                                </div>
                            ) : (
                                <div className="prompt-presets-sections">
                                    {(["global", "project"] as const).map((scope) => {
                                        const items = scopedPresets[scope];
                                        if (items.length === 0) {
                                            return null;
                                        }
                                        return (
                                            <section key={scope} className="prompt-presets-section">
                                                <div className="prompt-presets-section-title">
                                                    {scope === "global"
                                                        ? t("viewer.promptPresets.list.global", "Global")
                                                        : t("viewer.promptPresets.list.project", "Project")}
                                                </div>
                                                <div className="prompt-presets-cards">
                                                    {items.map((preset, index) => (
                                                        <div
                                                            key={preset.id}
                                                            className={`prompt-preset-card${selectedPreset?.id === preset.id ? " selected" : ""}`}
                                                            role="button"
                                                            tabIndex={0}
                                                            onClick={() => setSelectedPresetID(preset.id)}
                                                            onKeyDown={(event) => {
                                                                if (event.key === "Enter" || event.key === " ") {
                                                                    event.preventDefault();
                                                                    setSelectedPresetID(preset.id);
                                                                }
                                                            }}
                                                        >
                                                            <div className="prompt-preset-card-header">
                                                                <div className="prompt-preset-card-title">{preset.name}</div>
                                                                <span className={`prompt-preset-card-tag ${scope}`}>
                                                                    {scope === "global"
                                                                        ? t("viewer.promptPresets.list.globalShort", "Global")
                                                                        : t("viewer.promptPresets.list.projectShort", "Project")}
                                                                </span>
                                                            </div>
                                                            <div className="prompt-preset-card-body">{preset.body}</div>
                                                            <div className="prompt-preset-card-actions">
                                                                <button
                                                                    type="button"
                                                                    className="prompt-preset-card-btn icon"
                                                                    disabled={index === 0}
                                                                    onClick={(event) => {
                                                                        event.stopPropagation();
                                                                        void moveUp(
                                                                            preset.id,
                                                                            toPromptPresetStorageLocation(preset.storage_location),
                                                                            resolvePromptPresetProjectSessionName(scope, activeSession),
                                                                        ).catch(() => {
                                                                            // The hook already updates the error banner.
                                                                        });
                                                                    }}
                                                                    aria-label={t("viewer.promptPresets.list.moveUp", "Move up")}
                                                                >
                                                                    ▲
                                                                </button>
                                                                <button
                                                                    type="button"
                                                                    className="prompt-preset-card-btn icon"
                                                                    disabled={index === items.length - 1}
                                                                    onClick={(event) => {
                                                                        event.stopPropagation();
                                                                        void moveDown(
                                                                            preset.id,
                                                                            toPromptPresetStorageLocation(preset.storage_location),
                                                                            resolvePromptPresetProjectSessionName(scope, activeSession),
                                                                        ).catch(() => {
                                                                            // The hook already updates the error banner.
                                                                        });
                                                                    }}
                                                                    aria-label={t("viewer.promptPresets.list.moveDown", "Move down")}
                                                                >
                                                                    ▼
                                                                </button>
                                                                <button
                                                                    type="button"
                                                                    className="prompt-preset-card-btn"
                                                                    onClick={(event) => {
                                                                        event.stopPropagation();
                                                                        beginEditPreset(preset);
                                                                    }}
                                                                >
                                                                    {t("viewer.promptPresets.list.edit", "Edit")}
                                                                </button>
                                                                <button
                                                                    type="button"
                                                                    className="prompt-preset-card-btn danger"
                                                                    onClick={(event) => {
                                                                        event.stopPropagation();
                                                                        setPendingDelete({
                                                                            preset,
                                                                            projectSessionName: resolvePromptPresetProjectSessionName(scope, activeSession),
                                                                        });
                                                                    }}
                                                                >
                                                                    {t("viewer.promptPresets.list.delete", "Delete")}
                                                                </button>
                                                            </div>
                                                        </div>
                                                    ))}
                                                </div>
                                            </section>
                                        );
                                    })}
                                </div>
                            )}
                        </div>
                    )}

                    {screen === "editor" && draft !== null && (
                        <PromptPresetEditor
                            draft={draft}
                            saving={saving}
                            canSave={canSave}
                            activeSession={activeSession}
                            storageLocked={storageLocked}
                            onChange={setDraft}
                            onBack={handleBack}
                            onSave={() => {
                                void handleSave();
                            }}
                        />
                    )}
                </div>
            </ViewerPanelShell>

            <ConfirmDialog
                open={pendingDelete !== null}
                title={t("viewer.promptPresets.delete.title", "Delete prompt preset")}
                message={t(
                    "viewer.promptPresets.delete.message",
                    "Delete \"{name}\"? This action cannot be undone.",
                    {name: pendingDelete?.preset.name ?? ""},
                )}
                actions={[{
                    label: t("viewer.promptPresets.delete.confirm", "Delete"),
                    value: "delete",
                    variant: "danger",
                }]}
                onAction={(value) => {
                    if (value !== "delete" || pendingDelete === null) {
                        return;
                    }
                    void deletePreset(
                        pendingDelete.preset.id,
                        toPromptPresetStorageLocation(pendingDelete.preset.storage_location),
                        pendingDelete.projectSessionName,
                    )
                        .catch(() => {
                            // The hook already updates the error banner.
                        })
                        .finally(() => {
                            setPendingDelete(null);
                        });
                }}
                onClose={() => setPendingDelete(null)}
            />
            <ConfirmDialog
                open={pendingNavigation !== null}
                title={t("viewer.promptPresets.unsaved.title", "Unsaved prompt preset")}
                message={t(
                    "viewer.promptPresets.unsaved.message",
                    pendingNavigation === "close"
                        ? "Close without saving this prompt preset?"
                        : "Go back without saving this prompt preset?",
                )}
                actions={[{
                    label: t("viewer.promptPresets.unsaved.discard", pendingNavigation === "close" ? "Discard and close" : "Discard and go back"),
                    value: "discard",
                    variant: "danger",
                }]}
                onAction={(value) => {
                    if (value !== "discard") {
                        return;
                    }
                    const navigation = pendingNavigation;
                    setPendingNavigation(null);
                    if (navigation === "close") {
                        closeEditor();
                        closeView();
                        return;
                    }
                    closeEditor();
                }}
                onClose={() => setPendingNavigation(null)}
            />
        </>
    );
}
