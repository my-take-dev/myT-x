import { registerView } from "../../viewerRegistry";
import {mustGetViewerShortcutDef} from "../../viewerShortcutDefinitions";
import { GitGraphIcon } from "../../icons/GitGraphIcon";
import { GitGraphView } from "./GitGraphView";

const shortcutDef = mustGetViewerShortcutDef("git-graph");

registerView({
  id: shortcutDef.viewId,
  icon: GitGraphIcon,
  label: shortcutDef.label,
  component: GitGraphView,
  shortcut: shortcutDef.defaultShortcut,
});
