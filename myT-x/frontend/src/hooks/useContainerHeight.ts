import {useEffect, useLayoutEffect, useRef, useState} from "react";
import type {RefObject} from "react";

/**
 * Track the height of a container element via ResizeObserver with rAF debouncing.
 *
 * Returns `Math.max(minHeight, 0)` until the first observation fires.
 * Callers can use `height > 0` to defer rendering until the real size is known.
 *
 * IMPORTANT: `containerRef` must be a stable ref created by `useRef()`.
 * Passing a callback ref or a ref that changes between renders will cause the
 * ResizeObserver to observe a stale element.
 *
 * @param containerRef - Ref to the observed DOM element.
 * @param minHeight    - Optional floor value. Defaults to 0.
 */
export function useContainerHeight(
    containerRef: RefObject<HTMLElement | null>,
    minHeight = 0,
): number {
    const resizeFrameRef = useRef<number | null>(null);
    const minHeightRef = useRef(minHeight);
    const [observedElement, setObservedElement] = useState<HTMLElement | null>(null);
    const [height, setHeight] = useState(0);

    useEffect(() => {
        minHeightRef.current = minHeight;
    }, [minHeight]);

    // Intentional: no dependency array. Runs after every render to detect when
    // containerRef.current transitions from null to a DOM element (e.g., after
    // conditional rendering resolves). The setObservedElement call is guarded by
    // reference equality, so it only triggers a re-render when the element changes.
    useLayoutEffect(() => {
        const next = containerRef.current;
        setObservedElement((prev) => (prev === next ? prev : next));
    });

    useEffect(() => {
        const el = observedElement;
        if (!el) return;

        const ro = new ResizeObserver((entries) => {
            const last = entries[entries.length - 1];
            if (!last) return;
            if (resizeFrameRef.current !== null) {
                cancelAnimationFrame(resizeFrameRef.current);
            }
            resizeFrameRef.current = requestAnimationFrame(() => {
                resizeFrameRef.current = null;
                const raw = last.contentRect.height;
                const floored = Math.max(minHeightRef.current, Math.floor(raw));
                setHeight((prev) => {
                    // Absorb sub-pixel and adjacent-integer oscillation: when the floored
                    // measurement is within ±1px of the current value, use the minimum of
                    // the two. This prevents the FixedSizeList from overflowing its parent
                    // container (which has overflow: hidden) when the actual size shrinks
                    // slightly within tolerance. The min ensures the list height always
                    // stays ≤ the parent's actual rendered height, while still suppressing
                    // upward jitter. After one stabilization render the value settles at
                    // the lower bound of the oscillation range.
                    // Guard: only activate after the first real measurement so initial
                    // 0 → N transitions are never ignored.
                    if (prev > 0 && Math.abs(floored - prev) <= 1) return Math.min(prev, floored);
                    return floored;
                });
            });
        });
        ro.observe(el);

        return () => {
            if (resizeFrameRef.current !== null) {
                cancelAnimationFrame(resizeFrameRef.current);
                resizeFrameRef.current = null;
            }
            ro.disconnect();
        };
    }, [observedElement]);

    // Apply minHeight floor at return time (not just in ResizeObserver callback)
    // so changes to minHeight are reflected immediately without waiting for a resize event.
    return Math.max(minHeight, height);
}
