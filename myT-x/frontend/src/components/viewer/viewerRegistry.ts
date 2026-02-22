import type {ComponentType} from "react";

type ViewPosition = "top" | "bottom";

export interface ViewPlugin {
    id: string;
    icon: ComponentType<{ size?: number }>;
    label: string;
    component: ComponentType;
    /** Keyboard shortcut in format "Ctrl+Shift+X". Used by ViewerSystem for dynamic binding. */
    shortcut?: string;
    position?: ViewPosition;
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
        const normalized = plugin.shortcut.toLowerCase();
        const existing = registryState.plugins.find(
            (v) => v.shortcut?.toLowerCase() === normalized && v.id !== plugin.id,
        );
        if (existing) {
            console.warn(`[Registry] Duplicate shortcut "${plugin.shortcut}" between "${existing.id}" and "${plugin.id}"`);
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
