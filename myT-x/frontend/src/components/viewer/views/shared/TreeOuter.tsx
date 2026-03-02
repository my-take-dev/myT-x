import {forwardRef, type HTMLAttributes} from "react";

type TreeOuterComponent = ReturnType<typeof forwardRef<HTMLDivElement, HTMLAttributes<HTMLDivElement>>>;

/**
 * Factory that creates a custom outer element for FixedSizeList with ARIA tree role.
 * react-window's outerElementType does not forward custom props, so the aria-label
 * must be baked into the component via this factory.
 *
 * IMPORTANT: Call this function at **module level** or inside `useMemo` only.
 * Calling it inside a render function body creates a new component type on every render,
 * causing react-window to unmount and remount the entire list each cycle.
 *
 * Usage:
 *   outerElementType={TreeOuter}          // default aria-label="File tree"
 *   outerElementType={makeTreeOuter("Changed files")}
 */
export function makeTreeOuter(ariaLabel: string): TreeOuterComponent {
    return forwardRef<HTMLDivElement, HTMLAttributes<HTMLDivElement>>(
        function TreeOuter(props, ref) {
            return <div {...props} ref={ref} role="tree" aria-label={ariaLabel}/>;
        },
    );
}

// Backward-compatible alias for existing imports.
export const createTreeOuter = makeTreeOuter;

/** Default TreeOuter with aria-label="File tree" for FileTreeSidebar. */
export const TreeOuter = makeTreeOuter("File tree");
