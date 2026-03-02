/**
 * C-3: dirError set/clear path tests
 * C-4: setExpandedPathsAndSyncRef immediate sync tests
 * T-3: toggleDir concurrent request guard tests
 *
 * Tests the useFileTree hook's directory error handling,
 * expandedPaths ref synchronization, and toggleDir race condition guards.
 */
import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";

// ── Mocks ──

const apiMock = vi.hoisted(() => ({
    DevPanelListDir: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
    DevPanelReadFile: vi.fn<(...args: unknown[]) => Promise<unknown>>(),
}));

let mockActiveSession: string | null = "test-session";

vi.mock("../src/api", () => ({
    api: {
        DevPanelListDir: (...args: unknown[]) => apiMock.DevPanelListDir(...args),
        DevPanelReadFile: (...args: unknown[]) => apiMock.DevPanelReadFile(...args),
    },
}));

vi.mock("../src/stores/tmuxStore", () => ({
    useTmuxStore: (selector: (state: { activeSession: string | null }) => unknown) =>
        selector({activeSession: mockActiveSession}),
}));

import type {FileEntry} from "../src/components/viewer/views/file-tree/fileTreeTypes";
import {useFileTree, type UseFileTreeResult} from "../src/components/viewer/views/file-tree/useFileTree";

// ── Test component ──

let hookResult: UseFileTreeResult | null = null;

function FileTreeProbe() {
    hookResult = useFileTree();
    return (
        <div>
            <output data-testid="dirError">{hookResult.dirError ?? ""}</output>
            <output data-testid="contentError">{hookResult.contentError ?? ""}</output>
            <output data-testid="error">{hookResult.error ?? ""}</output>
            <output data-testid="isRootLoading">{String(hookResult.isRootLoading)}</output>
            <output data-testid="flatNodesCount">{hookResult.flatNodes.length}</output>
        </div>
    );
}

function getProbeText(container: HTMLElement, testId: string): string {
    return container.querySelector(`[data-testid="${testId}"]`)?.textContent ?? "";
}

// ── Helpers ──

function makeFileEntry(name: string, isDir: boolean): FileEntry {
    return {name, path: name, is_dir: isDir, size: isDir ? 0 : 100};
}

// ── Tests ──

describe("useFileTree – dirError lifecycle (C-3)", () => {
    let container: HTMLDivElement;
    let root: Root;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        hookResult = null;
        mockActiveSession = "test-session";
        apiMock.DevPanelListDir.mockReset();
        apiMock.DevPanelReadFile.mockReset();
        vi.spyOn(console, "error").mockImplementation(() => {});
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        hookResult = null;
        container.remove();
        vi.restoreAllMocks();
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = false;
    });

    it("sets dirError when toggleDir API call fails", async () => {
        // Root loading succeeds with a directory entry.
        const rootEntries = [makeFileEntry("src", true)];
        apiMock.DevPanelListDir
            .mockResolvedValueOnce(rootEntries)   // loadRoot
            .mockRejectedValueOnce(new Error("permission denied")); // toggleDir("src")

        act(() => {
            root.render(<FileTreeProbe/>);
        });
        // Wait for loadRoot to complete.
        await act(async () => {});

        expect(getProbeText(container, "dirError")).toBe("");
        expect(getProbeText(container, "error")).toBe("");

        // Expand the "src" directory – this will fail.
        act(() => {
            hookResult!.toggleDir("src");
        });
        await act(async () => {});

        // dirError should be set, but root-level error should remain null.
        expect(getProbeText(container, "dirError")).toContain("permission denied");
        expect(getProbeText(container, "error")).toBe("");
    });

    it("clears dirError when a subsequent toggleDir expansion succeeds", async () => {
        const rootEntries = [makeFileEntry("src", true), makeFileEntry("lib", true)];
        apiMock.DevPanelListDir
            .mockResolvedValueOnce(rootEntries)                         // loadRoot
            .mockRejectedValueOnce(new Error("network timeout"))        // toggleDir("src") fails
            .mockResolvedValueOnce([makeFileEntry("lib/a.ts", false)]); // toggleDir("lib") succeeds

        act(() => {
            root.render(<FileTreeProbe/>);
        });
        await act(async () => {});

        // Expand "src" – fails, dirError is set.
        act(() => {
            hookResult!.toggleDir("src");
        });
        await act(async () => {});
        expect(getProbeText(container, "dirError")).toContain("network timeout");

        // Expand "lib" – succeeds, dirError is cleared at expansion start.
        act(() => {
            hookResult!.toggleDir("lib");
        });
        await act(async () => {});
        expect(getProbeText(container, "dirError")).toBe("");
    });

    it("clears dirError on session change", async () => {
        const rootEntries = [makeFileEntry("src", true)];
        apiMock.DevPanelListDir
            .mockResolvedValueOnce(rootEntries)                    // loadRoot (session-1)
            .mockRejectedValueOnce(new Error("dir error"))         // toggleDir fails
            .mockResolvedValueOnce([]);                            // loadRoot (session-2)

        act(() => {
            root.render(<FileTreeProbe/>);
        });
        await act(async () => {});

        // Cause dirError.
        act(() => {
            hookResult!.toggleDir("src");
        });
        await act(async () => {});
        expect(getProbeText(container, "dirError")).toContain("dir error");

        // Switch session – dirError must be cleared.
        mockActiveSession = "session-2";
        act(() => {
            root.render(<FileTreeProbe/>);
        });
        await act(async () => {});

        expect(getProbeText(container, "dirError")).toBe("");
    });

    it("clears dirError on loadRoot refresh", async () => {
        const rootEntries = [makeFileEntry("src", true)];
        apiMock.DevPanelListDir
            .mockResolvedValueOnce(rootEntries)            // loadRoot
            .mockRejectedValueOnce(new Error("tmp error")) // toggleDir fails
            .mockResolvedValueOnce(rootEntries);           // loadRoot (refresh)

        act(() => {
            root.render(<FileTreeProbe/>);
        });
        await act(async () => {});

        act(() => {
            hookResult!.toggleDir("src");
        });
        await act(async () => {});
        expect(getProbeText(container, "dirError")).toContain("tmp error");

        // Manual refresh – loadRoot clears dirError.
        act(() => {
            hookResult!.loadRoot();
        });
        await act(async () => {});
        expect(getProbeText(container, "dirError")).toBe("");
    });

    it("clears dirError when retrying the same directory succeeds (I-1)", async () => {
        const rootEntries = [makeFileEntry("src", true)];
        apiMock.DevPanelListDir
            .mockResolvedValueOnce(rootEntries)                         // loadRoot
            .mockRejectedValueOnce(new Error("first attempt failed"))   // toggleDir("src") fails
            .mockResolvedValueOnce([makeFileEntry("src/a.ts", false)]); // toggleDir("src") retried, succeeds

        act(() => {
            root.render(<FileTreeProbe/>);
        });
        await act(async () => {});

        // Expand "src" – fails, dirError is set.
        act(() => {
            hookResult!.toggleDir("src");
        });
        await act(async () => {});
        expect(getProbeText(container, "dirError")).toContain("first attempt failed");

        // Retry same directory — "src" is not in cache (first call failed),
        // not in expandedPaths (expansion never completed), so this is a fresh expand.
        act(() => {
            hookResult!.toggleDir("src");
        });
        await act(async () => {});

        // dirError should be cleared by the successful .then() handler.
        expect(getProbeText(container, "dirError")).toBe("");
        // Directory should be expanded with children.
        expect(hookResult!.flatNodes.length).toBe(2); // src + src/a.ts
    });

    it("clears dirError when selectFile is called (I-4)", async () => {
        const rootEntries = [makeFileEntry("src", true), makeFileEntry("readme.md", false)];
        apiMock.DevPanelListDir
            .mockResolvedValueOnce(rootEntries)                    // loadRoot
            .mockRejectedValueOnce(new Error("expand failed"));    // toggleDir("src")
        apiMock.DevPanelReadFile.mockResolvedValueOnce("file content");

        act(() => {
            root.render(<FileTreeProbe/>);
        });
        await act(async () => {});

        // Expand "src" – fails, dirError is set.
        act(() => {
            hookResult!.toggleDir("src");
        });
        await act(async () => {});
        expect(getProbeText(container, "dirError")).toContain("expand failed");

        // Select a file – dirError should be cleared.
        act(() => {
            hookResult!.selectFile("readme.md");
        });
        await act(async () => {});
        expect(getProbeText(container, "dirError")).toBe("");
    });

    it("collapse clears dirError but preserves contentError (I-3)", async () => {
        const rootEntries = [makeFileEntry("src", true), makeFileEntry("lib", true), makeFileEntry("readme.md", false)];
        const srcChildren = [makeFileEntry("src/a.ts", false)];
        apiMock.DevPanelListDir
            .mockResolvedValueOnce(rootEntries)                   // loadRoot
            .mockResolvedValueOnce(srcChildren)                   // toggleDir("src") expand
            .mockRejectedValueOnce(new Error("lib failed"));      // toggleDir("lib") fails
        apiMock.DevPanelReadFile.mockRejectedValueOnce(new Error("read failed")); // selectFile fails

        act(() => {
            root.render(<FileTreeProbe/>);
        });
        await act(async () => {});

        // Expand "src" successfully.
        act(() => {
            hookResult!.toggleDir("src");
        });
        await act(async () => {});

        // Select a file – fails, contentError is set.
        act(() => {
            hookResult!.selectFile("readme.md");
        });
        await act(async () => {});
        expect(getProbeText(container, "contentError")).toContain("read failed");

        // Expand "lib" – fails, dirError is set.
        act(() => {
            hookResult!.toggleDir("lib");
        });
        await act(async () => {});
        expect(getProbeText(container, "dirError")).toContain("lib failed");

        // Collapse "src" – dirError is cleared but contentError persists
        // because contentError belongs to selectFile's scope, not toggleDir.
        act(() => {
            hookResult!.toggleDir("src");
        });
        expect(getProbeText(container, "dirError")).toBe("");
        expect(getProbeText(container, "contentError")).toContain("read failed");
    });

    it("clears dirError when collapsing a directory (S-5)", async () => {
        const rootEntries = [makeFileEntry("src", true), makeFileEntry("lib", true)];
        const srcChildren = [makeFileEntry("src/a.ts", false)];
        apiMock.DevPanelListDir
            .mockResolvedValueOnce(rootEntries)                 // loadRoot
            .mockResolvedValueOnce(srcChildren)                 // toggleDir("src") expand
            .mockRejectedValueOnce(new Error("lib failed"));    // toggleDir("lib") fails

        act(() => {
            root.render(<FileTreeProbe/>);
        });
        await act(async () => {});

        // Expand "src" successfully.
        act(() => {
            hookResult!.toggleDir("src");
        });
        await act(async () => {});

        // Expand "lib" – fails, dirError is set.
        act(() => {
            hookResult!.toggleDir("lib");
        });
        await act(async () => {});
        expect(getProbeText(container, "dirError")).toContain("lib failed");

        // Collapse "src" – dirError should be cleared.
        act(() => {
            hookResult!.toggleDir("src");
        });
        expect(getProbeText(container, "dirError")).toBe("");
    });
});

describe("useFileTree – loadRoot finally guard (I-7)", () => {
    let container: HTMLDivElement;
    let root: Root;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        hookResult = null;
        mockActiveSession = "session-1";
        apiMock.DevPanelListDir.mockReset();
        apiMock.DevPanelReadFile.mockReset();
        vi.spyOn(console, "error").mockImplementation(() => {});
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        hookResult = null;
        container.remove();
        vi.restoreAllMocks();
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = false;
    });

    it("stale loadRoot finally does not set isRootLoading(false) after session switch", async () => {
        // Deferred promise for first session's loadRoot.
        let resolveFirstLoad!: (entries: FileEntry[]) => void;
        const firstLoadPromise = new Promise<FileEntry[]>((res) => {
            resolveFirstLoad = res;
        });

        apiMock.DevPanelListDir
            .mockReturnValueOnce(firstLoadPromise)       // session-1 loadRoot (held)
            .mockResolvedValueOnce([]);                   // session-2 loadRoot (immediate)

        // Mount with session-1 — loadRoot starts, isRootLoading=true.
        act(() => {
            root.render(<FileTreeProbe/>);
        });
        // Allow the effect to fire (loadRoot called).
        await act(async () => {});

        expect(getProbeText(container, "isRootLoading")).toBe("true");

        // Switch to session-2 — triggers reset and a new loadRoot.
        mockActiveSession = "session-2";
        act(() => {
            root.render(<FileTreeProbe/>);
        });
        await act(async () => {});

        // session-2 loadRoot has completed (empty entries).
        expect(getProbeText(container, "isRootLoading")).toBe("false");

        // Now resolve session-1's stale loadRoot promise.
        await act(async () => {
            resolveFirstLoad([makeFileEntry("stale-dir", true)]);
        });

        // isRootLoading should remain false — stale finally was skipped.
        expect(getProbeText(container, "isRootLoading")).toBe("false");
        // Root entries should be empty (session-2's result), not stale data.
        expect(getProbeText(container, "flatNodesCount")).toBe("0");
    });
});

describe("useFileTree – selectFile stale finally guard (I-9)", () => {
    let container: HTMLDivElement;
    let root: Root;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        hookResult = null;
        mockActiveSession = "session-1";
        apiMock.DevPanelListDir.mockReset();
        apiMock.DevPanelReadFile.mockReset();
        vi.spyOn(console, "error").mockImplementation(() => {});
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        hookResult = null;
        container.remove();
        vi.restoreAllMocks();
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = false;
    });

    it("stale selectFile finally does not affect isLoadingContent after session switch", async () => {
        let resolveFirstRead!: (content: unknown) => void;
        const firstReadPromise = new Promise((res) => {
            resolveFirstRead = res;
        });

        const rootEntries = [makeFileEntry("readme.md", false)];
        apiMock.DevPanelListDir
            .mockResolvedValueOnce(rootEntries)   // session-1 loadRoot
            .mockResolvedValueOnce([]);            // session-2 loadRoot
        apiMock.DevPanelReadFile.mockReturnValueOnce(firstReadPromise); // session-1 selectFile (held)

        act(() => {
            root.render(<FileTreeProbe/>);
        });
        await act(async () => {});

        // Start selectFile — isLoadingContent should become true.
        act(() => {
            hookResult!.selectFile("readme.md");
        });
        await act(async () => {});
        expect(hookResult!.isLoadingContent).toBe(true);

        // Switch to session-2 — triggers reset (isLoadingContent -> false).
        mockActiveSession = "session-2";
        act(() => {
            root.render(<FileTreeProbe/>);
        });
        await act(async () => {});
        expect(hookResult!.isLoadingContent).toBe(false);

        // Resolve the stale selectFile promise from session-1.
        await act(async () => {
            resolveFirstRead({path: "readme.md", content: "stale", line_count: 1, size: 5, truncated: false, binary: false});
        });

        // isLoadingContent should remain false — stale data should not leak.
        expect(hookResult!.isLoadingContent).toBe(false);
        // fileContent should be null (session-2 has no selected file).
        expect(hookResult!.fileContent).toBeNull();
    });
});

describe("useFileTree – expandedPaths ref sync (C-4)", () => {
    let container: HTMLDivElement;
    let root: Root;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        hookResult = null;
        mockActiveSession = "test-session";
        apiMock.DevPanelListDir.mockReset();
        apiMock.DevPanelReadFile.mockReset();
        vi.spyOn(console, "error").mockImplementation(() => {});
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        hookResult = null;
        container.remove();
        vi.restoreAllMocks();
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = false;
    });

    it("rapid expand+collapse keeps expandedPaths consistent with flatNodes", async () => {
        // Root has two directories with cached children to avoid async API calls.
        const rootEntries = [makeFileEntry("a", true), makeFileEntry("b", true)];
        const aChildren = [makeFileEntry("a/file.ts", false)];
        const bChildren = [makeFileEntry("b/file.ts", false)];

        apiMock.DevPanelListDir
            .mockResolvedValueOnce(rootEntries) // loadRoot
            .mockResolvedValueOnce(aChildren)   // toggleDir("a")
            .mockResolvedValueOnce(bChildren);  // toggleDir("b")

        act(() => {
            root.render(<FileTreeProbe/>);
        });
        await act(async () => {});

        // Expand "a" (API call).
        act(() => {
            hookResult!.toggleDir("a");
        });
        await act(async () => {});

        // Expand "b" (API call).
        act(() => {
            hookResult!.toggleDir("b");
        });
        await act(async () => {});

        // Both should be expanded: root(2) + a/file(1) + b/file(1) = 4 nodes.
        expect(hookResult!.flatNodes.length).toBe(4);

        // Rapid collapse of "a" then "b" – synchronous operations, no API needed.
        act(() => {
            hookResult!.toggleDir("a");
            hookResult!.toggleDir("b");
        });

        // Both collapsed: only root entries remain (2 nodes).
        expect(hookResult!.flatNodes.length).toBe(2);
    });

    it("re-expanding a cached directory does not make API call", async () => {
        const rootEntries = [makeFileEntry("src", true)];
        const srcChildren = [makeFileEntry("src/index.ts", false)];

        apiMock.DevPanelListDir
            .mockResolvedValueOnce(rootEntries) // loadRoot
            .mockResolvedValueOnce(srcChildren); // first toggleDir("src")

        act(() => {
            root.render(<FileTreeProbe/>);
        });
        await act(async () => {});

        // Expand "src" (API call).
        act(() => {
            hookResult!.toggleDir("src");
        });
        await act(async () => {});
        expect(hookResult!.flatNodes.length).toBe(2); // src + src/index.ts

        const callCountAfterFirstExpand = apiMock.DevPanelListDir.mock.calls.length;

        // Collapse "src".
        act(() => {
            hookResult!.toggleDir("src");
        });
        expect(hookResult!.flatNodes.length).toBe(1);

        // Re-expand "src" – should use cache, no new API call.
        act(() => {
            hookResult!.toggleDir("src");
        });
        expect(hookResult!.flatNodes.length).toBe(2);
        expect(apiMock.DevPanelListDir.mock.calls.length).toBe(callCountAfterFirstExpand);
    });

    it("collapse after expand correctly reflects in flatNodes without stale ref", async () => {
        const rootEntries = [makeFileEntry("dir", true)];
        const dirChildren = [makeFileEntry("dir/a.ts", false), makeFileEntry("dir/b.ts", false)];

        apiMock.DevPanelListDir
            .mockResolvedValueOnce(rootEntries) // loadRoot
            .mockResolvedValueOnce(dirChildren); // toggleDir("dir")

        act(() => {
            root.render(<FileTreeProbe/>);
        });
        await act(async () => {});

        // Expand.
        act(() => {
            hookResult!.toggleDir("dir");
        });
        await act(async () => {});
        expect(hookResult!.flatNodes.length).toBe(3);

        // Immediate collapse – tests that expandedPathsRef.current
        // was updated synchronously inside the state updater.
        act(() => {
            hookResult!.toggleDir("dir");
        });
        expect(hookResult!.flatNodes.length).toBe(1);

        // Immediate re-expand from cache – verifies ref is correct after collapse.
        act(() => {
            hookResult!.toggleDir("dir");
        });
        expect(hookResult!.flatNodes.length).toBe(3);
    });
});

describe("useFileTree – toggleDir loading lifecycle (I-5)", () => {
    let container: HTMLDivElement;
    let root: Root;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        hookResult = null;
        mockActiveSession = "test-session";
        apiMock.DevPanelListDir.mockReset();
        apiMock.DevPanelReadFile.mockReset();
        vi.spyOn(console, "error").mockImplementation(() => {});
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        hookResult = null;
        container.remove();
        vi.restoreAllMocks();
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = false;
    });

    it("clears loadingPaths after successful toggleDir expansion", async () => {
        const rootEntries = [makeFileEntry("src", true)];
        const srcChildren = [makeFileEntry("src/index.ts", false)];

        apiMock.DevPanelListDir
            .mockResolvedValueOnce(rootEntries)  // loadRoot
            .mockResolvedValueOnce(srcChildren); // toggleDir("src")

        act(() => {
            root.render(<FileTreeProbe/>);
        });
        await act(async () => {});

        // Before expand: no loading nodes.
        expect(hookResult!.flatNodes.some(n => n.isDir && n.isLoading)).toBe(false);

        // Expand "src" — after async resolve, loading should be cleared.
        act(() => {
            hookResult!.toggleDir("src");
        });
        await act(async () => {});

        // Directory is expanded, no loading indicator remains.
        const srcNode = hookResult!.flatNodes.find(n => n.path === "src");
        expect(srcNode).toBeDefined();
        expect(srcNode!.isDir).toBe(true);
        if (srcNode!.isDir) {
            expect(srcNode!.isLoading).toBe(false);
            expect(srcNode!.isExpanded).toBe(true);
        }
        expect(hookResult!.flatNodes.length).toBe(2); // src + src/index.ts
    });

    it("clears loadingPaths after failed toggleDir expansion", async () => {
        const rootEntries = [makeFileEntry("src", true)];

        apiMock.DevPanelListDir
            .mockResolvedValueOnce(rootEntries)                    // loadRoot
            .mockRejectedValueOnce(new Error("network error"));    // toggleDir("src")

        act(() => {
            root.render(<FileTreeProbe/>);
        });
        await act(async () => {});

        act(() => {
            hookResult!.toggleDir("src");
        });
        await act(async () => {});

        // Loading should be cleared even after failure.
        const srcNode = hookResult!.flatNodes.find(n => n.path === "src");
        expect(srcNode).toBeDefined();
        if (srcNode!.isDir) {
            expect(srcNode!.isLoading).toBe(false);
            expect(srcNode!.isExpanded).toBe(false); // stays collapsed on error
        }
    });
});

describe("useFileTree – toggleDir requestId guard (T-3)", () => {
    let container: HTMLDivElement;
    let root: Root;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        hookResult = null;
        mockActiveSession = "test-session";
        apiMock.DevPanelListDir.mockReset();
        apiMock.DevPanelReadFile.mockReset();
        vi.spyOn(console, "error").mockImplementation(() => {});
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        hookResult = null;
        container.remove();
        vi.restoreAllMocks();
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = false;
    });

    it("stale toggleDir response is discarded when same directory is re-toggled", async () => {
        const rootEntries = [makeFileEntry("src", true)];

        // Deferred promises: 1st held, 2nd resolves immediately.
        let resolveFirstToggle!: (children: FileEntry[]) => void;
        const firstTogglePromise = new Promise<FileEntry[]>((res) => {
            resolveFirstToggle = res;
        });

        const secondToggleChildren = [makeFileEntry("src/new.ts", false)];

        // When the 1st toggleDir is still pending, "src" is not yet in
        // expandedPaths (it's added only in .then). So the 2nd toggleDir call
        // is also seen as an expand attempt → fires another API call.
        apiMock.DevPanelListDir
            .mockResolvedValueOnce(rootEntries)         // loadRoot
            .mockReturnValueOnce(firstTogglePromise)     // 1st toggleDir("src") — held
            .mockResolvedValueOnce(secondToggleChildren); // 2nd toggleDir("src") — resolves immediately

        act(() => {
            root.render(<FileTreeProbe/>);
        });
        await act(async () => {});

        // 1st expand — starts API call, held in pending state.
        act(() => {
            hookResult!.toggleDir("src");
        });
        await act(async () => {});

        // 2nd expand of the same directory while 1st is still pending.
        // Since 1st hasn't resolved, "src" is NOT in expandedPaths — this is
        // treated as another expand and fires a new API call.
        act(() => {
            hookResult!.toggleDir("src");
        });
        await act(async () => {});

        // 2nd toggleDir resolved with "src/new.ts".
        expect(hookResult!.flatNodes.length).toBe(2); // src + src/new.ts
        expect(hookResult!.flatNodes.some(n => n.path === "src/new.ts")).toBe(true);

        // Now resolve the stale 1st toggleDir — should be discarded by requestId guard.
        const staleChildren = [makeFileEntry("src/old.ts", false), makeFileEntry("src/stale.ts", false)];
        await act(async () => {
            resolveFirstToggle(staleChildren);
        });

        // Cache should NOT be overwritten with stale data.
        expect(hookResult!.flatNodes.length).toBe(2); // still src + src/new.ts
        expect(hookResult!.flatNodes.some(n => n.path === "src/old.ts")).toBe(false);
        expect(hookResult!.flatNodes.some(n => n.path === "src/stale.ts")).toBe(false);
    });

    it("stale toggleDir error is discarded when same directory has newer request", async () => {
        const rootEntries = [makeFileEntry("lib", true)];

        // Deferred promise for the first toggleDir call (will reject).
        let rejectFirstToggle!: (err: Error) => void;
        const firstTogglePromise = new Promise<FileEntry[]>((_res, rej) => {
            rejectFirstToggle = rej;
        });

        const secondToggleChildren = [makeFileEntry("lib/utils.ts", false)];

        apiMock.DevPanelListDir
            .mockResolvedValueOnce(rootEntries)          // loadRoot
            .mockReturnValueOnce(firstTogglePromise)      // 1st toggleDir("lib") — held
            .mockResolvedValueOnce(secondToggleChildren); // 2nd toggleDir("lib") — succeeds

        act(() => {
            root.render(<FileTreeProbe/>);
        });
        await act(async () => {});

        // 1st expand of "lib" — held.
        act(() => {
            hookResult!.toggleDir("lib");
        });
        await act(async () => {});

        // 2nd expand of same directory while 1st is pending.
        // "lib" is not in expandedPaths yet, so this is another expand attempt.
        act(() => {
            hookResult!.toggleDir("lib");
        });
        await act(async () => {});

        // 2nd request succeeded — directory is expanded.
        expect(hookResult!.flatNodes.length).toBe(2); // lib + lib/utils.ts
        expect(getProbeText(container, "dirError")).toBe("");

        // Now reject the stale 1st request — dirError should NOT be set.
        await act(async () => {
            rejectFirstToggle(new Error("stale network error"));
        });

        expect(getProbeText(container, "dirError")).toBe("");
        expect(hookResult!.flatNodes.length).toBe(2); // unchanged
    });
});
