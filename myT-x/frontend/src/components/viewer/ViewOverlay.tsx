import { getRegisteredViews } from "./viewerRegistry";
import { useViewerStore } from "./viewerStore";

export function ViewOverlay() {
  const activeViewId = useViewerStore((s) => s.activeViewId);

  if (activeViewId === null) {
    return null;
  }

  const views = getRegisteredViews();
  const activeView = views.find((v) => v.id === activeViewId);
  if (!activeView) {
    return null;
  }

  const ViewComponent = activeView.component;

  return (
    <div className="viewer-overlay">
      <ViewComponent />
    </div>
  );
}
