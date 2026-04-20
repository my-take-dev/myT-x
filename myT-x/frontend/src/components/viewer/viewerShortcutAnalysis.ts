import type {ReservedViewerShortcutDef} from "./viewerReservedShortcuts";
import {getReservedViewerShortcutDef} from "./viewerReservedShortcuts";
import {getEffectiveViewerShortcut, hasShortcutModifier, normalizeShortcut} from "./viewerShortcutUtils";

export interface ViewerShortcutSource {
    readonly id: string;
    readonly configuredShortcut: string | null | undefined;
    readonly defaultShortcut: string | null | undefined;
}

export type ViewerShortcutIssue =
    | { readonly kind: "modifier-required" }
    | { readonly kind: "duplicate-global-hotkey" }
    | { readonly kind: "reserved"; readonly reservedShortcut: ReservedViewerShortcutDef }
    | { readonly kind: "duplicate-view"; readonly conflictingViewIds: readonly string[] };

export interface ViewerShortcutAnalysis {
    readonly id: string;
    readonly effectiveShortcut: string | null;
    readonly normalizedShortcut: string;
    readonly bindingShortcut: string | null;
    readonly issue: ViewerShortcutIssue | null;
}

export function analyzeViewerShortcuts(
    sources: ReadonlyArray<ViewerShortcutSource>,
    globalHotkey: string = "",
): ReadonlyMap<string, ViewerShortcutAnalysis> {
    const analyses = new Map<string, ViewerShortcutAnalysis>();
    const ownersByShortcut = new Map<string, string[]>();
    const normalizedGlobalHotkey = normalizeShortcut(globalHotkey);

    for (const source of sources) {
        const effectiveShortcut = getEffectiveViewerShortcut(
            source.configuredShortcut,
            source.defaultShortcut,
        );
        if (!effectiveShortcut) {
            analyses.set(source.id, {
                id: source.id,
                effectiveShortcut: null,
                normalizedShortcut: "",
                bindingShortcut: null,
                issue: null,
            });
            continue;
        }

        if (!hasShortcutModifier(effectiveShortcut)) {
            analyses.set(source.id, {
                id: source.id,
                effectiveShortcut,
                normalizedShortcut: "",
                bindingShortcut: null,
                issue: {kind: "modifier-required"},
            });
            continue;
        }

        const normalizedShortcut = normalizeShortcut(effectiveShortcut);
        if (normalizedShortcut === "") {
            analyses.set(source.id, {
                id: source.id,
                effectiveShortcut,
                normalizedShortcut,
                bindingShortcut: null,
                issue: null,
            });
            continue;
        }

        if (normalizedGlobalHotkey !== "" && normalizedShortcut === normalizedGlobalHotkey) {
            analyses.set(source.id, {
                id: source.id,
                effectiveShortcut,
                normalizedShortcut,
                bindingShortcut: null,
                issue: {kind: "duplicate-global-hotkey"},
            });
            continue;
        }

        const reservedShortcut = getReservedViewerShortcutDef(normalizedShortcut);
        if (reservedShortcut) {
            analyses.set(source.id, {
                id: source.id,
                effectiveShortcut,
                normalizedShortcut,
                bindingShortcut: null,
                issue: {kind: "reserved", reservedShortcut},
            });
            continue;
        }

        const owners = ownersByShortcut.get(normalizedShortcut);
        if (owners) {
            owners.push(source.id);
        } else {
            ownersByShortcut.set(normalizedShortcut, [source.id]);
        }
        analyses.set(source.id, {
            id: source.id,
            effectiveShortcut,
            normalizedShortcut,
            bindingShortcut: effectiveShortcut,
            issue: null,
        });
    }

    for (const [normalizedShortcut, owners] of ownersByShortcut.entries()) {
        if (owners.length < 2) {
            continue;
        }
        for (const ownerId of owners) {
            const current = analyses.get(ownerId);
            if (!current) {
                continue;
            }
            analyses.set(ownerId, {
                ...current,
                normalizedShortcut,
                bindingShortcut: null,
                issue: {kind: "duplicate-view", conflictingViewIds: [...owners]},
            });
        }
    }

    return analyses;
}
