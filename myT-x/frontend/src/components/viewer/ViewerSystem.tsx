import {useEffect, useMemo, useRef} from "react";
import {useI18n} from "../../i18n";
import {useNotificationStore} from "../../stores/notificationStore";
import {useTmuxStore} from "../../stores/tmuxStore";
import {isImeTransitionalEvent} from "../../utils/ime";
import {ActivityStrip} from "./ActivityStrip";
import {DockedDivider} from "./DockedDivider";
import {ViewOverlay} from "./ViewOverlay";
import {getViewerShortcutValue} from "./viewerShortcutDefinitions";
import {getReservedViewerShortcutLabel} from "./viewerReservedShortcuts";
import {analyzeViewerShortcuts} from "./viewerShortcutAnalysis";
import {useIsViewerDocked} from "./useIsViewerDocked";
import {useRegisteredViews} from "./useRegisteredViews";
import {useViewerStore} from "./viewerStore";
import {buildShortcutFromKeyboardEvent, formatShortcutForDisplay} from "./viewerShortcutUtils";

// Side-effect imports: each view self-registers into the registry.
//
// 【サイドバーアイコンの配置ルール】
// 1. エラーログ表示以外のアイコンは、上から表示させたい順にここで import してください。
// 2. エラーログ表示 (error-log) は必ず一番下に配置してください。
// 3. エラーログ表示の下には絶対に新しいアイコンを追加（import）しないでください。
// 4. 新しいビューを追加する場合、position フィールドは省略してください（= "top" がデフォルト）。
// 5. position: "bottom" は error-log 専用です。他のビューには使用しないでください。

import "./views/file-tree";
import "./views/editor";
import "./views/git-graph";
import "./views/diff-view";
import "./views/input-history"; // DIFFの直下に配置
import "./views/mcp-manager";
import "./views/pane-scheduler";
import "./views/prompt-presets";
import "./views/orchestrator-teams";
import "./views/single-task-runner";
import "./views/task-scheduler";
import "./views/usage-dashboard";

// ---------------------------------------------------------
// これより下にはエラーログ表示以外のアイコンを追加しないこと
// ---------------------------------------------------------
import "./views/error-log";

export function ViewerSystem() {
    const {language, t} = useI18n();
    const activeViewId = useViewerStore((s) => s.activeViewId);
    const toggleView = useViewerStore((s) => s.toggleView);
    const closeView = useViewerStore((s) => s.closeView);
    const views = useRegisteredViews();
    const viewerShortcutsConfig = useTmuxStore((s) => s.config?.viewer_shortcuts ?? null);

    const addNotification = useNotificationStore((s) => s.addNotification);
    const isDocked = useIsViewerDocked();

    // Shortcut map: config overrides take priority over registry defaults.
    const {shortcutMap, shortcutWarnings} = useMemo(() => {
        const map = new Map<string, string>();
        const warnings: string[] = [];
        const analyses = analyzeViewerShortcuts(
            views.map((view) => ({
                id: view.id,
                configuredShortcut: getViewerShortcutValue(viewerShortcutsConfig, view.id),
                defaultShortcut: view.shortcut,
            })),
        );
        const warnedDuplicateGroups = new Set<string>();

        for (const view of views) {
            const analysis = analyses.get(view.id);
            if (!analysis || !analysis.effectiveShortcut) {
                continue;
            }
            if (analysis.issue?.kind === "modifier-required") {
                if (import.meta.env.DEV) {
                    console.warn(`[viewer-shortcut] ignored non-modifier shortcut for "${view.id}": "${analysis.effectiveShortcut}"`);
                }
                continue;
            }
            if (analysis.issue?.kind === "reserved") {
                const reservedShortcutLabel = getReservedViewerShortcutLabel(analysis.issue.reservedShortcut, language);
                warnings.push(
                    t(
                        "viewer.shortcuts.reserved",
                        `ショートカット "{shortcut}" は "{label}" で予約済みです`,
                        {shortcut: formatShortcutForDisplay(analysis.effectiveShortcut), label: reservedShortcutLabel},
                    ),
                );
                continue;
            }
            if (analysis.issue?.kind === "duplicate-view") {
                const duplicateKey = analysis.issue.conflictingViewIds.join("|");
                if (!warnedDuplicateGroups.has(duplicateKey)) {
                    warnedDuplicateGroups.add(duplicateKey);
                    if (import.meta.env.DEV) {
                        console.warn(
                            `[viewer-shortcut] duplicate shortcut "${analysis.normalizedShortcut}" for ${analysis.issue.conflictingViewIds.join(", ")}; disabling all`,
                        );
                    }
                    const [ownerA = view.id, ownerB = view.id] = analysis.issue.conflictingViewIds;
                    warnings.push(
                        t(
                            "viewer.shortcuts.duplicate",
                            `ショートカット "{shortcut}" が "{ownerA}" と "{ownerB}" で重複しています`,
                            {shortcut: formatShortcutForDisplay(analysis.effectiveShortcut), ownerA, ownerB},
                        ),
                    );
                }
                continue;
            }
            if (!analysis.bindingShortcut) {
                continue;
            }
            map.set(analysis.normalizedShortcut, view.id);
        }
        return {shortcutMap: map, shortcutWarnings: warnings};
    }, [views, viewerShortcutsConfig, language, t]);

    // Track previously notified duplicates to avoid re-firing the same
    // notifications when useMemo produces a new array reference with
    // identical content (e.g. on unrelated registry/config updates).
    const prevDuplicateKeyRef = useRef("");
    useEffect(() => {
        const key = shortcutWarnings.join("\n");
        const prevKey = prevDuplicateKeyRef.current;
        prevDuplicateKeyRef.current = key;
        if (key === "" || key === prevKey) {
            return;
        }
        for (const warning of shortcutWarnings) {
            addNotification(warning, "warn");
        }
    }, [shortcutWarnings, addNotification]);

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
            {isDocked && <DockedDivider/>}
            <ViewOverlay/>
        </>
    );
}
