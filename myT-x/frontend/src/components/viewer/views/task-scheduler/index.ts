import {registerView} from "../../viewerRegistry";
import {mustGetViewerShortcutDef} from "../../viewerShortcutDefinitions";
import {OrchestratorTaskIcon} from "../../icons/OrchestratorTaskIcon";
import {TaskSchedulerView} from "./TaskSchedulerView";

const shortcutDef = mustGetViewerShortcutDef("task-scheduler");

registerView({
    id: shortcutDef.viewId,
    icon: OrchestratorTaskIcon,
    label: shortcutDef.label,
    component: TaskSchedulerView,
    shortcut: shortcutDef.defaultShortcut,
});
