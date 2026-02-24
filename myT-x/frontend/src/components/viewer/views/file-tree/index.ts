import { registerView } from "../../viewerRegistry";
import {mustGetViewerShortcutDef} from "../../viewerShortcutDefinitions";
import { FileTreeIcon } from "../../icons/FileTreeIcon";
import { FileTreeView } from "./FileTreeView";

const shortcutDef = mustGetViewerShortcutDef("file-tree");

registerView({
  id: shortcutDef.viewId,
  icon: FileTreeIcon,
  label: shortcutDef.label,
  component: FileTreeView,
  shortcut: shortcutDef.defaultShortcut,
});
