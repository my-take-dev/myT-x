import {registerView} from "../../viewerRegistry";
import {mustGetViewerShortcutDef} from "../../viewerShortcutDefinitions";
import {UsageDashboardIcon} from "../../icons/UsageDashboardIcon";
import {UsageDashboardView} from "./UsageDashboardView";

const shortcutDef = mustGetViewerShortcutDef("usage-dashboard");

registerView({
    id: shortcutDef.viewId,
    icon: UsageDashboardIcon,
    label: shortcutDef.label,
    component: UsageDashboardView,
    shortcut: shortcutDef.defaultShortcut,
});
