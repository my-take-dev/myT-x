import {useEffect, useLayoutEffect, useRef, useState} from "react";
import type {RefObject} from "react";

interface UseContainerHeightOptions {
    /**
     * Suppress ResizeObserver noise by **freezing** height when the measured delta
     * is within ±N px of the current value. Both shrink and grow directions within
     * the threshold are suppressed (`return prev`), producing a complete freeze.
     *
     * Design rationale: `clientHeight` is an integer, so ±1px oscillation from
     * sub-pixel rounding is the primary noise source in HiDPI environments. A real
     * container resize always produces a delta > 1px, so freezing within ±1px
     * eliminates noise without masking genuine resizes. The previous `Math.min`
     * (shrink-follow) approach was removed because `clientHeight` does not exhibit
     * the sub-pixel oscillation that made directional tracking necessary with
     * `contentRect.height`.
     *
     * Defaults to 1. Set to 0 to disable noise suppression.
     * Non-negative integer expected. Negative values are clamped to 0.
     *
     * Boundary conditions:
     * - The **first** non-zero measurement always applies (threshold is skipped when
     *   `prev === 0`), ensuring the hook transitions from its initial zero state
     *   to the real container size without being suppressed.
     * - When `next === 0` (container hidden/collapsed), the threshold is also skipped
     *   so the hook correctly reflects zero height.
     *
     * The `options` object does not need to be referentially stable — only the
     * primitive `noiseThresholdPx` value is tracked in effect dependencies.
     */
    readonly noiseThresholdPx?: number;
}

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
 * @param options      - Optional stabilization options. Use `noiseThresholdPx` to
 *                       freeze height within ±N px of the current value, suppressing
 *                       sub-pixel ResizeObserver churn (e.g. `{noiseThresholdPx: 1}`).
 */
export function useContainerHeight(
    containerRef: RefObject<HTMLElement | null>,
    minHeight = 0,
    options: UseContainerHeightOptions = {},
): number {
    const resizeFrameRef = useRef<number | null>(null);
    const minHeightRef = useRef(minHeight);
    const noiseThresholdRef = useRef(Math.max(0, Math.floor(options.noiseThresholdPx ?? 1)));
    const [observedElement, setObservedElement] = useState<HTMLElement | null>(null);
    const [height, setHeight] = useState(0);

    // useLayoutEffect ensures refs are updated before the next ResizeObserver → rAF cycle
    // in the same frame, avoiding stale values in the setHeight callback.
    useLayoutEffect(() => {
        minHeightRef.current = minHeight;
    }, [minHeight]);

    useLayoutEffect(() => {
        noiseThresholdRef.current = Math.max(0, Math.floor(options.noiseThresholdPx ?? 1));
    }, [options.noiseThresholdPx]);

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
                // Prefer target.clientHeight (integer layout height) over contentRect.height.
                // contentRect can oscillate by sub-pixels in HiDPI environments, which is
                // sufficient to trigger 1px height churn in virtualized lists.
                // Fallback to contentRect when clientHeight is 0 (transient CSSOM state
                // during tab switches or visibility changes) or unavailable (non-HTMLElement).
                const targetHeight = last.target instanceof HTMLElement
                    ? last.target.clientHeight
                    : Number.NaN;
                const rawHeight = (Number.isFinite(targetHeight) && targetHeight > 0)
                    ? targetHeight
                    : Math.floor(last.contentRect.height);
                const next = Math.max(minHeightRef.current, rawHeight);
                if (!Number.isFinite(next)) {
                    console.error("[useContainerHeight] invalid measured height", {
                        targetHeight,
                        contentRect: last.contentRect.height,
                        next
                    });
                    return;
                }
                setHeight((prev) => {
                    if (prev === next) return prev;
                    const threshold = noiseThresholdRef.current;
                    if (threshold > 0 && prev > 0 && next > 0 && Math.abs(prev - next) <= threshold) {
                        return prev;
                    }
                    return next;
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
