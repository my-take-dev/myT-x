const MIN_BOUNDARY_EPSILON_PX = 2;

function resolveBoundaryEpsilonPx(): number {
    if (typeof window === "undefined") {
        return MIN_BOUNDARY_EPSILON_PX;
    }
    const dpr = Number(window.devicePixelRatio);
    if (!Number.isFinite(dpr) || dpr <= 0) {
        return MIN_BOUNDARY_EPSILON_PX;
    }
    return Math.max(MIN_BOUNDARY_EPSILON_PX, Math.ceil(dpr));
}

/**
 * Consumes a wheel event at scroll boundaries to prevent parent scroll chaining
 * and boundary jitter. Returns true when the event was consumed.
 *
 * @sideEffect Snaps `el.scrollTop` to its exact boundary value (0 or maxScrollTop)
 * when the element is within epsilon of the edge, eliminating sub-pixel residuals
 * that cause jitter on subsequent frames.
 */
export function consumeBoundaryWheel(el: HTMLElement, deltaY: number): boolean {
    if (deltaY === 0) return false;
    const epsilon = resolveBoundaryEpsilonPx();
    const maxScrollTop = Math.max(0, el.scrollHeight - el.clientHeight);

    if (deltaY < 0) {
        const atTop = el.scrollTop <= epsilon;
        if (atTop && el.scrollTop !== 0) {
            el.scrollTop = 0;
        }
        return atTop;
    }

    const distanceToBottom = maxScrollTop - el.scrollTop;
    const atBottom = distanceToBottom <= epsilon;
    if (atBottom && el.scrollTop !== maxScrollTop) {
        el.scrollTop = maxScrollTop;
    }
    return atBottom;
}

/**
 * Shared wheel event handler for react-window outer containers.
 * Forwards the event to the passthrough handler (if any), then consumes the event
 * at scroll boundaries to prevent parent scroll chaining.
 *
 * Usage (in forwardRef outerElementType components):
 *   onWheel={(e) => handleBoundaryWheel(e, props.onWheel)}
 */
export function handleBoundaryWheel(
    e: React.WheelEvent<HTMLElement>,
    passthrough?: React.WheelEventHandler<HTMLElement>,
): void {
    passthrough?.(e);
    if (e.defaultPrevented) return;
    if (consumeBoundaryWheel(e.currentTarget, e.deltaY)) {
        e.preventDefault();
        e.stopPropagation();
    }
}
