import {registerView} from "../../viewerRegistry";
import {mustGetViewerShortcutDef} from "../../viewerShortcutDefinitions";
import {SingleTaskRunnerIcon} from "../../icons/SingleTaskRunnerIcon";
import {SingleTaskRunnerView} from "./SingleTaskRunnerView";

const shortcutDef = mustGetViewerShortcutDef("single-task-runner");

registerView({
    id: shortcutDef.viewId,
    icon: SingleTaskRunnerIcon,
    label: shortcutDef.label,
    component: SingleTaskRunnerView,
    shortcut: shortcutDef.defaultShortcut,
});
