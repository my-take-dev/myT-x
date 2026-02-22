import {useCallback} from "react";
import {useErrorLogStore} from "../../stores/errorLogStore";
import type {ViewPlugin} from "./viewerRegistry";
import {getRegisteredViews} from "./viewerRegistry";
import {useViewerStore} from "./viewerStore";

function isBottomView(view: ViewPlugin): boolean {
    return view.position === "bottom";
}

export function ActivityStrip() {
    const views = getRegisteredViews();
    const activeViewId = useViewerStore((s) => s.activeViewId);
    const toggleView = useViewerStore((s) => s.toggleView);
    const unreadCount = useErrorLogStore((s) => s.unreadCount);
    const topViews = views.filter((view) => !isBottomView(view));
    const bottomViews = views.filter(isBottomView);

    const renderViewButton = useCallback((view: ViewPlugin) => {
        const Icon = view.icon;
        const isActive = activeViewId === view.id;

        return (
            <button
                key={view.id}
                className={`viewer-strip-btn${isActive ? " active" : ""}`}
                onClick={() => toggleView(view.id)}
                title={view.shortcut ? `${view.label} (${view.shortcut})` : view.label}
            >
                <Icon size={18}/>
                {view.id === "error-log" && unreadCount > 0 && (
                    <span className="viewer-strip-badge"/>
                )}
            </button>
        );
    }, [activeViewId, toggleView, unreadCount]);

    if (views.length === 0) {
        return null;
    }

    return (
        <div className="viewer-activity-strip">
            {topViews.map(renderViewButton)}
            {bottomViews.length > 0 && <div className="viewer-strip-spacer"/>}
            {bottomViews.map(renderViewButton)}
        </div>
    );
}
