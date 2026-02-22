import { registerView } from "../../viewerRegistry";
import { FileTreeIcon } from "../../icons/FileTreeIcon";
import { FileTreeView } from "./FileTreeView";

registerView({
  id: "file-tree",
  icon: FileTreeIcon,
  label: "File Tree",
  component: FileTreeView,
  shortcut: "Ctrl+Shift+E",
});
