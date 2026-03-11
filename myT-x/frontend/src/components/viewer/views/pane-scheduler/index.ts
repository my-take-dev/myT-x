import {registerView} from "../../viewerRegistry";
import {mustGetViewerShortcutDef} from "../../viewerShortcutDefinitions";
import {SchedulerIcon} from "../../icons/SchedulerIcon";
import {PaneSchedulerView} from "./PaneSchedulerView";

const shortcutDef = mustGetViewerShortcutDef("pane-scheduler");

registerView({
    id: shortcutDef.viewId,
    icon: SchedulerIcon,
    label: shortcutDef.label,
    component: PaneSchedulerView,
    shortcut: shortcutDef.defaultShortcut,
});
