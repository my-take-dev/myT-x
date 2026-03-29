import React, {useCallback, useRef, useState, useSyncExternalStore} from "react";
import {api} from "../../api";
import {useI18n} from "../../i18n";
import {useTmuxStore} from "../../stores/tmuxStore";
import {notifyAndLog} from "../../utils/notifyUtils";
import {normalizeViewerSidebarMode} from "../../utils/viewerSidebarMode";
import type {ViewPlugin} from "./viewerRegistry";
import {getRegisteredViews} from "./viewerRegistry";
import {getEffectiveViewerShortcut} from "./viewerShortcutUtils";
import {useViewerStore} from "./viewerStore";

const EMPTY_UNSUBSCRIBE = () => {
};
const EMPTY_SUBSCRIBE = () => EMPTY_UNSUBSCRIBE;
const ZERO_BADGE = () => 0;
const DOCK_ICON = "\u229E"; // ⊞ dock
const OVERLAY_ICON = "\u229F"; // ⊟ undock

function isBottomView(view: ViewPlugin): boolean {
    return view.position === "bottom";
}

interface ActivityButtonProps {
    view: ViewPlugin;
    isActive: boolean;
    viewerShortcutsConfig: Record<string, string> | null;
    onToggle: (viewID: string) => void;
}

const ActivityButton = React.memo(function ActivityButton({
                                                              view,
                                                              isActive,
                                                              viewerShortcutsConfig,
                                                              onToggle
                                                          }: ActivityButtonProps) {
    const Icon = view.icon;
    const effectiveShortcut = getEffectiveViewerShortcut(
        viewerShortcutsConfig?.[view.id],
        view.shortcut,
    );
    const subscribeBadge = view.subscribeBadgeCount ?? EMPTY_SUBSCRIBE;
    const getBadgeCount = view.getBadgeCount ?? ZERO_BADGE;
    const badgeCount = useSyncExternalStore(subscribeBadge, getBadgeCount, getBadgeCount);

    return (
        <button
            className={`viewer-strip-btn${isActive ? " active" : ""}`}
            onClick={() => onToggle(view.id)}
            title={effectiveShortcut ? `${view.label} (${effectiveShortcut})` : view.label}
        >
            <Icon size={18}/>
            {badgeCount > 0 && (
                <span className="viewer-strip-badge"/>
            )}
        </button>
    );
});

export function ActivityStrip() {
    const {language, t} = useI18n();
    const views = getRegisteredViews();
    const activeViewId = useViewerStore((s) => s.activeViewId);
    const toggleView = useViewerStore((s) => s.toggleView);
    const viewerShortcutsConfig = useTmuxStore((s) => s.config?.viewer_shortcuts ?? null);
    const sidebarMode = useTmuxStore((s) => normalizeViewerSidebarMode(s.config?.viewer_sidebar_mode));
    const isDocked = sidebarMode === "docked";
    const toggleInFlightRef = useRef(false);
    const [isTogglingSidebarMode, setIsTogglingSidebarMode] = useState(false);
    const topViews = views.filter((view) => !isBottomView(view));
    const bottomViews = views.filter(isBottomView);

    const handleToggleSidebarMode = useCallback(async () => {
        if (toggleInFlightRef.current) {
            return;
        }
        toggleInFlightRef.current = true;
        setIsTogglingSidebarMode(true);
        try {
            await api.ToggleViewerSidebarMode();
        } catch (err) {
            console.warn("[activity-strip] toggle sidebar mode failed", err);
            notifyAndLog("Toggle sidebar mode", "warn", err, "ActivityStrip");
        } finally {
            toggleInFlightRef.current = false;
            setIsTogglingSidebarMode(false);
        }
    }, [isDocked]);

    const toggleTitle = t(
        "viewer.activityStrip.toggleSidebarMode",
        language === "ja"
            ? (isDocked ? "オーバーレイ表示に切替" : "ドッキング表示に切替")
            : (isDocked ? "Switch to overlay view" : "Switch to docked view"),
    );

    if (views.length === 0) {
        return null;
    }

    return (
        <div className="viewer-activity-strip">
            {topViews.map((view) => (
                <ActivityButton
                    key={view.id}
                    view={view}
                    isActive={activeViewId === view.id}
                    viewerShortcutsConfig={viewerShortcutsConfig}
                    onToggle={toggleView}
                />
            ))}
            {bottomViews.length > 0 && <div className="viewer-strip-spacer"/>}
            {bottomViews.map((view) => (
                <ActivityButton
                    key={view.id}
                    view={view}
                    isActive={activeViewId === view.id}
                    viewerShortcutsConfig={viewerShortcutsConfig}
                    onToggle={toggleView}
                />
            ))}
            <button
                type="button"
                className="viewer-strip-btn"
                onClick={handleToggleSidebarMode}
                title={toggleTitle}
                aria-label={toggleTitle}
                disabled={isTogglingSidebarMode}
            >
                {isDocked ? OVERLAY_ICON : DOCK_ICON}
            </button>
        </div>
    );
}
