import {registerView} from "../../viewerRegistry";
import {mustGetViewerShortcutDef} from "../../viewerShortcutDefinitions";
import {ErrorLogIcon} from "../../icons/ErrorLogIcon";
import {ErrorLogView} from "./ErrorLogView";
import {useErrorLogStore} from "../../../../stores/errorLogStore";

const shortcutDef = mustGetViewerShortcutDef("error-log");

registerView({
    id: shortcutDef.viewId,
    icon: ErrorLogIcon,
    label: shortcutDef.label,
    component: ErrorLogView,
    shortcut: shortcutDef.defaultShortcut,
    position: "bottom",
    getBadgeCount: () => useErrorLogStore.getState().unreadCount,
    subscribeBadgeCount: (listener) =>
        useErrorLogStore.subscribe((state, prevState) => {
            if (state.unreadCount !== prevState.unreadCount) {
                listener();
            }
        }),
});
