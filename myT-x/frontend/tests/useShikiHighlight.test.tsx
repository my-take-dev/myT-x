import {act, useEffect} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import type {ThemedToken} from "shiki/core";

const mocked = vi.hoisted(() => ({
    getHighlightSkipInfo: vi.fn(),
    highlightCode: vi.fn(),
    pathToShikiLang: vi.fn(),
    notifyHighlightFailure: vi.fn(),
}));

vi.mock("../src/utils/shikiHighlighter", () => ({
    getHighlightSkipInfo: (...args: [string]) => mocked.getHighlightSkipInfo(...args),
    highlightCode: (...args: [string, string]) => mocked.highlightCode(...args),
    pathToShikiLang: (...args: [string]) => mocked.pathToShikiLang(...args),
}));

vi.mock("../src/utils/notifyUtils", () => ({
    notifyHighlightFailure: () => mocked.notifyHighlightFailure(),
}));

import {useShikiHighlight, _resetLoggedSkippedHighlights} from "../src/hooks/useShikiHighlight";

interface HookValue {
    tokens: ThemedToken[][] | null;
    skipInfo: {
        reason: "size-limit" | "line-count-limit" | "line-length-limit";
        limit: number;
        actual: number;
    } | null;
    isHighlightFailed: boolean;
}

function HookProbe({
    code,
    filePath,
    lang,
    onValue,
}: {
    code?: string;
    filePath?: string;
    lang?: string;
    onValue: (value: HookValue) => void;
}) {
    const value = useShikiHighlight(code, filePath, lang);
    useEffect(() => {
        onValue(value);
    }, [onValue, value]);
    return null;
}

function createDeferred<T>() {
    let resolve!: (value: T) => void;
    let reject!: (reason?: unknown) => void;
    const promise = new Promise<T>((res, rej) => {
        resolve = res;
        reject = rej;
    });
    return {promise, resolve, reject};
}

async function flushMicrotasks(): Promise<void> {
    await Promise.resolve();
    await Promise.resolve();
}

describe("useShikiHighlight", () => {
    let container: HTMLDivElement;
    let root: Root;
    let latest: HookValue | null;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        latest = null;

        mocked.getHighlightSkipInfo.mockReset();
        mocked.highlightCode.mockReset();
        mocked.pathToShikiLang.mockReset();
        mocked.notifyHighlightFailure.mockReset();

        // Reset module-scope dedup Set to prevent test pollution.
        // Without this, a prior test that triggers a skip-warning suppresses
        // the warning in subsequent tests (silent false-negative).
        _resetLoggedSkippedHighlights();

        mocked.getHighlightSkipInfo.mockReturnValue(null);
        mocked.pathToShikiLang.mockReturnValue("typescript");
        mocked.highlightCode.mockResolvedValue([[{content: "x", color: "#fff"}]]);
        vi.spyOn(console, "warn").mockImplementation(() => undefined);
        vi.spyOn(console, "error").mockImplementation(() => undefined);
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

    it("returns idle state when code is undefined", () => {
        act(() => {
            root.render(
                <HookProbe
                    code={undefined}
                    filePath="a.ts"
                    onValue={(value) => {
                        latest = value;
                    }}
                />,
            );
        });

        expect(latest).toEqual({
            tokens: null,
            skipInfo: null,
            isHighlightFailed: false,
        });
        expect(mocked.highlightCode).not.toHaveBeenCalled();
    });

    it("returns skipInfo when guard blocks highlighting", () => {
        mocked.getHighlightSkipInfo.mockReturnValue({
            reason: "size-limit",
            limit: 10,
            actual: 99,
        });

        act(() => {
            root.render(
                <HookProbe
                    code={"x".repeat(99)}
                    filePath="big.ts"
                    onValue={(value) => {
                        latest = value;
                    }}
                />,
            );
        });

        expect(latest?.tokens).toBeNull();
        expect(latest?.skipInfo).toEqual({
            reason: "size-limit",
            limit: 10,
            actual: 99,
        });
        expect(latest?.isHighlightFailed).toBe(false);
        expect(mocked.highlightCode).not.toHaveBeenCalled();
    });

    it("returns tokens on successful highlight", async () => {
        act(() => {
            root.render(
                <HookProbe
                    code={"const x = 1;"}
                    filePath="a.ts"
                    onValue={(value) => {
                        latest = value;
                    }}
                />,
            );
        });

        await act(async () => {
            await flushMicrotasks();
        });

        expect(mocked.highlightCode).toHaveBeenCalledWith("const x = 1;", "typescript");
        expect(latest?.tokens).not.toBeNull();
        expect(latest?.skipInfo).toBeNull();
        expect(latest?.isHighlightFailed).toBe(false);
    });

    it("prefers explicit lang over filePath language detection", async () => {
        act(() => {
            root.render(
                <HookProbe
                    code={"const x = 1;"}
                    filePath="ignored.md"
                    lang="typescript"
                    onValue={(value) => {
                        latest = value;
                    }}
                />,
            );
        });

        await act(async () => {
            await flushMicrotasks();
        });

        expect(mocked.pathToShikiLang).not.toHaveBeenCalled();
        expect(mocked.highlightCode).toHaveBeenCalledWith("const x = 1;", "typescript");
        expect(latest?.isHighlightFailed).toBe(false);
    });

    it("stays idle when no language can be resolved from filePath", () => {
        mocked.pathToShikiLang.mockReturnValue(null);

        act(() => {
            root.render(
                <HookProbe
                    code={"const x = 1;"}
                    filePath="README.unknownext"
                    onValue={(value) => {
                        latest = value;
                    }}
                />,
            );
        });

        expect(mocked.highlightCode).not.toHaveBeenCalled();
        expect(latest).toEqual({
            tokens: null,
            skipInfo: null,
            isHighlightFailed: false,
        });
    });

    it("marks failure and notifies when highlight rejects", async () => {
        mocked.highlightCode.mockRejectedValue(new Error("boom"));

        act(() => {
            root.render(
                <HookProbe
                    code={"const x = 1;"}
                    filePath="a.ts"
                    onValue={(value) => {
                        latest = value;
                    }}
                />,
            );
        });

        await act(async () => {
            await flushMicrotasks();
        });

        expect(latest?.tokens).toBeNull();
        expect(latest?.skipInfo).toBeNull();
        expect(latest?.isHighlightFailed).toBe(true);
        expect(mocked.notifyHighlightFailure).toHaveBeenCalledTimes(1);
    });

    it("reverts to idle (not failed) when highlight returns null after attempt", async () => {
        mocked.highlightCode.mockResolvedValue(null);

        act(() => {
            root.render(
                <HookProbe
                    code={"const x = 1;"}
                    filePath="a.ts"
                    onValue={(value) => {
                        latest = value;
                    }}
                />,
            );
        });

        await act(async () => {
            await flushMicrotasks();
        });

        // null result means cooldown skip or unknown lang — not a failure.
        expect(latest?.tokens).toBeNull();
        expect(latest?.skipInfo).toBeNull();
        expect(latest?.isHighlightFailed).toBe(false);
        expect(mocked.notifyHighlightFailure).not.toHaveBeenCalled();
    });

    it("resets to idle state when code changes from a value to undefined", async () => {
        // First render with actual code — wait for tokens
        act(() => {
            root.render(
                <HookProbe
                    code="const x = 1;"
                    filePath="a.ts"
                    onValue={(value) => {
                        latest = value;
                    }}
                />,
            );
        });

        await act(async () => {
            await flushMicrotasks();
        });

        // Confirm we have tokens
        expect(latest?.tokens).not.toBeNull();
        expect(latest?.isHighlightFailed).toBe(false);

        // Re-render with code=undefined
        act(() => {
            root.render(
                <HookProbe
                    code={undefined}
                    filePath="a.ts"
                    onValue={(value) => {
                        latest = value;
                    }}
                />,
            );
        });

        // Should revert to idle state
        expect(latest).toEqual({
            tokens: null,
            skipInfo: null,
            isHighlightFailed: false,
        });
    });

    it("ignores stale async results after inputs change", async () => {
        const first = createDeferred<ThemedToken[][] | null>();
        const second = createDeferred<ThemedToken[][] | null>();
        mocked.highlightCode.mockImplementation((code: string) => {
            if (code === "first") return first.promise;
            return second.promise;
        });

        act(() => {
            root.render(
                <HookProbe
                    code="first"
                    filePath="a.ts"
                    onValue={(value) => {
                        latest = value;
                    }}
                />,
            );
        });

        act(() => {
            root.render(
                <HookProbe
                    code="second"
                    filePath="a.ts"
                    onValue={(value) => {
                        latest = value;
                    }}
                />,
            );
        });

        await act(async () => {
            first.resolve([[{content: "first", color: "#111"}]]);
            await flushMicrotasks();
        });
        expect(latest?.tokens).toBeNull();

        await act(async () => {
            second.resolve([[{content: "second", color: "#222"}]]);
            await flushMicrotasks();
        });
        expect(latest?.tokens?.[0]?.[0]?.content).toBe("second");
    });

    it("does not update state when unmounted during in-flight highlight", async () => {
        const deferred = createDeferred<ThemedToken[][] | null>();
        mocked.highlightCode.mockReturnValue(deferred.promise);

        // 1. Start a highlight (mock highlightCode to return a deferred promise)
        act(() => {
            root.render(
                <HookProbe
                    code="const unmount = 1;"
                    filePath="a.ts"
                    onValue={(value) => {
                        latest = value;
                    }}
                />,
            );
        });

        // Highlight is in-flight, state should be idle
        expect(latest?.tokens).toBeNull();
        expect(latest?.isHighlightFailed).toBe(false);

        // 2. Unmount the component
        act(() => {
            root.unmount();
        });

        // 3. Resolve the deferred promise after unmount
        await act(async () => {
            deferred.resolve([[{content: "unmount", color: "#333"}]]);
            await flushMicrotasks();
        });

        // 4. No errors/warnings about unmounted setState should occur
        //    (the cancelled flag prevents the setState call).
        //    Re-create root for afterEach cleanup
        root = createRoot(container);
    });

    it("dedup: second null result for same lang+filePath does not emit console.warn again", async () => {
        mocked.highlightCode.mockResolvedValue(null);
        const warnSpy = vi.mocked(console.warn);

        // First render — null result triggers warn.
        act(() => {
            root.render(
                <HookProbe
                    code="const a = 1;"
                    filePath="dedup.ts"
                    onValue={(value) => {
                        latest = value;
                    }}
                />,
            );
        });
        await act(async () => {
            await flushMicrotasks();
        });

        const warnCountAfterFirst = warnSpy.mock.calls.length;
        expect(warnCountAfterFirst).toBe(1);

        // Re-render with different code but same filePath — triggers highlight again.
        act(() => {
            root.render(
                <HookProbe
                    code="const b = 2;"
                    filePath="dedup.ts"
                    onValue={(value) => {
                        latest = value;
                    }}
                />,
            );
        });
        await act(async () => {
            await flushMicrotasks();
        });

        // Second null result for same lang+filePath should NOT emit another warn.
        expect(warnSpy.mock.calls.length).toBe(warnCountAfterFirst);
    });

    it("ignores stale rejection after inputs change", async () => {
        const first = createDeferred<ThemedToken[][] | null>();
        const second = createDeferred<ThemedToken[][] | null>();
        mocked.highlightCode.mockImplementation((code: string) => {
            if (code === "first") return first.promise;
            return second.promise;
        });

        act(() => {
            root.render(
                <HookProbe
                    code="first"
                    filePath="a.ts"
                    onValue={(value) => {
                        latest = value;
                    }}
                />,
            );
        });

        act(() => {
            root.render(
                <HookProbe
                    code="second"
                    filePath="a.ts"
                    onValue={(value) => {
                        latest = value;
                    }}
                />,
            );
        });

        await act(async () => {
            first.reject(new Error("stale failure"));
            await flushMicrotasks();
        });

        expect(mocked.notifyHighlightFailure).not.toHaveBeenCalled();

        await act(async () => {
            second.resolve([[{content: "second", color: "#222"}]]);
            await flushMicrotasks();
        });

        expect(latest?.tokens?.[0]?.[0]?.content).toBe("second");
    });
});
