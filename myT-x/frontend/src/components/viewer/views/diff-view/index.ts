import { registerView } from "../../viewerRegistry";
import { DiffIcon } from "../../icons/DiffIcon";
import { DiffView } from "./DiffView";

registerView({
  id: "diff",
  icon: DiffIcon,
  label: "Diff",
  component: DiffView,
  shortcut: "Ctrl+Shift+D",
  position: "bottom",
});
