import {registerView} from "../../viewerRegistry";
import {mustGetViewerShortcutDef} from "../../viewerShortcutDefinitions";
import {McpIcon} from "../../icons/McpIcon";
import {McpManagerView} from "./McpManagerView";

// Side-effect only: registers the MCP Manager view into viewerRegistry.
const shortcutDef = mustGetViewerShortcutDef("mcp-manager");

registerView({
    id: shortcutDef.viewId,
    icon: McpIcon,
    label: shortcutDef.label,
    component: McpManagerView,
    shortcut: shortcutDef.defaultShortcut,
});
