import {SessionMemoIcon} from "../../icons/SessionMemoIcon";
import {registerView} from "../../viewerRegistry";
import {mustGetViewerShortcutDef} from "../../viewerShortcutDefinitions";
import {SessionMemoView} from "./SessionMemoView";

const shortcutDef = mustGetViewerShortcutDef("session-memo");

registerView({
    id: shortcutDef.viewId,
    icon: SessionMemoIcon,
    label: shortcutDef.label,
    component: SessionMemoView,
    shortcut: shortcutDef.defaultShortcut,
});
