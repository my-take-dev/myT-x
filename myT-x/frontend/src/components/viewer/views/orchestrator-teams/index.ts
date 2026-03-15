import {registerView} from "../../viewerRegistry";
import {mustGetViewerShortcutDef} from "../../viewerShortcutDefinitions";
import {OrchestratorTeamIcon} from "../../icons/OrchestratorTeamIcon";
import {OrchestratorTeamsView} from "./OrchestratorTeamsView";

const shortcutDef = mustGetViewerShortcutDef("orchestrator-teams");

registerView({
    id: shortcutDef.viewId,
    icon: OrchestratorTeamIcon,
    label: shortcutDef.label,
    component: OrchestratorTeamsView,
    shortcut: shortcutDef.defaultShortcut,
});
