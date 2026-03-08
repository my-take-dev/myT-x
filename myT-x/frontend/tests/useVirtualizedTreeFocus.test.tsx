import {act, useEffect, useRef} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it} from "vitest";
import {useVirtualizedTreeFocus} from "../src/components/viewer/views/shared/useVirtualizedTreeFocus";

interface TestNode {
    readonly path: string;
    readonly depth: number;
    readonly isDir: boolean;
}

interface ScrollCall {
    readonly index: number;
    readonly align: "auto" | "smart" | "center" | "end" | "start" | undefined;
}

interface ExposedHook {
    readonly focusedIndex: number;
    readonly focusIndex: (index: number) => void;
    readonly findParentIndex: (index: number) => number;
    readonly scrollCalls: readonly ScrollCall[];
}

function HookProbe({
                       flatNodes,
                       selectedPath,
                       onExpose,
                   }: {
    flatNodes: readonly TestNode[];
    selectedPath: string | null;
    onExpose: (value: ExposedHook) => void;
}) {
    const scrollCallsRef = useRef<ScrollCall[]>([]);
    const listRef = useRef<{
        scrollToItem: (index: number, align?: "auto" | "smart" | "center" | "end" | "start") => void;
    } | null>(null);
    if (!listRef.current) {
        listRef.current = {
            scrollToItem: (index, align) => {
                scrollCallsRef.current.push({index, align});
            },
        };
    }

    const {focusedIndex, focusIndex, findParentIndex} = useVirtualizedTreeFocus(flatNodes, selectedPath, listRef);

    useEffect(() => {
        onExpose({
            focusedIndex,
            focusIndex,
            findParentIndex,
            scrollCalls: scrollCallsRef.current,
        });
    }, [focusedIndex, focusIndex, findParentIndex, onExpose]);

    return null;
}

describe("useVirtualizedTreeFocus", () => {
    let container: HTMLDivElement;
    let root: Root;
    let latest: ExposedHook | null;

    const nodes: readonly TestNode[] = [
        {path: "/dir", depth: 0, isDir: true},
        {path: "/dir/file-a.txt", depth: 1, isDir: false},
        {path: "/dir/sub", depth: 1, isDir: true},
        {path: "/dir/sub/file-b.txt", depth: 2, isDir: false},
    ];

    const renderProbe = (flatNodes: readonly TestNode[], selectedPath: string | null) => {
        act(() => {
            root.render(
                <HookProbe
                    flatNodes={flatNodes}
                    selectedPath={selectedPath}
                    onExpose={(value) => {
                        latest = value;
                    }}
                />,
            );
        });
    };

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        latest = null;
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        container.remove();
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = false;
    });

    it("syncs focusedIndex to selectedPath when present", () => {
        renderProbe(nodes, "/dir/sub/file-b.txt");
        expect(latest?.focusedIndex).toBe(3);
    });

    it("clamps focusIndex and scrolls with smart alignment", () => {
        renderProbe(nodes, null);
        expect(latest?.focusedIndex).toBe(0);

        act(() => {
            latest?.focusIndex(999);
        });

        expect(latest?.focusedIndex).toBe(nodes.length - 1);
        const calls = latest?.scrollCalls ?? [];
        const lastCall = calls[calls.length - 1];
        expect(lastCall).toEqual({index: nodes.length - 1, align: "smart"});
    });

    it("does not scroll when focus target resolves to current index", () => {
        renderProbe(nodes, null);
        expect(latest?.focusedIndex).toBe(0);
        expect(latest?.scrollCalls.length).toBe(0);

        act(() => {
            latest?.focusIndex(-999); // clamps to 0 (already focused)
        });

        expect(latest?.focusedIndex).toBe(0);
        expect(latest?.scrollCalls.length).toBe(0);
    });

    it("clamps stale focusedIndex when node list shrinks and selectedPath is missing", () => {
        renderProbe(nodes, null);
        act(() => {
            latest?.focusIndex(3);
        });
        expect(latest?.focusedIndex).toBe(3);

        renderProbe([nodes[0]], null);
        expect(latest?.focusedIndex).toBe(0);
    });

    it("does not scroll when upper-clamped focus target matches current index", () => {
        renderProbe(nodes, null);

        // Move to last index
        act(() => {
            latest?.focusIndex(999);
        });
        expect(latest?.focusedIndex).toBe(nodes.length - 1);
        const callsAfterFirst = latest?.scrollCalls.length ?? 0;

        // Same clamped target again - should not scroll
        act(() => {
            latest?.focusIndex(999);
        });
        expect(latest?.focusedIndex).toBe(nodes.length - 1);
        expect(latest?.scrollCalls.length).toBe(callsAfterFirst);
    });

    it("does not scroll when focusIndex is called with exact current index", () => {
        renderProbe(nodes, null);
        expect(latest?.focusedIndex).toBe(0);
        expect(latest?.scrollCalls.length).toBe(0);

        // Direct match (not via clamp) — should not scroll
        act(() => {
            latest?.focusIndex(0);
        });
        expect(latest?.focusedIndex).toBe(0);
        expect(latest?.scrollCalls.length).toBe(0);
    });

    it("finds the nearest parent directory index", () => {
        renderProbe(nodes, null);
        expect(latest?.findParentIndex(3)).toBe(2);
        expect(latest?.findParentIndex(1)).toBe(0);
        expect(latest?.findParentIndex(0)).toBe(-1);
    });
});
