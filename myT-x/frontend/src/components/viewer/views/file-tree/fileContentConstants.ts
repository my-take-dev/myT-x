/** Fallback row height for FixedSizeList before DOM measurement. */
export const FILE_CONTENT_ROW_HEIGHT_FALLBACK = 20;

/** Debounce delay before copying a mouse-drag selection to clipboard. */
export const COPY_ON_SELECT_DEBOUNCE_MS = 100;

/** Duration the selection notice is shown in the header bar. */
export const COPY_SELECTION_NOTICE_MS = 1800;

/**
 * Extra rows beyond one viewport to keep mounted for overscan.
 * Combined with the dynamically computed viewport row count, this ensures selection
 * anchors survive moderate scrolling without being unmounted.
 * Value rationale: 4 rows provides a comfortable buffer for mouse-drag selections
 * that extend slightly beyond the visible area, without over-rendering on small screens.
 */
export const OVERSCAN_BUFFER = 4;

/** Safety cap to avoid excessive offscreen row rendering on very tall viewports. */
export const MAX_OVERSCAN_ROWS = 120;

/** Reserve a sensible initial viewport so the list does not collapse before ResizeObserver reports. */
export const MIN_BODY_VIEWPORT_ROWS = 12;

/**
 * Sentinel value returned by getTypographyStyleSignature when getComputedStyle fails
 * (e.g. detached elements). Using a fixed string ensures repeated failures hit the
 * cache instead of triggering new DOM probe measurements each time.
 */
export const TYPOGRAPHY_SIGNATURE_ERROR = "typography-signature-error";
