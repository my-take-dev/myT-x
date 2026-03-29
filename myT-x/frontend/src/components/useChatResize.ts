import {useCallback, useEffect, useMemo, useRef, useState} from "react";

type AnchorPosition = "bottom" | "top" | "left" | "right";

export const ANCHOR_BUTTONS: AnchorPosition[] = ["left", "top", "bottom", "right"];
export const ANCHOR_ARROWS: Record<AnchorPosition, string> = {
    bottom: "\u2193",
    right: "\u2192",
    top: "\u2191",
    left: "\u2190",
};

const MIN_OVERLAY_HEIGHT_PX = 120;
const MIN_OVERLAY_WIDTH_PX = 200;

interface UseChatResizeParams {
    readonly expanded: boolean;
    readonly chatOverlayPercentage: number;
}

export function useChatResize({expanded, chatOverlayPercentage}: UseChatResizeParams) {
    const [isHalfSize, setIsHalfSize] = useState(false);
    const [anchor, setAnchor] = useState<AnchorPosition>("bottom");
    const [heightPx, setHeightPx] = useState<number | null>(null);
    const [fullHeightPx, setFullHeightPx] = useState<number | null>(null);
    const [widthPx, setWidthPx] = useState<number | null>(null);
    const [fullWidthPx, setFullWidthPx] = useState<number | null>(null);
    const overlayRef = useRef<HTMLDivElement>(null);
    const dragCleanupRef = useRef<(() => void) | null>(null);
    const anchorRef = useRef<AnchorPosition>("bottom");

    const isHorizontal = anchor === "left" || anchor === "right";

    // Record initial dimension in px when overlay first renders or anchor mode changes.
    useEffect(() => {
        if (!expanded || !overlayRef.current) return;
        const rect = overlayRef.current.getBoundingClientRect();
        if (isHorizontal) {
            if (fullWidthPx == null && rect.width > 0) setFullWidthPx(rect.width);
        } else {
            if (fullHeightPx == null && rect.height > 0) setFullHeightPx(rect.height);
        }
    }, [expanded, isHorizontal, fullHeightPx, fullWidthPx]);

    // Cleanup drag listeners on unmount.
    useEffect(() => {
        return () => {
            dragCleanupRef.current?.();
        };
    }, []);

    // Keep anchor ref in sync for use inside resize handler.
    anchorRef.current = anchor;

    // Drag-to-resize handler: mousedown on the resize handle starts tracking
    // mousemove to update overlay dimensions, with mouseup/blur cleanup.
    const startResize = useCallback((handle: HTMLDivElement) => {
        const mainContent = handle.closest(".main-content");
        if (!(mainContent instanceof HTMLElement)) return;

        const initialUserSelect = document.body.style.userSelect;
        document.body.style.userSelect = "none";

        const currentAnchor = anchorRef.current;
        const isHoriz = currentAnchor === "left" || currentAnchor === "right";
        const containerRect = mainContent.getBoundingClientRect();
        const maxSize = isHoriz
            ? containerRect.width * 0.95
            : containerRect.height * 0.95;
        const minSize = isHoriz ? MIN_OVERLAY_WIDTH_PX : MIN_OVERLAY_HEIGHT_PX;

        const onMove = (event: MouseEvent) => {
            const rect = mainContent.getBoundingClientRect();
            if (isHoriz) {
                const newWidth = currentAnchor === "left"
                    ? event.clientX - rect.left
                    : rect.right - event.clientX;
                setWidthPx(Math.max(minSize, Math.min(maxSize, newWidth)));
            } else {
                const newHeight = currentAnchor === "top"
                    ? event.clientY - rect.top
                    : rect.bottom - event.clientY;
                setHeightPx(Math.max(minSize, Math.min(maxSize, newHeight)));
            }
        };

        const cleanup = () => {
            window.removeEventListener("mousemove", onMove);
            window.removeEventListener("mouseup", onUp);
            window.removeEventListener("blur", onUp);
            document.body.style.userSelect = initialUserSelect;
        };

        const onUp = () => {
            cleanup();
            if (dragCleanupRef.current === cleanup) {
                dragCleanupRef.current = null;
            }
            if (overlayRef.current) {
                const overlayRect = overlayRef.current.getBoundingClientRect();
                if (isHoriz) {
                    if (overlayRect.width > 0) {
                        setFullWidthPx(overlayRect.width);
                        setIsHalfSize(false);
                    }
                } else {
                    if (overlayRect.height > 0) {
                        setFullHeightPx(overlayRect.height);
                        setIsHalfSize(false);
                    }
                }
            }
        };

        if (dragCleanupRef.current) {
            dragCleanupRef.current();
        }
        dragCleanupRef.current = cleanup;
        window.addEventListener("mousemove", onMove);
        window.addEventListener("mouseup", onUp);
        window.addEventListener("blur", onUp, {once: true});
    }, []);

    // Toggle half size.
    const toggleHalfSize = useCallback(() => {
        if (isHorizontal) {
            if (fullWidthPx == null) return;
            if (isHalfSize) {
                setWidthPx(fullWidthPx);
            } else {
                setWidthPx(Math.max(MIN_OVERLAY_WIDTH_PX, Math.round(fullWidthPx / 2)));
            }
        } else {
            if (fullHeightPx == null) return;
            if (isHalfSize) {
                setHeightPx(fullHeightPx);
            } else {
                setHeightPx(Math.max(MIN_OVERLAY_HEIGHT_PX, Math.round(fullHeightPx / 2)));
            }
        }
        setIsHalfSize((prev) => !prev);
    }, [isHorizontal, fullWidthPx, fullHeightPx, isHalfSize]);

    // Set anchor position directly.
    const changeAnchor = useCallback((pos: AnchorPosition) => {
        setAnchor(pos);
        setIsHalfSize(false);
    }, []);

    // Overlay class name based on anchor position.
    const overlayClassName = useMemo(() => {
        if (anchor === "bottom") return "chat-overlay";
        return `chat-overlay chat-overlay--anchor-${anchor}`;
    }, [anchor]);

    // Style: px-based if set, otherwise %-based from config.
    const overlayStyle = useMemo(() => {
        if (isHorizontal) {
            const w = widthPx != null ? `${widthPx}px` : `${chatOverlayPercentage}%`;
            return {width: w};
        }
        const h = heightPx != null ? `${heightPx}px` : `${chatOverlayPercentage}%`;
        return {height: h};
    }, [isHorizontal, widthPx, heightPx, chatOverlayPercentage]);

    return {
        isHalfSize,
        anchor,
        overlayRef,
        isHorizontal,
        startResize,
        toggleHalfSize,
        changeAnchor,
        overlayClassName,
        overlayStyle,
    };
}
