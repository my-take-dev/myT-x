import {useCallback, useEffect, useRef} from "react";
import {DOCK_RATIO_MAX, DOCK_RATIO_MIN, computeDockedLayoutMetrics, normalizeDockedViewportWidth} from "./viewerDocking";
import {useViewerStore} from "./viewerStore";

const KEYBOARD_STEP = 0.02;

export function DockedDivider() {
    const dockRatio = useViewerStore((s) => s.dockRatio);
    const setDockRatio = useViewerStore((s) => s.setDockRatio);
    const resetDockRatio = useViewerStore((s) => s.resetDockRatio);
    // Safety net: if the component unmounts mid-drag, clean up global listeners.
    const dragCleanupRef = useRef<(() => void) | null>(null);

    useEffect(() => {
        return () => {
            dragCleanupRef.current?.();
        };
    }, []);

    // Convert viewport-space mouse delta to an unscaled dock ratio using the
    // visible content span as the denominator.
    const startDrag = (divider: HTMLDivElement, startX: number) => {
        const appBody = divider.closest(".app-body");
        if (!(appBody instanceof HTMLElement)) {
            console.error("[DockedDivider] BUG: .app-body not found in ancestor chain");
            return;
        }
        // NOTE: transform is applied to .app-body__inner, not .app-body itself,
        // so getBoundingClientRect() still reports the unscaled viewport width.
        const measuredViewportWidth = appBody.getBoundingClientRect().width;
        if (measuredViewportWidth <= 0) {
            console.error("[DockedDivider] BUG: appBody viewport width is", measuredViewportWidth);
            return;
        }
        const viewportWidth = normalizeDockedViewportWidth(measuredViewportWidth);
        const {displayedContentWidth} = computeDockedLayoutMetrics(viewportWidth);
        if (displayedContentWidth <= 0) {
            console.error("[DockedDivider] BUG: displayedContentWidth <= 0 during drag start");
            return;
        }
        const startRatio = useViewerStore.getState().dockRatio;
        const initialUserSelect = document.body.style.userSelect;

        appBody.classList.add("app-body--dragging");
        document.body.style.userSelect = "none";

        const onMove = (event: MouseEvent) => {
            setDockRatio(startRatio + (event.clientX - startX) / displayedContentWidth);
        };
        const cleanup = () => {
            window.removeEventListener("mousemove", onMove);
            window.removeEventListener("mouseup", onUp);
            window.removeEventListener("blur", onUp);
            appBody.classList.remove("app-body--dragging");
            document.body.style.userSelect = initialUserSelect;
        };
        const onUp = () => {
            cleanup();
            if (dragCleanupRef.current === cleanup) {
                dragCleanupRef.current = null;
            }
        };
        if (dragCleanupRef.current) {
            dragCleanupRef.current();
        }
        dragCleanupRef.current = cleanup;
        window.addEventListener("mousemove", onMove);
        window.addEventListener("mouseup", onUp);
        window.addEventListener("blur", onUp, {once: true});
    };

    const handleKeyDown = useCallback(
        (e: React.KeyboardEvent<HTMLDivElement>) => {
            switch (e.key) {
                case "ArrowLeft":
                    e.preventDefault();
                    setDockRatio(useViewerStore.getState().dockRatio - KEYBOARD_STEP);
                    break;
                case "ArrowRight":
                    e.preventDefault();
                    setDockRatio(useViewerStore.getState().dockRatio + KEYBOARD_STEP);
                    break;
                case "Home":
                    e.preventDefault();
                    setDockRatio(DOCK_RATIO_MIN);
                    break;
                case "End":
                    e.preventDefault();
                    setDockRatio(DOCK_RATIO_MAX);
                    break;
                default:
                    break;
            }
        },
        [setDockRatio],
    );

    return (
        <div
            className="docked-divider"
            role="separator"
            aria-orientation="vertical"
            aria-label="Resize panels"
            aria-valuemin={0}
            aria-valuemax={100}
            aria-valuenow={Math.round(dockRatio * 100)}
            tabIndex={0}
            onMouseDown={(e) => {
                e.preventDefault();
                startDrag(e.currentTarget, e.clientX);
            }}
            onDoubleClick={(e) => {
                e.preventDefault();
                e.stopPropagation();
                resetDockRatio();
            }}
            onKeyDown={handleKeyDown}
        />
    );
}
