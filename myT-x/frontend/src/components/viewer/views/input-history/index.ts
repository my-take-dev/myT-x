import {registerView} from "../../viewerRegistry";
import {mustGetViewerShortcutDef} from "../../viewerShortcutDefinitions";
import {InputHistoryIcon} from "../../icons/InputHistoryIcon";
import {InputHistoryView} from "./InputHistoryView";
import {useInputHistoryStore} from "../../../../stores/inputHistoryStore";

const shortcutDef = mustGetViewerShortcutDef("input-history");

registerView({
    id: shortcutDef.viewId,
    icon: InputHistoryIcon,
    label: shortcutDef.label,
    component: InputHistoryView,
    shortcut: shortcutDef.defaultShortcut,
    getBadgeCount: () => useInputHistoryStore.getState().unreadCount,
    subscribeBadgeCount: (listener) =>
        useInputHistoryStore.subscribe((state, prevState) => {
            if (state.unreadCount !== prevState.unreadCount) {
                listener();
            }
        }),
});
