import {forwardRef, type CSSProperties, type HTMLAttributes} from "react";
import {handleBoundaryWheel} from "../../../../utils/scrollBoundary";

type ScrollStableOuterComponent = ReturnType<typeof forwardRef<HTMLDivElement, HTMLAttributes<HTMLDivElement>>>;

interface ScrollStableOuterOptions {
    /** ARIA role for the container element (e.g. "tree", "list"). */
    readonly role: string;
    /** Accessible label for the container. */
    readonly ariaLabel: string;
    /**
     * Horizontal overflow mode. Defaults to "hidden".
     * Use "scroll" for containers that need persistent horizontal scrollbar
     * (e.g. FileContentViewer where wide lines exist).
     */
    readonly overflowX?: CSSProperties["overflowX"];
}

/**
 * Factory that creates a scroll-stabilized outer element for react-window lists/trees.
 *
 * Applies a consistent set of scroll stabilization properties:
 * - `overflowY: "auto"` with configurable `overflowX`
 * - `overscrollBehavior: "none"` prevents parent scroll chaining
 * - `overflowAnchor: "none"` prevents scroll anchor adjustments
 * - `scrollbarGutter: "stable"` reserves scrollbar space to prevent width oscillation
 * - Boundary wheel handling via `handleBoundaryWheel` to prevent jitter at edges
 *
 * react-window's `outerElementType` does not forward custom props, so ARIA attributes
 * must be baked into the component via this factory.
 *
 * IMPORTANT: Call this function at **module level** or inside `useMemo` only.
 * Calling it inside a render function body creates a new component type on every render,
 * causing react-window to unmount and remount the entire list each cycle.
 *
 * Usage:
 *   outerElementType={TreeOuter}                                              // tree, "File tree"
 *   outerElementType={makeTreeOuter("Changed files")}                         // tree, custom label
 *   outerElementType={makeScrollStableOuter({role: "list", ariaLabel: "Sessions"})}
 */
export function makeScrollStableOuter(options: ScrollStableOuterOptions): ScrollStableOuterComponent {
    return forwardRef<HTMLDivElement, HTMLAttributes<HTMLDivElement>>(
        function ScrollStableOuter(props, ref) {
            // react-window sets inline `overflow: auto` on the outer element.
            // Override with scroll-stabilized properties to prevent jitter.
            const mergedStyle: CSSProperties = {
                ...props.style,
                overflowX: options.overflowX ?? "hidden",
                overflowY: "auto",
                overscrollBehavior: "none",
                overflowAnchor: "none",
                scrollbarGutter: "stable",
            };
            return <div {...props} ref={ref} role={options.role} aria-label={options.ariaLabel} style={mergedStyle}
                        onWheel={(e) => handleBoundaryWheel(e, props.onWheel)}/>;
        },
    );
}

/** Convenience factory for tree-role containers. */
export function makeTreeOuter(ariaLabel: string): ScrollStableOuterComponent {
    return makeScrollStableOuter({role: "tree", ariaLabel});
}

// Backward-compatible alias for existing imports.
export const createTreeOuter = makeTreeOuter;

/** Default TreeOuter with aria-label="File tree" for FileTreeSidebar. */
export const TreeOuter = makeTreeOuter("File tree");
