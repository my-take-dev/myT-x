import React, {useSyncExternalStore} from "react";
import {useTmuxStore} from "../../stores/tmuxStore";
import type {ViewPlugin} from "./viewerRegistry";
import {getRegisteredViews} from "./viewerRegistry";
import {getEffectiveViewerShortcut} from "./viewerShortcutUtils";
import {useViewerStore} from "./viewerStore";

const EMPTY_UNSUBSCRIBE = () => {
};
const EMPTY_SUBSCRIBE = () => EMPTY_UNSUBSCRIBE;
const ZERO_BADGE = () => 0;

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
    const views = getRegisteredViews();
    const activeViewId = useViewerStore((s) => s.activeViewId);
    const toggleView = useViewerStore((s) => s.toggleView);
    const viewerShortcutsConfig = useTmuxStore((s) => s.config?.viewer_shortcuts ?? null);
    const topViews = views.filter((view) => !isBottomView(view));
    const bottomViews = views.filter(isBottomView);

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
        </div>
    );
}
