import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";

const apiMock = vi.hoisted(() => ({
    DevPanelSearchFiles: vi.fn<(sessionName: string, query: string) => Promise<Array<{path: string}>>>(),
}));

let mockActiveSession = "session-a";
let mockSessions = [
    {id: 1, name: "session-a"},
    {id: 2, name: "session-b"},
];

vi.mock("../src/api", () => ({
    api: {
        DevPanelSearchFiles: (...args: [string, string]) => apiMock.DevPanelSearchFiles(...args),
    },
}));

vi.mock("../src/stores/tmuxStore", () => ({
    useTmuxStore: (selector: (state: {sessions: typeof mockSessions; activeSession: string | null}) => unknown) => (
        selector({sessions: mockSessions, activeSession: mockActiveSession})
    ),
}));

import {useFileSearch} from "../src/components/viewer/views/file-tree/useFileSearch";

type HookResult = ReturnType<typeof useFileSearch>;

let hookResult: HookResult | null = null;

function deferred<T>() {
    let resolve!: (value: T) => void;
    let reject!: (reason?: unknown) => void;
    const promise = new Promise<T>((innerResolve, innerReject) => {
        resolve = innerResolve;
        reject = innerReject;
    });
    return {promise, resolve, reject};
}

function Probe() {
    hookResult = useFileSearch();
    return (
        <div>
            <output data-testid="query">{hookResult.query}</output>
            <output data-testid="count">{String(hookResult.results.length)}</output>
            <output data-testid="searching">{String(hookResult.isSearching)}</output>
            <output data-testid="error">{hookResult.searchError ?? ""}</output>
        </div>
    );
}

function getProbeText(container: HTMLElement, testId: string): string {
    return container.querySelector(`[data-testid="${testId}"]`)?.textContent ?? "";
}

describe("useFileSearch", () => {
    let container: HTMLDivElement;
    let root: Root;

    beforeEach(() => {
        vi.useFakeTimers();
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        hookResult = null;
        mockActiveSession = "session-a";
        mockSessions = [
            {id: 1, name: "session-a"},
            {id: 2, name: "session-b"},
        ];
        apiMock.DevPanelSearchFiles.mockReset();
        (globalThis as {IS_REACT_ACT_ENVIRONMENT?: boolean}).IS_REACT_ACT_ENVIRONMENT = true;
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        container.remove();
        vi.useRealTimers();
        vi.restoreAllMocks();
        (globalThis as {IS_REACT_ACT_ENVIRONMENT?: boolean}).IS_REACT_ACT_ENVIRONMENT = false;
    });

    it("clears the loading flag after a failed search request", async () => {
        vi.spyOn(console, "error").mockImplementation(() => undefined);
        apiMock.DevPanelSearchFiles.mockRejectedValueOnce(new Error("boom"));

        act(() => {
            root.render(<Probe/>);
        });

        act(() => {
            hookResult!.setQuery("app");
        });
        await act(async () => {
            vi.advanceTimersByTime(300);
            await Promise.resolve();
        });
        await act(async () => {
            await Promise.resolve();
        });

        expect(getProbeText(container, "count")).toBe("0");
        expect(getProbeText(container, "searching")).toBe("false");
        expect(getProbeText(container, "error")).toBe("boom");
    });

    it("ignores stale search results after the active session changes", async () => {
        const pendingSearch = deferred<Array<{path: string}>>();
        apiMock.DevPanelSearchFiles.mockReturnValueOnce(pendingSearch.promise);

        act(() => {
            root.render(<Probe/>);
        });

        act(() => {
            hookResult!.setQuery("app");
        });
        await act(async () => {
            vi.advanceTimersByTime(300);
            await Promise.resolve();
        });

        expect(apiMock.DevPanelSearchFiles).toHaveBeenCalledWith("session-a", "app");
        expect(getProbeText(container, "searching")).toBe("true");

        mockActiveSession = "session-b";
        act(() => {
            root.render(<Probe/>);
        });

        await act(async () => {
            pendingSearch.resolve([{path: "src/app.ts"}]);
            await pendingSearch.promise;
        });

        expect(getProbeText(container, "query")).toBe("");
        expect(getProbeText(container, "count")).toBe("0");
        expect(getProbeText(container, "error")).toBe("");
        expect(getProbeText(container, "searching")).toBe("false");
    });

    it("ignores stale same-session responses after a newer query starts", async () => {
        const firstSearch = deferred<Array<{path: string}>>();
        const secondSearch = deferred<Array<{path: string}>>();
        apiMock.DevPanelSearchFiles
            .mockReturnValueOnce(firstSearch.promise)
            .mockReturnValueOnce(secondSearch.promise);

        act(() => {
            root.render(<Probe/>);
        });

        act(() => {
            hookResult!.setQuery("old");
        });
        await act(async () => {
            vi.advanceTimersByTime(300);
            await Promise.resolve();
        });

        act(() => {
            hookResult!.setQuery("new");
        });
        await act(async () => {
            vi.advanceTimersByTime(300);
            await Promise.resolve();
        });

        expect(apiMock.DevPanelSearchFiles).toHaveBeenNthCalledWith(1, "session-a", "old");
        expect(apiMock.DevPanelSearchFiles).toHaveBeenNthCalledWith(2, "session-a", "new");

        await act(async () => {
            firstSearch.resolve([{path: "src/old.ts"}]);
            await firstSearch.promise;
        });

        expect(getProbeText(container, "count")).toBe("0");
        expect(getProbeText(container, "searching")).toBe("true");

        await act(async () => {
            secondSearch.resolve([{path: "src/new.ts"}]);
            await secondSearch.promise;
        });

        expect(getProbeText(container, "query")).toBe("new");
        expect(getProbeText(container, "count")).toBe("1");
        expect(getProbeText(container, "searching")).toBe("false");
        expect(getProbeText(container, "error")).toBe("");
    });
});
