import {PromptPresetsIcon} from "../../icons/PromptPresetsIcon";
import {registerView} from "../../viewerRegistry";
import {mustGetViewerShortcutDef} from "../../viewerShortcutDefinitions";
import {PromptPresetsView} from "./PromptPresetsView";

const shortcutDef = mustGetViewerShortcutDef("prompt-presets");

registerView({
    id: shortcutDef.viewId,
    icon: PromptPresetsIcon,
    label: shortcutDef.label,
    component: PromptPresetsView,
    shortcut: shortcutDef.defaultShortcut,
});
