import {useCallback, useEffect, useState} from "react";
import type {RefObject} from "react";
import type {TreeNavigationNode} from "./treeNavigation";
import {findParentDirectoryIndex} from "./treeNavigation";

interface TreeFocusNode extends TreeNavigationNode {
    readonly path: string;
}

/** Minimal scrollable list interface used by the hook (subset of FixedSizeList). */
interface ScrollableList {
    readonly scrollToItem: FixedSizeListScrollToItem;
}

/** Derived from react-window's FixedSizeList.scrollToItem signature. */
type FixedSizeListScrollToItem = (index: number, align?: "auto" | "smart" | "center" | "end" | "start") => void;

/**
 * Shared hook for virtualized tree focus management.
 *
 * Encapsulates focusedIndex state, clamped focus navigation with auto-scroll,
 * parent directory lookup, and selectedPath synchronisation.
 *
 * Used by DiffFileSidebar and FileTreeSidebar.
 */
export function useVirtualizedTreeFocus<T extends TreeFocusNode>(
    flatNodes: readonly T[],
    selectedPath: string | null,
    listRef: RefObject<ScrollableList | null>,
): {
    readonly focusedIndex: number;
    readonly focusIndex: (index: number) => void;
    readonly findParentIndex: (index: number) => number;
} {
    const [focusedIndex, setFocusedIndex] = useState(0);

    const focusIndex = useCallback((index: number) => {
        if (flatNodes.length === 0) {
            setFocusedIndex(0);
            return;
        }
        const clamped = Math.max(0, Math.min(index, flatNodes.length - 1));
        setFocusedIndex(clamped);
        listRef.current?.scrollToItem(clamped, "smart");
    }, [flatNodes.length, listRef]);

    const findParentIndex = useCallback((index: number): number => {
        return findParentDirectoryIndex(flatNodes, index);
    }, [flatNodes]);

    // Keep focusedIndex in sync with selectedPath and flatNodes changes
    useEffect(() => {
        if (flatNodes.length === 0) {
            setFocusedIndex(0);
            return;
        }
        const selectedIndex = selectedPath
            ? flatNodes.findIndex((node) => node.path === selectedPath)
            : -1;
        if (selectedIndex >= 0) {
            setFocusedIndex(selectedIndex);
            return;
        }
        setFocusedIndex((prev) => Math.max(0, Math.min(prev, flatNodes.length - 1)));
    }, [flatNodes, selectedPath]);

    return {focusedIndex, focusIndex, findParentIndex};
}
