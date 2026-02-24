import { registerView } from "../../viewerRegistry";
import {mustGetViewerShortcutDef} from "../../viewerShortcutDefinitions";
import { DiffIcon } from "../../icons/DiffIcon";
import { DiffView } from "./DiffView";

const shortcutDef = mustGetViewerShortcutDef("diff");

registerView({
  id: shortcutDef.viewId,
  icon: DiffIcon,
  label: shortcutDef.label,
  component: DiffView,
  shortcut: shortcutDef.defaultShortcut,
});
