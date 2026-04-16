import type {Dispatch, SetStateAction} from "react";
import {useCallback, useEffect, useMemo, useRef, useState} from "react";
import {promptpresets} from "../../../../../wailsjs/go/models";
import {api} from "../../../../api";
import {usePromptPresetStore} from "../../../../stores/promptPresetStore";
import {useTmuxStore} from "../../../../stores/tmuxStore";
import {toErrorMessage} from "../../../../utils/errorUtils";
import {shouldIgnoreSessionMutation, shouldIgnoreSessionRequest} from "../../../../utils/sessionGuard";
import type {PromptPreset, PromptPresetDraft, PromptPresetStorageLocation} from "./types";
import {
    normalizePromptPresetLoadResult,
    toPromptPresetStorageLocation,
} from "./types";

interface UsePromptPresetsResult {
    presets: PromptPreset[];
    loading: boolean;
    error: string | null;
    warning: string | null;
    activeSession: string | null;
    refresh: (capturedSessionKey?: string) => Promise<void>;
    savePreset: (draft: PromptPresetDraft) => Promise<void>;
    deletePreset: (
        presetID: string,
        storageLocation: PromptPresetStorageLocation,
        projectSessionName?: string | null,
    ) => Promise<void>;
    moveUp: (
        presetID: string,
        storageLocation: PromptPresetStorageLocation,
        projectSessionName?: string | null,
    ) => Promise<void>;
    moveDown: (
        presetID: string,
        storageLocation: PromptPresetStorageLocation,
        projectSessionName?: string | null,
    ) => Promise<void>;
    setError: Dispatch<SetStateAction<string | null>>;
    setWarning: Dispatch<SetStateAction<string | null>>;
}

function buildPresetPayload(draft: PromptPresetDraft): promptpresets.PromptPreset {
    return promptpresets.PromptPreset.createFrom({
        id: draft.id,
        name: draft.name,
        body: draft.body,
        order: draft.order,
        storage_location: draft.storageLocation,
    });
}

function resolveMutationSessionName(
    storageLocation: PromptPresetStorageLocation,
    projectSessionName: string | null | undefined,
): string {
    if (storageLocation === "global") {
        return "";
    }
    const sessionName = projectSessionName?.trim() ?? "";
    if (sessionName === "") {
        throw new Error("An active session is required for project prompt presets.");
    }
    return sessionName;
}

export function usePromptPresets(): UsePromptPresetsResult {
    const bumpVersion = usePromptPresetStore((state) => state.bumpVersion);
    const sessions = useTmuxStore((state) => state.sessions);
    const activeSession = useTmuxStore((state) => state.activeSession);
    const activeSessionSnapshot = useMemo(
        () => (activeSession ? sessions.find((entry) => entry.name === activeSession) ?? null : null),
        [activeSession, sessions],
    );
    const activeSessionKey = activeSessionSnapshot ? `${activeSessionSnapshot.name}:${activeSessionSnapshot.id}` : "";
    const latestSessionKeyRef = useRef(activeSessionKey);
    const isMountedRef = useRef(true);
    const refreshRequestTokenRef = useRef(0);

    const [presets, setPresets] = useState<PromptPreset[]>([]);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);
    const [warning, setWarning] = useState<string | null>(null);

    latestSessionKeyRef.current = activeSessionKey;

    useEffect(() => {
        isMountedRef.current = true;
        return () => {
            isMountedRef.current = false;
        };
    }, []);

    const refresh = useCallback(async (capturedSessionKey: string = latestSessionKeyRef.current) => {
        const requestToken = ++refreshRequestTokenRef.current;
        setLoading(true);
        try {
            const result = normalizePromptPresetLoadResult(await api.LoadPromptPresets(activeSession ?? ""));
            if (
                shouldIgnoreSessionRequest(
                    capturedSessionKey,
                    requestToken,
                    isMountedRef,
                    latestSessionKeyRef,
                    refreshRequestTokenRef,
                )
            ) {
                return;
            }
            setPresets(result.presets);
            setError(null);
            setWarning(result.warnings.length > 0 ? result.warnings.join("\n") : null);
        } catch (err: unknown) {
            if (
                shouldIgnoreSessionRequest(
                    capturedSessionKey,
                    requestToken,
                    isMountedRef,
                    latestSessionKeyRef,
                    refreshRequestTokenRef,
                )
            ) {
                return;
            }
            console.warn("[prompt-presets] failed to load prompt presets", err);
            setError(toErrorMessage(err, "Failed to load prompt presets."));
            setWarning(null);
            throw err;
        } finally {
            if (isMountedRef.current) {
                setLoading(false);
            }
        }
    }, [activeSession]);

    useEffect(() => {
        void refresh(activeSessionKey).catch(() => {
            // The hook already updated the user-visible error state.
        });
    }, [activeSessionKey, refresh]);

    const savePreset = useCallback(async (draft: PromptPresetDraft) => {
        const capturedSessionKey = latestSessionKeyRef.current;
        setError(null);
        setWarning(null);
        const sessionName = resolveMutationSessionName(draft.storageLocation, draft.projectSessionName);
        try {
            await api.SavePromptPreset(buildPresetPayload(draft), sessionName);
            if (shouldIgnoreSessionMutation(capturedSessionKey, isMountedRef, latestSessionKeyRef)) {
                return;
            }
            try {
                await refresh(capturedSessionKey);
            } catch (refreshErr: unknown) {
                if (shouldIgnoreSessionMutation(capturedSessionKey, isMountedRef, latestSessionKeyRef)) {
                    return;
                }
                console.warn("[prompt-presets] prompt preset saved but reload failed", refreshErr);
                setError("Prompt preset saved, but reloading the list failed.");
            }
            bumpVersion();
        } catch (err: unknown) {
            if (shouldIgnoreSessionMutation(capturedSessionKey, isMountedRef, latestSessionKeyRef)) {
                return;
            }
            setError(toErrorMessage(err, "Failed to save prompt preset."));
            throw err;
        }
    }, [bumpVersion, refresh]);

    const deletePreset = useCallback(async (
        presetID: string,
        storageLocation: PromptPresetStorageLocation,
        projectSessionName?: string | null,
    ) => {
        const capturedSessionKey = latestSessionKeyRef.current;
        setError(null);
        setWarning(null);
        const sessionName = resolveMutationSessionName(storageLocation, projectSessionName);
        try {
            await api.DeletePromptPreset(presetID, storageLocation, sessionName);
            if (shouldIgnoreSessionMutation(capturedSessionKey, isMountedRef, latestSessionKeyRef)) {
                return;
            }
            try {
                await refresh(capturedSessionKey);
            } catch (refreshErr: unknown) {
                if (shouldIgnoreSessionMutation(capturedSessionKey, isMountedRef, latestSessionKeyRef)) {
                    return;
                }
                console.warn("[prompt-presets] prompt preset deleted but reload failed", refreshErr);
                setError("Prompt preset deleted, but reloading the list failed.");
            }
            bumpVersion();
        } catch (err: unknown) {
            if (shouldIgnoreSessionMutation(capturedSessionKey, isMountedRef, latestSessionKeyRef)) {
                return;
            }
            setError(toErrorMessage(err, "Failed to delete prompt preset."));
            throw err;
        }
    }, [bumpVersion, refresh]);

    const movePreset = useCallback(async (
        presetID: string,
        storageLocation: PromptPresetStorageLocation,
        direction: "up" | "down",
        projectSessionName?: string | null,
    ) => {
        const targetPreset = presets.find((preset) => preset.id === presetID);
        if (!targetPreset) {
            return;
        }

        const scopedPresets = presets.filter((preset) => toPromptPresetStorageLocation(preset.storage_location) === storageLocation);
        const index = scopedPresets.findIndex((preset) => preset.id === presetID);
        if (index < 0) {
            return;
        }
        if (direction === "up" && index === 0) {
            return;
        }
        if (direction === "down" && index === scopedPresets.length - 1) {
            return;
        }

        const swapIndex = direction === "up" ? index - 1 : index + 1;
        const reorderedIDs = scopedPresets.map((preset) => preset.id);
        [reorderedIDs[index], reorderedIDs[swapIndex]] = [reorderedIDs[swapIndex], reorderedIDs[index]];

        const capturedSessionKey = latestSessionKeyRef.current;
        setError(null);
        setWarning(null);
        const sessionName = resolveMutationSessionName(storageLocation, projectSessionName);
        try {
            await api.ReorderPromptPresets(reorderedIDs, storageLocation, sessionName);
            if (shouldIgnoreSessionMutation(capturedSessionKey, isMountedRef, latestSessionKeyRef)) {
                return;
            }
            try {
                await refresh(capturedSessionKey);
            } catch (refreshErr: unknown) {
                if (shouldIgnoreSessionMutation(capturedSessionKey, isMountedRef, latestSessionKeyRef)) {
                    return;
                }
                console.warn("[prompt-presets] prompt presets reordered but reload failed", refreshErr);
                setError("Prompt preset order updated, but reloading the list failed.");
            }
            bumpVersion();
        } catch (err: unknown) {
            if (shouldIgnoreSessionMutation(capturedSessionKey, isMountedRef, latestSessionKeyRef)) {
                return;
            }
            setError(toErrorMessage(err, "Failed to reorder prompt presets."));
            throw err;
        }
    }, [bumpVersion, presets, refresh]);

    const moveUp = useCallback(async (
        presetID: string,
        storageLocation: PromptPresetStorageLocation,
        projectSessionName?: string | null,
    ) => {
        await movePreset(presetID, storageLocation, "up", projectSessionName);
    }, [movePreset]);

    const moveDown = useCallback(async (
        presetID: string,
        storageLocation: PromptPresetStorageLocation,
        projectSessionName?: string | null,
    ) => {
        await movePreset(presetID, storageLocation, "down", projectSessionName);
    }, [movePreset]);

    return {
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
    };
}
