import { registerView } from "../../viewerRegistry";
import { GitGraphIcon } from "../../icons/GitGraphIcon";
import { GitGraphView } from "./GitGraphView";

registerView({
  id: "git-graph",
  icon: GitGraphIcon,
  label: "Git Graph",
  component: GitGraphView,
  shortcut: "Ctrl+Shift+G",
});
