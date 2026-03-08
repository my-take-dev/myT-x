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

    /**
     * Emit a resize entry. `clientHeight` defaults to `Math.floor(height)` when
     * omitted, which simulates the browser's integer clientHeight from a sub-pixel
     * contentRect. Tests that verify clientHeight behavior should always provide
     * an explicit `clientHeight` value to avoid relying on this default.
     */
    emit(height: number, options?: { readonly clientHeight?: number; readonly target?: Element }): void {
        const target = options?.target ?? document.createElement("div");
        if (target instanceof HTMLElement) {
            Object.defineProperty(target, "clientHeight", {
                value: options?.clientHeight ?? Math.floor(height),
                configurable: true,
            });
        }
        this.callback([
            {
                contentRect: {height} as DOMRectReadOnly,
                target,
            } as ResizeObserverEntry,
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

function getRenderedHeight(probeContainer: HTMLElement): string | null {
    return probeContainer.querySelector('[data-testid="height"]')?.textContent ?? null;
}

function HeightProbe({minHeight = 0}: { readonly minHeight?: number }) {
    const ref = useRef<HTMLDivElement | null>(null);
    const height = useContainerHeight(ref, minHeight);
    return (
        <div>
            <div ref={ref}/>
            <output data-testid="height">{height}</output>
        </div>
    );
}

function HeightProbeWithNoiseFilter({
                                        minHeight = 0,
                                        noiseThresholdPx = 0,
                                    }: {
    readonly minHeight?: number;
    readonly noiseThresholdPx?: number;
}) {
    const ref = useRef<HTMLDivElement | null>(null);
    const height = useContainerHeight(ref, minHeight, {noiseThresholdPx});
    return (
        <div>
            <div ref={ref}/>
            <output data-testid="height">{height}</output>
        </div>
    );
}

function NullRefProbe({minHeight = 0}: { readonly minHeight?: number }) {
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

    it("uses target.clientHeight as the canonical measured value", () => {
        act(() => {
            root.render(<HeightProbe/>);
        });

        const observer = MockResizeObserver.instances[0];
        expect(observer).toBeDefined();

        act(() => {
            observer.emit(123.9, {clientHeight: 124});
            flushRafQueue();
        });

        expect(getRenderedHeight(container)).toBe("124");
    });

    it("falls back to floored contentRect height when target is not an HTMLElement", () => {
        act(() => {
            root.render(<HeightProbe/>);
        });

        const observer = MockResizeObserver.instances[0];
        const svgTarget = document.createElementNS("http://www.w3.org/2000/svg", "svg");

        act(() => {
            observer.emit(123.9, {target: svgTarget});
            flushRafQueue();
        });

        expect(getRenderedHeight(container)).toBe("123");
    });

    it("applies minHeight immediately when minHeight prop changes", () => {
        act(() => {
            root.render(<HeightProbe minHeight={0}/>);
        });

        const observer = MockResizeObserver.instances[0];

        act(() => {
            observer.emit(40, {clientHeight: 40});
            flushRafQueue();
        });
        expect(getRenderedHeight(container)).toBe("40");

        act(() => {
            root.render(<HeightProbe minHeight={80}/>);
        });
        expect(getRenderedHeight(container)).toBe("80");
    });

    it("applies minHeight floor when observed height is smaller", () => {
        act(() => {
            root.render(<HeightProbe minHeight={50}/>);
        });

        const observer = MockResizeObserver.instances[0];

        act(() => {
            observer.emit(30, {clientHeight: 30});
            flushRafQueue();
        });

        expect(getRenderedHeight(container)).toBe("50");
    });

    it("keeps height stable when contentRect jitters but clientHeight is stable", () => {
        act(() => {
            root.render(<HeightProbe/>);
        });

        const observer = MockResizeObserver.instances[0];

        act(() => {
            observer.emit(500.4, {clientHeight: 500});
            flushRafQueue();
        });
        expect(getRenderedHeight(container)).toBe("500");

        act(() => {
            observer.emit(499.2, {clientHeight: 500});
            flushRafQueue();
        });
        expect(getRenderedHeight(container)).toBe("500");

        act(() => {
            observer.emit(500.8, {clientHeight: 500});
            flushRafQueue();
        });
        expect(getRenderedHeight(container)).toBe("500");
    });

    it("tracks real 2px+ changes from clientHeight (default noiseThresholdPx=1)", () => {
        act(() => {
            root.render(<HeightProbe/>);
        });

        const observer = MockResizeObserver.instances[0];

        act(() => {
            observer.emit(500.9, {clientHeight: 500});
            flushRafQueue();
        });
        expect(getRenderedHeight(container)).toBe("500");

        // 1px change is suppressed by default threshold
        act(() => {
            observer.emit(499.1, {clientHeight: 499});
            flushRafQueue();
        });
        expect(getRenderedHeight(container)).toBe("500");

        // 2px change passes the threshold
        act(() => {
            observer.emit(498.1, {clientHeight: 498});
            flushRafQueue();
        });
        expect(getRenderedHeight(container)).toBe("498");

        act(() => {
            observer.emit(500.1, {clientHeight: 500});
            flushRafQueue();
        });
        expect(getRenderedHeight(container)).toBe("500");
    });

    it("tracks every 1px change when noiseThresholdPx is explicitly 0", () => {
        act(() => {
            root.render(<HeightProbeWithNoiseFilter noiseThresholdPx={0}/>);
        });

        const observer = MockResizeObserver.instances[0];

        act(() => {
            observer.emit(500.9, {clientHeight: 500});
            flushRafQueue();
        });
        expect(getRenderedHeight(container)).toBe("500");

        act(() => {
            observer.emit(499.1, {clientHeight: 499});
            flushRafQueue();
        });
        expect(getRenderedHeight(container)).toBe("499");

        act(() => {
            observer.emit(500.1, {clientHeight: 500});
            flushRafQueue();
        });
        expect(getRenderedHeight(container)).toBe("500");
    });

    it("ignores +/-1px noise when noiseThresholdPx is set to 1", () => {
        act(() => {
            root.render(<HeightProbeWithNoiseFilter noiseThresholdPx={1}/>);
        });

        const observer = MockResizeObserver.instances[0];

        act(() => {
            observer.emit(500.1, {clientHeight: 500});
            flushRafQueue();
        });
        expect(getRenderedHeight(container)).toBe("500");

        // TC-1: 1px decrease is also suppressed
        act(() => {
            observer.emit(499.1, {clientHeight: 499});
            flushRafQueue();
        });
        expect(getRenderedHeight(container)).toBe("500");

        // 1px increase is suppressed
        act(() => {
            observer.emit(501.9, {clientHeight: 501});
            flushRafQueue();
        });
        expect(getRenderedHeight(container)).toBe("500");

        // TC-5: exactly threshold+1 (2px) difference updates the value
        act(() => {
            observer.emit(502.1, {clientHeight: 502});
            flushRafQueue();
        });
        expect(getRenderedHeight(container)).toBe("502");

        act(() => {
            observer.emit(503.2, {clientHeight: 503});
            flushRafQueue();
        });
        expect(getRenderedHeight(container)).toBe("502");

        // 3px jump updates
        act(() => {
            observer.emit(505.2, {clientHeight: 505});
            flushRafQueue();
        });
        expect(getRenderedHeight(container)).toBe("505");
    });

    it("applies the first non-zero measurement from initial zero", () => {
        act(() => {
            root.render(<HeightProbe/>);
        });

        expect(getRenderedHeight(container)).toBe("0");
        const observer = MockResizeObserver.instances[0];

        act(() => {
            observer.emit(0.5, {clientHeight: 0});
            flushRafQueue();
        });
        expect(getRenderedHeight(container)).toBe("0");

        act(() => {
            observer.emit(1.6, {clientHeight: 1});
            flushRafQueue();
        });
        expect(getRenderedHeight(container)).toBe("1");
    });

    it("disconnects observer and cancels pending animation frame on unmount", () => {
        act(() => {
            root.render(<HeightProbe/>);
        });
        const observer = MockResizeObserver.instances[0];

        // Emit without flushing rAF queue — this leaves a pending rAF that the
        // cleanup should cancel. Flushing first would settle the rAF, making the
        // cancelAnimationFrame assertion meaningless.
        act(() => {
            observer.emit(60, {clientHeight: 60});
        });
        expect(rafQueue.size).toBeGreaterThan(0);

        act(() => {
            root.unmount();
        });

        expect(observer.disconnect).toHaveBeenCalled();
        expect(window.cancelAnimationFrame).toHaveBeenCalled();
    });

    it("does not observe when ref target stays null", () => {
        act(() => {
            root.render(<NullRefProbe minHeight={7}/>);
        });

        expect(MockResizeObserver.instances.length).toBe(0);
        expect(getRenderedHeight(container)).toBe("7");
    });

    it("ignores empty ResizeObserver entry arrays", () => {
        act(() => {
            root.render(<HeightProbe/>);
        });

        const observer = MockResizeObserver.instances[0];
        expect(observer).toBeDefined();

        act(() => {
            observer.emitEntries([]);
            flushRafQueue();
        });

        expect(getRenderedHeight(container)).toBe("0");
    });

    it("re-observes element after component remounts", () => {
        act(() => {
            root.render(<HeightProbe/>);
        });
        const firstObserver = MockResizeObserver.instances[0];
        expect(firstObserver).toBeDefined();

        act(() => {
            root.unmount();
        });
        expect(firstObserver.disconnect).toHaveBeenCalled();

        // Remount into the same container
        root = createRoot(container);
        act(() => {
            root.render(<HeightProbe/>);
        });

        const secondObserver = MockResizeObserver.instances[MockResizeObserver.instances.length - 1];
        expect(secondObserver).not.toBe(firstObserver);

        act(() => {
            secondObserver.emit(300, {clientHeight: 300});
            flushRafQueue();
        });

        expect(getRenderedHeight(container)).toBe("300");
    });

    it("clamps negative noiseThresholdPx to 0 (filter disabled)", () => {
        act(() => {
            root.render(<HeightProbeWithNoiseFilter noiseThresholdPx={-1}/>);
        });

        const observer = MockResizeObserver.instances[0];

        act(() => {
            observer.emit(500.1, {clientHeight: 500});
            flushRafQueue();
        });
        expect(getRenderedHeight(container)).toBe("500");

        // With threshold clamped to 0, even 1px change should pass through
        act(() => {
            observer.emit(499.1, {clientHeight: 499});
            flushRafQueue();
        });
        expect(getRenderedHeight(container)).toBe("499");
    });

    it("floors fractional noiseThresholdPx (0.9 → 0, filter disabled)", () => {
        act(() => {
            root.render(<HeightProbeWithNoiseFilter noiseThresholdPx={0.9}/>);
        });

        const observer = MockResizeObserver.instances[0];

        act(() => {
            observer.emit(500.1, {clientHeight: 500});
            flushRafQueue();
        });
        expect(getRenderedHeight(container)).toBe("500");

        // Math.floor(0.9) = 0, so 1px change should pass through
        act(() => {
            observer.emit(499.1, {clientHeight: 499});
            flushRafQueue();
        });
        expect(getRenderedHeight(container)).toBe("499");
    });

    it("suppresses ±2px noise when noiseThresholdPx is 2", () => {
        act(() => {
            root.render(<HeightProbeWithNoiseFilter noiseThresholdPx={2}/>);
        });

        const observer = MockResizeObserver.instances[0];

        act(() => {
            observer.emit(500.1, {clientHeight: 500});
            flushRafQueue();
        });
        expect(getRenderedHeight(container)).toBe("500");

        // 2px change is within threshold — suppressed
        act(() => {
            observer.emit(498.1, {clientHeight: 498});
            flushRafQueue();
        });
        expect(getRenderedHeight(container)).toBe("500");

        // 3px change exceeds threshold — passes
        act(() => {
            observer.emit(497.1, {clientHeight: 497});
            flushRafQueue();
        });
        expect(getRenderedHeight(container)).toBe("497");
    });

    it("falls back to contentRect when clientHeight is 0 (transient CSSOM state)", () => {
        act(() => {
            root.render(<HeightProbe minHeight={50}/>);
        });

        const observer = MockResizeObserver.instances[0];

        // clientHeight=0 but contentRect has real value → should use contentRect floor
        act(() => {
            observer.emit(200.7, {clientHeight: 0});
            flushRafQueue();
        });
        // Math.floor(200.7) = 200, and max(minHeight=50, 200) = 200
        expect(getRenderedHeight(container)).toBe("200");
    });
});
