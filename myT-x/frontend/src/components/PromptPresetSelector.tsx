import type {Dispatch, SetStateAction} from "react";
import {useCallback, useEffect, useMemo, useRef, useState} from "react";
import {api} from "../api";
import {usePromptPresetStore} from "../stores/promptPresetStore";
import {useTmuxStore} from "../stores/tmuxStore";
import {toErrorMessage} from "../utils/errorUtils";
import {shouldIgnoreSessionRequest} from "../utils/sessionGuard";
import {
    normalizePromptPresetLoadResult,
    type PromptPreset,
} from "./viewer/views/prompt-presets/types";

interface PromptPresetSelectorProps {
    setText: Dispatch<SetStateAction<string>>;
    onApplied?: () => void;
}

export function appendPromptPresetBody(previousText: string, presetBody: string): string {
    return previousText.length > 0 ? `${previousText}\n${presetBody}` : presetBody;
}

export function PromptPresetSelector({setText, onApplied}: PromptPresetSelectorProps) {
    const promptPresetVersion = usePromptPresetStore((state) => state.version);
    const sessions = useTmuxStore((state) => state.sessions);
    const activeSession = useTmuxStore((state) => state.activeSession);
    const activeSessionSnapshot = useMemo(
        () => (activeSession ? sessions.find((entry) => entry.name === activeSession) ?? null : null),
        [activeSession, sessions],
    );
    const activeSessionKey = activeSessionSnapshot ? `${activeSessionSnapshot.name}:${activeSessionSnapshot.id}` : "";
    const latestSessionKeyRef = useRef(activeSessionKey);
    const isMountedRef = useRef(true);
    const loadRequestTokenRef = useRef(0);

    const [presets, setPresets] = useState<PromptPreset[]>([]);
    const [selectedPresetID, setSelectedPresetID] = useState("");
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

    useEffect(() => {
        const loadPresets = async () => {
            const capturedSessionKey = latestSessionKeyRef.current;
            const requestToken = ++loadRequestTokenRef.current;
            setLoading(true);
            try {
                const result = normalizePromptPresetLoadResult(await api.LoadPromptPresets(activeSession ?? ""));
                if (
                    shouldIgnoreSessionRequest(
                        capturedSessionKey,
                        requestToken,
                        isMountedRef,
                        latestSessionKeyRef,
                        loadRequestTokenRef,
                    )
                ) {
                    return;
                }
                const nextPresets = result.presets;
                setPresets(nextPresets);
                setSelectedPresetID((current) => {
                    if (current !== "" && nextPresets.some((preset) => preset.id === current)) {
                        return current;
                    }
                    return nextPresets[0]?.id ?? "";
                });
                setError(null);
                setWarning(result.warnings.length > 0 ? result.warnings.join("\n") : null);
            } catch (err: unknown) {
                if (
                    shouldIgnoreSessionRequest(
                        capturedSessionKey,
                        requestToken,
                        isMountedRef,
                        latestSessionKeyRef,
                        loadRequestTokenRef,
                    )
                ) {
                    return;
                }
                console.warn("[prompt-preset-selector] failed to load prompt presets", err);
                setPresets([]);
                setSelectedPresetID("");
                setError(toErrorMessage(err, "Failed to load prompt presets."));
                setWarning(null);
            } finally {
                if (isMountedRef.current) {
                    setLoading(false);
                }
            }
        };

        void loadPresets();
    }, [activeSession, activeSessionKey, promptPresetVersion]);

    const selectedPreset = useMemo(
        () => presets.find((preset) => preset.id === selectedPresetID) ?? presets[0] ?? null,
        [presets, selectedPresetID],
    );
    const disabled = loading || selectedPreset === null || error !== null;

    const handleApply = useCallback(() => {
        if (selectedPreset === null) {
            return;
        }
        setText((previous) => appendPromptPresetBody(previous, selectedPreset.body));
        onApplied?.();
    }, [onApplied, selectedPreset, setText]);

    return (
        <div className="prompt-preset-selector">
            <select
                className="prompt-preset-selector-select"
                value={selectedPreset?.id ?? ""}
                onChange={(event) => setSelectedPresetID(event.target.value)}
                disabled={disabled}
                title={error ?? warning ?? "Prompt preset"}
                aria-label="Prompt preset"
            >
                {error && <option value="">{error}</option>}
                {!error && presets.length === 0 && (
                    <option value="">{loading ? "Loading presets..." : "No presets"}</option>
                )}
                {!error && presets.map((preset) => (
                    <option key={preset.id} value={preset.id}>
                        {preset.name}
                    </option>
                ))}
            </select>
            <button
                type="button"
                className="prompt-preset-selector-apply"
                disabled={disabled}
                onClick={handleApply}
            >
                Apply
            </button>
            {warning && (
                <span className="prompt-preset-selector-warning" title={warning} aria-label={warning}>
                    !
                </span>
            )}
        </div>
    );
}
