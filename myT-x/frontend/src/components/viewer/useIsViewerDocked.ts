import {useTmuxStore} from "../../stores/tmuxStore";
import {normalizeViewerSidebarMode} from "../../utils/viewerSidebarMode";
import {useViewerStore} from "./viewerStore";
import {isViewerDocked} from "./viewerDocking";

export function useIsViewerDocked(): boolean {
    const activeViewId = useViewerStore((s) => s.activeViewId);
    const viewerSidebarMode = useTmuxStore((s) => normalizeViewerSidebarMode(s.config?.viewer_sidebar_mode));
    return isViewerDocked(viewerSidebarMode, activeViewId);
}
