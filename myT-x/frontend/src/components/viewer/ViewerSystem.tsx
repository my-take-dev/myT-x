import {useEffect, useMemo, useRef, useState} from "react";
import {ActivityStrip} from "./ActivityStrip";
import {ViewOverlay} from "./ViewOverlay";
import {useViewerStore} from "./viewerStore";
import {getRegisteredViews, subscribeRegistry} from "./viewerRegistry";
import {
    buildShortcutFromKeyboardEvent,
    getEffectiveViewerShortcut,
    hasShortcutModifier,
    normalizeShortcut,
} from "./viewerShortcutUtils";
import {useTmuxStore} from "../../stores/tmuxStore";
import {useNotificationStore} from "../../stores/notificationStore";
import {isImeTransitionalEvent} from "../../utils/ime";

// Side-effect imports: each view self-registers into the registry.
//
// 【サイドバーアイコンの配置ルール】
// 1. エラーログ表示以外のアイコンは、上から表示させたい順にここで import してください。
// 2. エラーログ表示 (error-log) は必ず一番下に配置してください。
// 3. エラーログ表示の下には絶対に新しいアイコンを追加（import）しないでください。
// 4. 新しいビューを追加する場合、position フィールドは省略してください（= "top" がデフォルト）。
// 5. position: "bottom" は error-log 専用です。他のビューには使用しないでください。

import "./views/file-tree";
import "./views/git-graph";
import "./views/diff-view";
import "./views/input-history"; // DIFFの直下に配置
import "./views/mcp-manager";

// ---------------------------------------------------------
// これより下にはエラーログ表示以外のアイコンを追加しないこと
// ---------------------------------------------------------
import "./views/error-log";

export function ViewerSystem() {
    const activeViewId = useViewerStore((s) => s.activeViewId);
    const toggleView = useViewerStore((s) => s.toggleView);
    const closeView = useViewerStore((s) => s.closeView);
    const [registryVersion, setRegistryVersion] = useState(0);

    // HMR/runtime-safe registry updates: rebuild memoized view/shortcut state when
    // view modules re-register.
    useEffect(() => {
        return subscribeRegistry(() => {
            setRegistryVersion((version) => version + 1);
        });
    }, []);

    const views = useMemo(() => getRegisteredViews(), [registryVersion]);
    const viewerShortcutsConfig = useTmuxStore((s) => s.config?.viewer_shortcuts ?? null);

    const addNotification = useNotificationStore((s) => s.addNotification);

    // Shortcut map: config overrides take priority over registry defaults.
    const {shortcutMap, duplicateWarnings} = useMemo(() => {
        const map = new Map<string, string>();
        const duplicates: string[] = [];
        for (const view of views) {
            const effectiveShortcut = getEffectiveViewerShortcut(
                viewerShortcutsConfig?.[view.id],
                view.shortcut,
            );
            if (!effectiveShortcut) {
                continue;
            }
            if (!hasShortcutModifier(effectiveShortcut)) {
                if (import.meta.env.DEV) {
                    console.warn(`[viewer-shortcut] ignored non-modifier shortcut for "${view.id}": "${effectiveShortcut}"`);
                }
                continue;
            }
            const normalized = normalizeShortcut(effectiveShortcut);
            if (!normalized) {
                continue;
            }
            const existingOwner = map.get(normalized);
            if (existingOwner) {
                if (import.meta.env.DEV) {
                    console.warn(
                        `[viewer-shortcut] duplicate shortcut "${normalized}" for "${existingOwner}" and "${view.id}"; keeping first`,
                    );
                }
                duplicates.push(
                    `ショートカット "${effectiveShortcut}" が "${existingOwner}" と "${view.id}" で重複しています`,
                );
                continue;
            }
            map.set(normalized, view.id);
        }
        return {shortcutMap: map, duplicateWarnings: duplicates};
    }, [views, viewerShortcutsConfig]);

    // Track previously notified duplicates to avoid re-firing the same
    // notifications when useMemo produces a new array reference with
    // identical content (e.g. on unrelated registry/config updates).
    const prevDuplicateKeyRef = useRef("");
    useEffect(() => {
        const key = duplicateWarnings.join("\n");
        const prevKey = prevDuplicateKeyRef.current;
        prevDuplicateKeyRef.current = key;
        if (key === "" || key === prevKey) {
            return;
        }
        for (const warning of duplicateWarnings) {
            addNotification(warning, "warn");
        }
    }, [duplicateWarnings, addNotification]);

    useEffect(() => {
        const handler = (e: KeyboardEvent) => {
            if (e.defaultPrevented) return;
            if (isImeTransitionalEvent(e)) return;

            if (e.key === "Escape" && activeViewId !== null) {
                e.preventDefault();
                closeView();
                return;
            }

            const combo = buildShortcutFromKeyboardEvent(e);
            if (combo === "") {
                return;
            }
            const viewId = shortcutMap.get(combo);
            if (viewId) {
                e.preventDefault();
                toggleView(viewId);
            }
        };

        window.addEventListener("keydown", handler);
        return () => window.removeEventListener("keydown", handler);
    }, [activeViewId, toggleView, closeView, shortcutMap]);

    if (views.length === 0) {
        return null;
    }

    return (
        <>
            <ActivityStrip/>
            <ViewOverlay/>
        </>
    );
}
