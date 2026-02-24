import type {ComponentType} from "react";
import {normalizeShortcut} from "./viewerShortcutUtils";

type ViewPosition = "top" | "bottom";

export interface ViewPlugin {
    id: string;
    icon: ComponentType<{ size?: number }>;
    label: string;
    component: ComponentType;
    /** Keyboard shortcut in format "Ctrl+Shift+X". Used by ViewerSystem for dynamic binding. */
    shortcut?: string;
    /**
     * Vertical placement in the ActivityStrip sidebar.
     * - Omit (or use "top") for standard views - displayed in import order.
     * - "bottom" is reserved exclusively for error-log. Using it on other views
     *   places them below the spacer, which breaks the intended icon order.
     */
    position?: ViewPosition;
    /** Optional unread indicator source for ActivityStrip badges. */
    getBadgeCount?: () => number;
    /** Optional unread indicator subscription for reactive ActivityStrip badges. */
    subscribeBadgeCount?: (listener: () => void) => () => void;
}

const REGISTRY_KEY = Symbol.for("mytx.viewer.registry");

type RegistryState = {
    plugins: ViewPlugin[];
    listeners: Set<() => void>;
};

type RegistryGlobal = typeof globalThis & {
    [key: symbol]: unknown;
};

function resolveRegistryState(): RegistryState {
    const globalObject = globalThis as RegistryGlobal;
    const existing = globalObject[REGISTRY_KEY] as RegistryState | undefined;
    if (!existing) {
        globalObject[REGISTRY_KEY] = {
            plugins: [],
            listeners: new Set(),
        };
    }
    return globalObject[REGISTRY_KEY] as RegistryState;
}

const registryState = resolveRegistryState();

function notifyRegistryListeners(): void {
    for (const listener of registryState.listeners) {
        listener();
    }
}

function normalizePosition(position: unknown): ViewPosition {
    if (position === "bottom") {
        return "bottom";
    }
    if (position === "top" || position === undefined) {
        return "top";
    }
    if (import.meta.env.DEV) {
        console.warn(`[Registry] Invalid view position "${String(position)}", falling back to "top"`);
    }
    return "top";
}

if (import.meta.hot) {
    import.meta.hot.dispose(() => {
        registryState.plugins.length = 0;
        notifyRegistryListeners();
    });
}

export function registerView(plugin: ViewPlugin): void {
    const normalizedPlugin: ViewPlugin = {
        ...plugin,
        position: normalizePosition(plugin.position),
    };

    if (import.meta.env.DEV && plugin.shortcut) {
        const normalized = normalizeShortcut(plugin.shortcut);
        if (normalized !== "") {
            const existing = registryState.plugins.find((v) => {
                if (v.id === plugin.id || !v.shortcut) {
                    return false;
                }
                return normalizeShortcut(v.shortcut) === normalized;
            });
            if (existing) {
                console.warn(`[Registry] Duplicate shortcut "${plugin.shortcut}" between "${existing.id}" and "${plugin.id}"`);
            }
        }
    }
    if (import.meta.env.DEV) {
        if (plugin.getBadgeCount && !plugin.subscribeBadgeCount) {
            console.warn(`[Registry] View "${plugin.id}" provides getBadgeCount without subscribeBadgeCount; badge may not update reactively`);
        }
        if (!plugin.getBadgeCount && plugin.subscribeBadgeCount) {
            console.warn(`[Registry] View "${plugin.id}" provides subscribeBadgeCount without getBadgeCount`);
        }
    }
    const index = registryState.plugins.findIndex((existing) => existing.id === plugin.id);
    if (index >= 0) {
        registryState.plugins[index] = normalizedPlugin;
        notifyRegistryListeners();
        return;
    }
    registryState.plugins.push(normalizedPlugin);
    notifyRegistryListeners();
}

export function subscribeRegistry(listener: () => void): () => void {
    registryState.listeners.add(listener);
    return () => {
        registryState.listeners.delete(listener);
    };
}

// NOTE: Registry is still a module-scope singleton, but subscribeRegistry enables
// reactive consumers (e.g. ViewerSystem) to rebuild derived state after HMR updates.
export function getRegisteredViews(): readonly ViewPlugin[] {
    return registryState.plugins;
}

