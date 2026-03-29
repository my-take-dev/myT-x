import {registerView} from "../../viewerRegistry";
import {mustGetViewerShortcutDef} from "../../viewerShortcutDefinitions";
import {TaskSchedulerIcon} from "../../icons/TaskSchedulerIcon";
import {TaskSchedulerView} from "./TaskSchedulerView";

const shortcutDef = mustGetViewerShortcutDef("task-scheduler");

registerView({
    id: shortcutDef.viewId,
    icon: TaskSchedulerIcon,
    label: shortcutDef.label,
    component: TaskSchedulerView,
    shortcut: shortcutDef.defaultShortcut,
});
