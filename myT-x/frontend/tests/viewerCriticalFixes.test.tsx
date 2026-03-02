import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";

const mocked = vi.hoisted(() => ({
    parseDiffFiles: vi.fn(),
    computeHunkGaps: vi.fn(() => new Map()),
    diffHeaderToFilePath: vi.fn((header: string | null) => header ?? "(untitled)"),
}));

vi.mock("../src/utils/diffParser", () => ({
    parseDiffFiles: (raw: string) => mocked.parseDiffFiles(raw),
    computeHunkGaps: (hunks: unknown[]) => mocked.computeHunkGaps(hunks),
    diffHeaderToFilePath: (header: string | null) => mocked.diffHeaderToFilePath(header),
}));

import {DIFF_COLLAPSE_THRESHOLD, DiffViewer} from "../src/components/viewer/views/git-graph/DiffViewer";
import {CopyPathButton} from "../src/components/viewer/views/shared/CopyPathButton";

describe("critical viewer fixes", () => {
    let container: HTMLDivElement;
    let root: Root;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        mocked.parseDiffFiles.mockReset();
        mocked.computeHunkGaps.mockReset();
        mocked.computeHunkGaps.mockReturnValue(new Map());
        mocked.diffHeaderToFilePath.mockReset();
        mocked.diffHeaderToFilePath.mockImplementation((header: string | null) => header ?? "(untitled)");
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

    it("CopyPathButton does not forward MouseEvent to onClick callback", () => {
        const onClickWithOptionalArg = vi.fn((arg?: unknown) => arg);

        act(() => {
            root.render(
                <CopyPathButton
                    state="idle"
                    onClick={onClickWithOptionalArg as unknown as () => void}
                />,
            );
        });

        const button = container.querySelector("button");
        expect(button).toBeTruthy();
        act(() => {
            button?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });

        expect(onClickWithOptionalArg).toHaveBeenCalledTimes(1);
        expect(onClickWithOptionalArg.mock.calls[0]).toEqual([]);
    });

    it("DiffViewer shows error message from thrown Error when parsing throws", () => {
        mocked.parseDiffFiles.mockImplementation(() => {
            throw new Error("boom");
        });
        const consoleErrorSpy = vi.spyOn(console, "error").mockImplementation(() => undefined);

        act(() => {
            root.render(<DiffViewer diff="invalid diff"/>);
        });

        // toErrorMessage extracts the Error.message for user display;
        // full error details are also logged to console.
        const msgEl = container.querySelector(".viewer-message");
        expect(msgEl).not.toBeNull();
        expect(msgEl!.textContent).toBe("boom");
        expect(consoleErrorSpy).toHaveBeenCalled();
    });

    it("DiffViewer shows fallback message when parsing throws non-Error", () => {
        mocked.parseDiffFiles.mockImplementation(() => {
            throw 42; // eslint-disable-line no-throw-literal
        });
        vi.spyOn(console, "error").mockImplementation(() => undefined);

        act(() => {
            root.render(<DiffViewer diff="bad"/>);
        });

        const msgEl = container.querySelector(".viewer-message");
        expect(msgEl).not.toBeNull();
        expect(msgEl!.textContent).toBe("Failed to parse diff.");
    });

    it("DiffViewer shows 'No diff available' when diff produces zero files", () => {
        mocked.parseDiffFiles.mockReturnValue([]);

        act(() => {
            root.render(<DiffViewer diff=""/>);
        });

        const msgEl = container.querySelector(".viewer-message");
        expect(msgEl).not.toBeNull();
        expect(msgEl!.textContent).toBe("No diff available");
    });

    it("DiffViewer auto-collapses files when count exceeds threshold", () => {
        const files = Array.from({length: DIFF_COLLAPSE_THRESHOLD + 2}, (_, i) => ({
            header: `diff --git a/file${i}.ts b/file${i}.ts`,
            hunks: [{
                header: "@@ -1,1 +1,1 @@",
                lines: [{type: "context" as const, content: "x", oldLineNum: 1, newLineNum: 1}],
                startOldLine: 1,
                startNewLine: 1,
            }],
        }));
        mocked.parseDiffFiles.mockReturnValue(files);

        act(() => {
            root.render(<DiffViewer diff="multi-file"/>);
        });

        // All sections should be collapsed (aria-expanded=false).
        const buttons = container.querySelectorAll(".diff-file-header");
        expect(buttons.length).toBe(DIFF_COLLAPSE_THRESHOLD + 2);
        buttons.forEach((btn) => {
            expect(btn.getAttribute("aria-expanded")).toBe("false");
        });
    });

    it("DiffViewer expands all files at exactly DIFF_COLLAPSE_THRESHOLD (boundary)", () => {
        const files = Array.from({length: DIFF_COLLAPSE_THRESHOLD}, (_, i) => ({
            header: `diff --git a/file${i}.ts b/file${i}.ts`,
            hunks: [{
                header: "@@ -1,1 +1,1 @@",
                lines: [{type: "context" as const, content: "x", oldLineNum: 1, newLineNum: 1}],
                startOldLine: 1,
                startNewLine: 1,
            }],
        }));
        mocked.parseDiffFiles.mockReturnValue(files);

        act(() => {
            root.render(<DiffViewer diff="boundary-10"/>);
        });

        // Exactly at threshold: files.length > DIFF_COLLAPSE_THRESHOLD is false, so expanded.
        const buttons = container.querySelectorAll(".diff-file-header");
        expect(buttons.length).toBe(DIFF_COLLAPSE_THRESHOLD);
        buttons.forEach((btn) => {
            expect(btn.getAttribute("aria-expanded")).toBe("true");
        });
    });

    it("DiffViewer collapses files at threshold+1 (boundary)", () => {
        const files = Array.from({length: DIFF_COLLAPSE_THRESHOLD + 1}, (_, i) => ({
            header: `diff --git a/file${i}.ts b/file${i}.ts`,
            hunks: [{
                header: "@@ -1,1 +1,1 @@",
                lines: [{type: "context" as const, content: "x", oldLineNum: 1, newLineNum: 1}],
                startOldLine: 1,
                startNewLine: 1,
            }],
        }));
        mocked.parseDiffFiles.mockReturnValue(files);

        act(() => {
            root.render(<DiffViewer diff="boundary-11"/>);
        });

        // One above threshold: all collapsed.
        const buttons = container.querySelectorAll(".diff-file-header");
        expect(buttons.length).toBe(DIFF_COLLAPSE_THRESHOLD + 1);
        buttons.forEach((btn) => {
            expect(btn.getAttribute("aria-expanded")).toBe("false");
        });
    });

    it("DiffFileSection collapses existing instances when file count crosses threshold (I-6)", () => {
        // Start with 5 files (below threshold) — file0-file4 are expanded.
        // Then add files so count exceeds threshold. The first 5 DiffFileSection
        // instances persist (same key) but initialCollapsed changes from false → true.
        // The useEffect must fire and collapse them.
        const makeFiles = (count: number) => Array.from({length: count}, (_, i) => ({
            header: `diff --git a/file${i}.ts b/file${i}.ts`,
            hunks: [{
                header: "@@ -1,1 +1,1 @@",
                lines: [{type: "context" as const, content: "x", oldLineNum: 1, newLineNum: 1}],
                startOldLine: 1,
                startNewLine: 1,
            }],
        }));

        // Render 5 files — all expanded.
        mocked.parseDiffFiles.mockReturnValue(makeFiles(5));
        act(() => {
            root.render(<DiffViewer diff="five-files"/>);
        });
        let buttons = container.querySelectorAll(".diff-file-header");
        expect(buttons.length).toBe(5);
        buttons.forEach((btn) => {
            expect(btn.getAttribute("aria-expanded")).toBe("true");
        });

        // Re-render with files above threshold — first 5 keys persist, initialCollapsed flips to true.
        // Render-time guard syncs collapsed state for existing instances.
        mocked.parseDiffFiles.mockReturnValue(makeFiles(DIFF_COLLAPSE_THRESHOLD + 2));
        act(() => {
            root.render(<DiffViewer diff="twelve-files"/>);
        });
        buttons = container.querySelectorAll(".diff-file-header");
        expect(buttons.length).toBe(DIFF_COLLAPSE_THRESHOLD + 2);
        buttons.forEach((btn) => {
            expect(btn.getAttribute("aria-expanded")).toBe("false");
        });
    });

    it("collapses manually-toggled section when file count then crosses threshold (I-6-b)", () => {
        const makeFiles = (count: number) => Array.from({length: count}, (_, i) => ({
            header: `diff --git a/file${i}.ts b/file${i}.ts`,
            hunks: [{
                header: "@@ -1,1 +1,1 @@",
                lines: [{type: "context" as const, content: "x", oldLineNum: 1, newLineNum: 1}],
                startOldLine: 1,
                startNewLine: 1,
            }],
        }));

        // 1. Render files above threshold — all collapsed.
        const aboveThreshold = DIFF_COLLAPSE_THRESHOLD + 2;
        mocked.parseDiffFiles.mockReturnValue(makeFiles(aboveThreshold));
        act(() => {
            root.render(<DiffViewer diff="twelve-files"/>);
        });
        let buttons = container.querySelectorAll(".diff-file-header");
        expect(buttons.length).toBe(aboveThreshold);
        buttons.forEach((btn) => {
            expect(btn.getAttribute("aria-expanded")).toBe("false");
        });

        // 2. User manually expands the first section by clicking.
        act(() => {
            buttons[0].dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });
        buttons = container.querySelectorAll(".diff-file-header");
        expect(buttons[0].getAttribute("aria-expanded")).toBe("true");

        // 3. Re-render with more files (still above threshold, initialCollapsed stays true).
        //    Since initialCollapsed hasn't changed (still true), render-time guard does NOT re-fire.
        //    The user's manual toggle is preserved — file0 stays expanded.
        const moreFiles = DIFF_COLLAPSE_THRESHOLD + 5;
        mocked.parseDiffFiles.mockReturnValue(makeFiles(moreFiles));
        act(() => {
            root.render(<DiffViewer diff="fifteen-files"/>);
        });
        buttons = container.querySelectorAll(".diff-file-header");
        expect(buttons.length).toBe(moreFiles);
        // file0 (same key, persisted instance) retains its manual expand state.
        expect(buttons[0].getAttribute("aria-expanded")).toBe("true");
        // New sections mount with initialCollapsed=true → collapsed.
        for (let i = aboveThreshold; i < moreFiles; i++) {
            expect(buttons[i].getAttribute("aria-expanded")).toBe("false");
        }
    });

    it("manual collapse persists when diff changes but file count stays same (T-4)", () => {
        const makeFiles = (count: number, prefix: string) => Array.from({length: count}, (_, i) => ({
            header: `diff --git a/${prefix}${i}.ts b/${prefix}${i}.ts`,
            hunks: [{
                header: "@@ -1,1 +1,1 @@",
                lines: [{type: "context" as const, content: "x", oldLineNum: 1, newLineNum: 1}],
                startOldLine: 1,
                startNewLine: 1,
            }],
        }));

        // Render 5 files (below threshold) — all expanded.
        mocked.parseDiffFiles.mockReturnValue(makeFiles(5, "file"));
        act(() => {
            root.render(<DiffViewer diff="first-diff"/>);
        });
        let buttons = container.querySelectorAll(".diff-file-header");
        expect(buttons.length).toBe(5);
        buttons.forEach((btn) => {
            expect(btn.getAttribute("aria-expanded")).toBe("true");
        });

        // User manually collapses file0.
        act(() => {
            buttons[0].dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });
        buttons = container.querySelectorAll(".diff-file-header");
        expect(buttons[0].getAttribute("aria-expanded")).toBe("false");

        // Re-render with same file count but different diff content.
        // initialCollapsed stays false (5 <= 10), so the render-time guard does NOT
        // re-sync — the user's manual collapse should be preserved.
        mocked.parseDiffFiles.mockReturnValue(makeFiles(5, "file"));
        act(() => {
            root.render(<DiffViewer diff="second-diff"/>);
        });
        buttons = container.querySelectorAll(".diff-file-header");
        expect(buttons.length).toBe(5);
        // file0 retains the user's manual collapse.
        expect(buttons[0].getAttribute("aria-expanded")).toBe("false");
        // Other files remain expanded.
        for (let i = 1; i < 5; i++) {
            expect(buttons[i].getAttribute("aria-expanded")).toBe("true");
        }
    });

    it("DiffViewer re-expands files when count drops below threshold (I-8)", () => {
        const makeFiles = (count: number) => Array.from({length: count}, (_, i) => ({
            header: `diff --git a/file${i}.ts b/file${i}.ts`,
            hunks: [{
                header: "@@ -1,1 +1,1 @@",
                lines: [{type: "context" as const, content: "x", oldLineNum: 1, newLineNum: 1}],
                startOldLine: 1,
                startNewLine: 1,
            }],
        }));

        // Start with files above threshold — all collapsed.
        const above = DIFF_COLLAPSE_THRESHOLD + 2;
        mocked.parseDiffFiles.mockReturnValue(makeFiles(above));
        act(() => {
            root.render(<DiffViewer diff="many-files"/>);
        });
        const collapsedButtons = container.querySelectorAll(".diff-file-header");
        expect(collapsedButtons.length).toBe(above);
        collapsedButtons.forEach((btn) => {
            expect(btn.getAttribute("aria-expanded")).toBe("false");
        });

        // Re-render with 5 files (below threshold) — initialCollapsed flips to false.
        mocked.parseDiffFiles.mockReturnValue(makeFiles(5));
        act(() => {
            root.render(<DiffViewer diff="few-files"/>);
        });
        const expandedButtons = container.querySelectorAll(".diff-file-header");
        expect(expandedButtons.length).toBe(5);
        expandedButtons.forEach((btn) => {
            expect(btn.getAttribute("aria-expanded")).toBe("true");
        });
    });
});
