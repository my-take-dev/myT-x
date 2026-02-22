import {registerView} from "../../viewerRegistry";
import {ErrorLogIcon} from "../../icons/ErrorLogIcon";
import {ErrorLogView} from "./ErrorLogView";

registerView({
    id: "error-log",
    icon: ErrorLogIcon,
    label: "Error Log",
    component: ErrorLogView,
    shortcut: "Ctrl+Shift+L",
    position: "bottom",
});
