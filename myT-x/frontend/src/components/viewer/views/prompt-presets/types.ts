import {promptpresets} from "../../../../../wailsjs/go/models";

export type PromptPreset = promptpresets.PromptPreset;
export type PromptPresetStorageLocation = "global" | "project";

export interface PromptPresetDraft {
    id: string;
    name: string;
    body: string;
    order: number;
    storageLocation: PromptPresetStorageLocation;
    projectSessionName: string | null;
}

export interface PromptPresetLoadResult {
    presets: PromptPreset[];
    warnings: string[];
}

export function generatePromptPresetDraftID(): string {
    if (typeof globalThis.crypto?.randomUUID === "function") {
        return globalThis.crypto.randomUUID();
    }
    return `prompt-preset-${Date.now()}-${Math.random().toString(16).slice(2)}`;
}

export function toPromptPresetStorageLocation(storageLocation?: string): PromptPresetStorageLocation {
    if (storageLocation === undefined || storageLocation === "" || storageLocation === "global") {
        return "global";
    }
    if (storageLocation === "project") {
        return "project";
    }
    throw new Error(`Unsupported prompt preset storage location: ${storageLocation}`);
}

export function normalizePromptPresetLoadResult(result: unknown): PromptPresetLoadResult {
    if (Array.isArray(result)) {
        validatePromptPresets(result);
        return {presets: result as PromptPreset[], warnings: []};
    }
    if (result === null || typeof result !== "object") {
        throw new Error("LoadPromptPresets returned an invalid payload.");
    }

    const payload = result as {presets?: unknown; warnings?: unknown};
    const presets = Array.isArray(payload.presets) ? (payload.presets as PromptPreset[]) : [];
    const warnings = Array.isArray(payload.warnings)
        ? payload.warnings.filter((warning): warning is string => typeof warning === "string" && warning.trim() !== "")
        : [];
    validatePromptPresets(presets);
    return {presets, warnings};
}

export function resolvePromptPresetProjectSessionName(
    storageLocation: PromptPresetStorageLocation,
    activeSession: string | null,
): string | null {
    return storageLocation === "project" ? activeSession : null;
}

export function validatePromptPresets(presets: PromptPreset[]): void {
    for (const preset of presets) {
        toPromptPresetStorageLocation(preset.storage_location);
    }
}

export function toPromptPresetDraft(preset: PromptPreset, activeSession: string | null): PromptPresetDraft {
    const storageLocation = toPromptPresetStorageLocation(preset.storage_location);
    return {
        id: preset.id,
        name: preset.name,
        body: preset.body,
        order: preset.order,
        storageLocation,
        projectSessionName: resolvePromptPresetProjectSessionName(storageLocation, activeSession),
    };
}
