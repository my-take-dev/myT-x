import {useEffect, useRef} from "react";
import type {AnchorPosition} from "./useChatResize";

interface ChatDividerProps {
    readonly anchor: AnchorPosition;
    readonly onRatioChange: (ratio: number) => void;
    readonly onReset: () => void;
}

// Compute the panel ratio from the mouse position relative to the layout rect.
// For top/left anchors, the panel grows from the layout origin (top-left corner).
// For bottom/right anchors, the panel grows from the opposite edge.
function ratioFromEvent(anchor: AnchorPosition, rect: DOMRect, event: MouseEvent): number {
    switch (anchor) {
        case "top":
            return (event.clientY - rect.top) / rect.height;
        case "bottom":
            return (rect.bottom - event.clientY) / rect.height;
        case "left":
            return (event.clientX - rect.left) / rect.width;
        case "right":
            return (rect.right - event.clientX) / rect.width;
        default: {
            const _exhaustive: never = anchor;
            console.warn("[ChatDivider] unknown anchor:", _exhaustive);
            return 0.5;
        }
    }
}

export function ChatDivider({anchor, onRatioChange, onReset}: ChatDividerProps) {
    const direction = anchor === "left" || anchor === "right" ? "horizontal" : "vertical";
    const dragCleanupRef = useRef<(() => void) | null>(null);

    useEffect(() => {
        return () => {
            dragCleanupRef.current?.();
        };
    }, []);

    const startDrag = (divider: HTMLDivElement) => {
        const layout = divider.closest(".chat-layout");
        if (!(layout instanceof HTMLElement)) {
            console.warn("[ChatDivider] .chat-layout not found in ancestor chain");
            return;
        }

        const initialUserSelect = document.body.style.userSelect;
        layout.classList.add("chat-layout--dragging");
        document.body.style.userSelect = "none";

        const onMove = (event: MouseEvent) => {
            const rect = layout.getBoundingClientRect();
            if (rect.width <= 0 || rect.height <= 0) {
                return;
            }
            onRatioChange(ratioFromEvent(anchor, rect, event));
        };

        const cleanup = () => {
            window.removeEventListener("mousemove", onMove);
            window.removeEventListener("mouseup", onUp);
            window.removeEventListener("blur", onUp);
            layout.classList.remove("chat-layout--dragging");
            document.body.style.userSelect = initialUserSelect;
            if (dragCleanupRef.current === cleanup) {
                dragCleanupRef.current = null;
            }
        };

        const onUp = () => {
            cleanup();
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
            className={`chat-divider chat-divider--${direction}`}
            onMouseDown={(event) => {
                event.preventDefault();
                startDrag(event.currentTarget);
            }}
            onDoubleClick={(event) => {
                event.preventDefault();
                event.stopPropagation();
                onReset();
            }}
            aria-hidden="true"
        />
    );
}
