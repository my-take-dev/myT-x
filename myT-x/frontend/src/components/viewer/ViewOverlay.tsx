import {ErrorBoundary} from "../ErrorBoundary";
import {useRegisteredViews} from "./useRegisteredViews";
import {useViewerStore} from "./viewerStore";

export function ViewOverlay() {
  const activeViewId = useViewerStore((s) => s.activeViewId);
  const views = useRegisteredViews();

  if (activeViewId === null) {
    return null;
  }

  const activeView = views.find((v) => v.id === activeViewId);
  if (!activeView) {
    return null;
  }

  const ViewComponent = activeView.component;

  return (
    <div className="viewer-overlay">
      <ErrorBoundary key={activeView.id}>
        <ViewComponent />
      </ErrorBoundary>
    </div>
  );
}
