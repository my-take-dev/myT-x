import {act, useRef} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {useContainerHeight} from "../src/hooks/useContainerHeight";

class MockResizeObserver {
    static instances: MockResizeObserver[] = [];
    readonly callback: ResizeObserverCallback;
    readonly observe = vi.fn();
    readonly disconnect = vi.fn();

    constructor(callback: ResizeObserverCallback) {
        this.callback = callback;
        MockResizeObserver.instances.push(this);
    }

    emit(height: number): void {
        this.callback([
            {contentRect: {height} as DOMRectReadOnly} as ResizeObserverEntry,
        ], this as unknown as ResizeObserver);
    }

    emitEntries(entries: ResizeObserverEntry[]): void {
        this.callback(entries, this as unknown as ResizeObserver);
    }
}

let rafId = 0;
let rafQueue = new Map<number, FrameRequestCallback>();

function flushRafQueue(): void {
    const queued = [...rafQueue.entries()];
    rafQueue.clear();
    for (const [, callback] of queued) {
        callback(0);
    }
}

function HeightProbe({minHeight = 0}: { minHeight?: number }) {
    const ref = useRef<HTMLDivElement | null>(null);
    const height = useContainerHeight(ref, minHeight);
    return (
        <div>
            <div ref={ref}/>
            <output data-testid="height">{height}</output>
        </div>
    );
}

function NullRefProbe({minHeight = 0}: { minHeight?: number }) {
    const ref = useRef<HTMLDivElement | null>(null);
    const height = useContainerHeight(ref, minHeight);
    return <output data-testid="height">{height}</output>;
}

describe("useContainerHeight", () => {
    let container: HTMLDivElement;
    let root: Root;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        MockResizeObserver.instances = [];
        rafId = 0;
        rafQueue = new Map<number, FrameRequestCallback>();

        vi.stubGlobal("ResizeObserver", MockResizeObserver);
        vi.spyOn(window, "requestAnimationFrame").mockImplementation((callback: FrameRequestCallback) => {
            rafId += 1;
            rafQueue.set(rafId, callback);
            return rafId;
        });
        vi.spyOn(window, "cancelAnimationFrame").mockImplementation((id: number) => {
            rafQueue.delete(id);
        });
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        container.remove();
        vi.restoreAllMocks();
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = false;
    });

    it("updates height from ResizeObserver using floored integer value", () => {
        act(() => {
            root.render(<HeightProbe/>);
        });

        const observer = MockResizeObserver.instances[0];
        expect(observer).toBeDefined();

        act(() => {
            observer.emit(123.9);
            flushRafQueue();
        });

        expect(container.querySelector('[data-testid="height"]')?.textContent).toBe("123");
    });

    it("applies minHeight immediately when minHeight prop changes", () => {
        act(() => {
            root.render(<HeightProbe minHeight={0}/>);
        });
        const observer = MockResizeObserver.instances[0];

        act(() => {
            observer.emit(40);
            flushRafQueue();
        });
        expect(container.querySelector('[data-testid="height"]')?.textContent).toBe("40");

        act(() => {
            root.render(<HeightProbe minHeight={80}/>);
        });
        expect(container.querySelector('[data-testid="height"]')?.textContent).toBe("80");
    });

    it("applies minHeight floor when observed height is smaller", () => {
        act(() => {
            root.render(<HeightProbe minHeight={50}/>);
        });
        const observer = MockResizeObserver.instances[0];

        act(() => {
            observer.emit(30);
            flushRafQueue();
        });

        expect(container.querySelector('[data-testid="height"]')?.textContent).toBe("50");
    });

    it("absorbs sub-pixel oscillation within 1px of current value", () => {
        act(() => {
            root.render(<HeightProbe/>);
        });

        const observer = MockResizeObserver.instances[0];

        // Initial observation at 500px
        act(() => {
            observer.emit(500);
            flushRafQueue();
        });
        expect(container.querySelector('[data-testid="height"]')?.textContent).toBe("500");

        // Second observation at 500.4px (< 1px diff) — should NOT update
        act(() => {
            observer.emit(500.4);
            flushRafQueue();
        });
        expect(container.querySelector('[data-testid="height"]')?.textContent).toBe("500");
    });

    it("suppresses update when floor diff is within ±1px tolerance", () => {
        act(() => {
            root.render(<HeightProbe/>);
        });

        const observer = MockResizeObserver.instances[0];

        // Initial observation at 500px
        act(() => {
            observer.emit(500);
            flushRafQueue();
        });
        expect(container.querySelector('[data-testid="height"]')?.textContent).toBe("500");

        // Observation at 501.5px → floor=501, |501-500|=1 → within tolerance, suppressed
        act(() => {
            observer.emit(501.5);
            flushRafQueue();
        });
        expect(container.querySelector('[data-testid="height"]')?.textContent).toBe("500");
    });

    it("always applies initial measurement even from zero", () => {
        act(() => {
            root.render(<HeightProbe/>);
        });

        // Initial height is 0
        expect(container.querySelector('[data-testid="height"]')?.textContent).toBe("0");

        const observer = MockResizeObserver.instances[0];

        // First observation should always be applied regardless of prev being 0
        act(() => {
            observer.emit(0.5);
            flushRafQueue();
        });
        // prev is 0, so the guard `prev > 0 && ...` is false → update is applied
        // Math.floor(0.5) = 0, so height remains 0, but the update itself was applied
        expect(container.querySelector('[data-testid="height"]')?.textContent).toBe("0");

        // Verify with a non-zero value: first real measurement is always applied
        act(() => {
            observer.emit(250);
            flushRafQueue();
        });
        expect(container.querySelector('[data-testid="height"]')?.textContent).toBe("250");
    });

    it("disconnects observer and cancels pending animation frame on unmount", () => {
        act(() => {
            root.render(<HeightProbe/>);
        });
        const observer = MockResizeObserver.instances[0];

        act(() => {
            observer.emit(60);
        });

        act(() => {
            root.unmount();
        });

        expect(observer.disconnect).toHaveBeenCalled();
        expect(window.cancelAnimationFrame).toHaveBeenCalled();
    });

    // NullRefProbe never assigns a DOM element to the ref, so useContainerHeight's
    // useEffect guard (`if (!el) return`) skips ResizeObserver creation entirely.
    it("does not observe when ref target stays null", () => {
        act(() => {
            root.render(<NullRefProbe minHeight={7}/>);
        });

        // No observer instantiated because the ref target is always null.
        expect(MockResizeObserver.instances.length).toBe(0);
        // Height falls back to minHeight since no observation ever fires.
        expect(container.querySelector('[data-testid="height"]')?.textContent).toBe("7");
    });

    it("tracks downward drift within tolerance to prevent container overflow", () => {
        act(() => {
            root.render(<HeightProbe/>);
        });
        const observer = MockResizeObserver.instances[0];

        act(() => {
            observer.emit(500);
            flushRafQueue();
        });
        expect(container.querySelector('[data-testid="height"]')?.textContent).toBe("500");

        // raw=499.9 → floor=499, |499-500|=1 → within tolerance → min(500,499)=499
        // The value decreases to prevent FixedSizeList from overflowing the parent
        // container when the parent shrinks within the tolerance window.
        act(() => {
            observer.emit(499.9);
            flushRafQueue();
        });
        expect(container.querySelector('[data-testid="height"]')?.textContent).toBe("499");
    });

    it("suppresses update when floor value equals prev (raw=500.4, prev=500)", () => {
        act(() => {
            root.render(<HeightProbe/>);
        });
        const observer = MockResizeObserver.instances[0];

        act(() => {
            observer.emit(500);
            flushRafQueue();
        });
        expect(container.querySelector('[data-testid="height"]')?.textContent).toBe("500");

        // raw=500.4 → floor=500 === 500 → suppressed
        act(() => {
            observer.emit(500.4);
            flushRafQueue();
        });
        expect(container.querySelector('[data-testid="height"]')?.textContent).toBe("500");
    });

    it("settles at oscillation minimum during N → N-1 → N cycle", () => {
        act(() => {
            root.render(<HeightProbe/>);
        });
        const observer = MockResizeObserver.instances[0];

        act(() => {
            observer.emit(500);
            flushRafQueue();
        });
        expect(container.querySelector('[data-testid="height"]')?.textContent).toBe("500");

        // 499.2 → floor=499, |499-500|=1 → tolerance → min(500,499)=499
        act(() => {
            observer.emit(499.2);
            flushRafQueue();
        });
        expect(container.querySelector('[data-testid="height"]')?.textContent).toBe("499");

        // 500.8 → floor=500, |500-499|=1 → tolerance → min(499,500)=499
        // Value stays at 499 — the lower bound prevents container overflow.
        act(() => {
            observer.emit(500.8);
            flushRafQueue();
        });
        expect(container.querySelector('[data-testid="height"]')?.textContent).toBe("499");

        // Subsequent downward measurement is also absorbed: min(499,499)=499
        act(() => {
            observer.emit(499.5);
            flushRafQueue();
        });
        expect(container.querySelector('[data-testid="height"]')?.textContent).toBe("499");
    });

    it("updates height when change exceeds ±1px tolerance", () => {
        act(() => {
            root.render(<HeightProbe/>);
        });
        const observer = MockResizeObserver.instances[0];

        act(() => {
            observer.emit(500);
            flushRafQueue();
        });
        expect(container.querySelector('[data-testid="height"]')?.textContent).toBe("500");

        // 510 → floor=510, |510-500|=10 → exceeds tolerance, update applied
        act(() => {
            observer.emit(510);
            flushRafQueue();
        });
        expect(container.querySelector('[data-testid="height"]')?.textContent).toBe("510");

        // 498 → floor=498, |498-510|=12 → exceeds tolerance, update applied
        act(() => {
            observer.emit(498);
            flushRafQueue();
        });
        expect(container.querySelector('[data-testid="height"]')?.textContent).toBe("498");
    });

    it("stabilizes after one downward adjustment then suppresses further oscillation", () => {
        act(() => {
            root.render(<HeightProbe/>);
        });
        const observer = MockResizeObserver.instances[0];

        // Initial measurement
        act(() => {
            observer.emit(400);
            flushRafQueue();
        });
        expect(container.querySelector('[data-testid="height"]')?.textContent).toBe("400");

        // First downward: 399.3 → floor=399, min(400,399)=399
        act(() => {
            observer.emit(399.3);
            flushRafQueue();
        });
        expect(container.querySelector('[data-testid="height"]')?.textContent).toBe("399");

        // Upward bounce: 400.7 → floor=400, min(399,400)=399 — suppressed
        act(() => {
            observer.emit(400.7);
            flushRafQueue();
        });
        expect(container.querySelector('[data-testid="height"]')?.textContent).toBe("399");

        // Another downward: 399.1 → floor=399, min(399,399)=399 — no change
        act(() => {
            observer.emit(399.1);
            flushRafQueue();
        });
        expect(container.querySelector('[data-testid="height"]')?.textContent).toBe("399");

        // Genuine resize beyond tolerance: 410 → |410-399|=11 > 1 → update
        act(() => {
            observer.emit(410);
            flushRafQueue();
        });
        expect(container.querySelector('[data-testid="height"]')?.textContent).toBe("410");
    });

    it("ignores empty ResizeObserver entry arrays", () => {
        act(() => {
            root.render(<HeightProbe/>);
        });
        const observer = MockResizeObserver.instances[0];
        expect(observer).toBeDefined();

        act(() => {
            observer.emitEntries([]);
            // The empty-entries guard returns early before scheduling a rAF,
            // so flushRafQueue is a no-op here — included for structural consistency.
            flushRafQueue();
        });

        expect(container.querySelector('[data-testid="height"]')?.textContent).toBe("0");
    });
});
