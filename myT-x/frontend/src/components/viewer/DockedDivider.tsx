import {useEffect, useRef} from "react";
import {useViewerStore} from "./viewerStore";

export function DockedDivider() {
    const setDockRatio = useViewerStore((s) => s.setDockRatio);
    const resetDockRatio = useViewerStore((s) => s.resetDockRatio);
    // Safety net: if the component unmounts mid-drag, clean up global listeners.
    const dragCleanupRef = useRef<(() => void) | null>(null);

    useEffect(() => {
        return () => {
            dragCleanupRef.current?.();
        };
    }, []);

    const startDrag = (divider: HTMLDivElement) => {
        const parent = divider.closest(".app-body");
        if (!(parent instanceof HTMLElement)) {
            return;
        }
        const initialUserSelect = document.body.style.userSelect;

        parent.classList.add("app-body--dragging");
        document.body.style.userSelect = "none";

        const onMove = (event: MouseEvent) => {
            const rect = parent.getBoundingClientRect();
            if (rect.width <= 0) {
                return;
            }
            setDockRatio((event.clientX - rect.left) / rect.width);
        };
        const cleanup = () => {
            window.removeEventListener("mousemove", onMove);
            window.removeEventListener("mouseup", onUp);
            window.removeEventListener("blur", onUp);
            parent.classList.remove("app-body--dragging");
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

    return (
        <div
            className="docked-divider"
            onMouseDown={(e) => {
                e.preventDefault();
                startDrag(e.currentTarget);
            }}
            onDoubleClick={(e) => {
                e.preventDefault();
                e.stopPropagation();
                resetDockRatio();
            }}
        />
    );
}
