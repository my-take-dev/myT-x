import {type RefObject, useEffect, useRef, useState} from "react";
import {FILE_CONTENT_ROW_HEIGHT_FALLBACK} from "./fileContentConstants";
import {calculateRowHeight, getTypographyStyleSignature} from "./fileContentUtils";

/**
 * Tracks row height for a virtualized FixedSizeList via DOM probe measurement.
 *
 * Initial value is FILE_CONTENT_ROW_HEIGHT_FALLBACK (20px). This is replaced by an accurate
 * DOM-measured value once the container element is mounted and calculateRowHeight runs.
 * The first render uses the fallback height, which is close enough to avoid layout jumps.
 *
 * Re-measurement is triggered when bodyHeight changes (indicating a resize) or when
 * the virtualized body becomes visible. A typography signature cache prevents redundant
 * DOM probe measurements when styles have not changed.
 */
export function useRowHeight(
    containerRef: RefObject<HTMLDivElement | null>,
    shouldMeasure: boolean,
    bodyHeight: number,
): number {
    const [rowHeight, setRowHeight] = useState(FILE_CONTENT_ROW_HEIGHT_FALLBACK);
    const rowHeightCacheRef = useRef<{ signature: string; value: number } | null>(null);

    useEffect(() => {
        if (!shouldMeasure) return;

        const el = containerRef.current;
        if (!el) return;

        const typographySignature = getTypographyStyleSignature(el);
        const cachedRowHeight = rowHeightCacheRef.current;
        if (cachedRowHeight && cachedRowHeight.signature === typographySignature) {
            setRowHeight((prev) => (prev === cachedRowHeight.value ? prev : cachedRowHeight.value));
            return;
        }
        const measuredRowHeight = calculateRowHeight(el);
        rowHeightCacheRef.current = {
            signature: typographySignature,
            value: measuredRowHeight,
        };
        setRowHeight((prev) => (prev === measuredRowHeight ? prev : measuredRowHeight));
    }, [shouldMeasure, bodyHeight, containerRef]);

    return rowHeight;
}
