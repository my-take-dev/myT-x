/**
 * C-3 regression test: Verify that `isLoading` is always reset to `false` via `.finally()`,
 * even when the `.then()` handler encounters an unexpected exception.
 *
 * Before the `.finally()` fix, `setIsLoading(false)` lived inside `.then()`. If any
 * statement in the `.then()` callback threw, `isLoading` would remain `true` forever.
 */
import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";

// ── Mocks ──

const apiMock = vi.hoisted(() => ({
    DevPanelWorkingDiff: vi.fn<() => Promise<unknown>>(),
}));

let mockActiveSession: string | null = "test-session";

vi.mock("../src/api", () => ({
    api: {
        DevPanelWorkingDiff: (...args: unknown[]) => apiMock.DevPanelWorkingDiff(...(args as [])),
    },
}));

vi.mock("../src/stores/tmuxStore", () => ({
    useTmuxStore: (selector: (state: { activeSession: string | null }) => unknown) =>
        selector({activeSession: mockActiveSession}),
}));

import {useDiffView} from "../src/components/viewer/views/diff-view/useDiffView";

// ── Test component ──

function DiffViewProbe() {
    const {isLoading, error} = useDiffView();
    return (
        <div>
            <output data-testid="isLoading">{String(isLoading)}</output>
            <output data-testid="error">{error ?? ""}</output>
        </div>
    );
}

function getProbeText(container: HTMLElement, testId: string): string {
    return container.querySelector(`[data-testid="${testId}"]`)?.textContent ?? "";
}

// ── Tests ──

describe("useDiffView – .finally() loading reset", () => {
    let container: HTMLDivElement;
    let root: Root;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        mockActiveSession = "test-session";
        apiMock.DevPanelWorkingDiff.mockReset();
        vi.spyOn(console, "error").mockImplementation(() => {});
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

    it("resets isLoading to false after successful API response", async () => {
        apiMock.DevPanelWorkingDiff.mockResolvedValueOnce({files: []});

        act(() => {
            root.render(<DiffViewProbe/>);
        });
        await act(async () => {});

        expect(getProbeText(container, "isLoading")).toBe("false");
        expect(getProbeText(container, "error")).toBe("");
    });

    it("resets isLoading to false after API rejection", async () => {
        apiMock.DevPanelWorkingDiff.mockRejectedValueOnce(new Error("network failure"));

        act(() => {
            root.render(<DiffViewProbe/>);
        });
        await act(async () => {});

        expect(getProbeText(container, "isLoading")).toBe("false");
        expect(getProbeText(container, "error")).not.toBe("");
    });

    it("resets isLoading to false when .then() handler throws (regression: .finally() safety)", async () => {
        // Simulate an exception inside .then(): accessing result.files throws.
        // Before the .finally() fix, setIsLoading(false) would be skipped,
        // leaving isLoading stuck at true forever.
        const poisonedResult = Object.create(null);
        Object.defineProperty(poisonedResult, "files", {
            get(): never {
                throw new Error("simulated .then() exception");
            },
        });
        apiMock.DevPanelWorkingDiff.mockResolvedValueOnce(poisonedResult);

        act(() => {
            root.render(<DiffViewProbe/>);
        });
        await act(async () => {});

        expect(getProbeText(container, "isLoading")).toBe("false");
        // The exception is caught by .catch() which sets an error message.
        expect(getProbeText(container, "error")).not.toBe("");
    });

    it("does not call API when activeSession is null", async () => {
        mockActiveSession = null;

        act(() => {
            root.render(<DiffViewProbe/>);
        });
        await act(async () => {});

        expect(apiMock.DevPanelWorkingDiff).not.toHaveBeenCalled();
        expect(getProbeText(container, "isLoading")).toBe("false");
    });
});
