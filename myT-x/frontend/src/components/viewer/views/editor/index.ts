import {registerView} from "../../viewerRegistry";
import {mustGetViewerShortcutDef} from "../../viewerShortcutDefinitions";
import {EditorIcon} from "../../icons/EditorIcon";
import {EditorView} from "./EditorView";

const shortcutDef = mustGetViewerShortcutDef("editor");

registerView({
    id: shortcutDef.viewId,
    icon: EditorIcon,
    label: shortcutDef.label,
    component: EditorView,
    shortcut: shortcutDef.defaultShortcut,
});
