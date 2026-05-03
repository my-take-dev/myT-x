import {describe, expect, it} from "vitest";
import {
    applyPaneReplayBoundary,
    canWritePaneReplay,
    clearPaneReplayBoundary,
    sanitizeTerminalReplay,
    setPaneReplayBoundaryPrefix,
    TerminalOutputFilter,
} from "../src/utils/terminalOutputFilter";

function sanitizeReplayOutput(chunk: string): string {
    return sanitizeTerminalReplay(chunk).output;
}

describe("TerminalOutputFilter", () => {
    it("strips scrollback purge CSI sequences", () => {
        expect(sanitizeReplayOutput("before\x1b[3Jafter")).toBe("beforeafter");
        expect(sanitizeReplayOutput("before\x1b[?3Jafter")).toBe("beforeafter");
    });

    it("preserves normal erase display and unrelated CSI sequences", () => {
        expect(sanitizeReplayOutput("before\x1b[2Jafter")).toBe("before\x1b[2Jafter");
        expect(sanitizeReplayOutput("before\x1b[31mred\x1b[0mafter")).toBe(
            "before\x1b[31mred\x1b[0mafter",
        );
        expect(sanitizeReplayOutput("before\x1bcafter")).toBe("before\x1bcafter");
    });

    it("strips scrollback purge sequences split across chunks", () => {
        const filter = new TerminalOutputFilter();

        expect(filter.sanitize("before\x1b[")).toBe("before");
        expect(filter.sanitize("3Jafter")).toBe("after");
    });

    it("strips private scrollback purge sequences split across chunks", () => {
        const filter = new TerminalOutputFilter();

        expect(filter.sanitize("before\x1b[?")).toBe("before");
        expect(filter.sanitize("3Jafter")).toBe("after");
    });

    it("preserves split non-purge CSI sequences", () => {
        const filter = new TerminalOutputFilter();

        expect(filter.sanitize("before\x1b[")).toBe("before");
        expect(filter.sanitize("2Jafter")).toBe("\x1b[2Jafter");
    });

    it("keeps incomplete replay suffixes as boundary state", () => {
        expect(sanitizeTerminalReplay("before\x1b[")).toEqual({
            output: "before",
            pendingPrefix: "\x1b[",
        });
        expect(sanitizeTerminalReplay("before\x1b[?3")).toEqual({
            output: "before",
            pendingPrefix: "\x1b[?3",
        });
    });

    it("resets pending streaming suffixes without emitting them", () => {
        const filter = new TerminalOutputFilter();

        expect(filter.sanitize("before\x1b[")).toBe("before");
        filter.reset();

        expect(filter.sanitize("3Jafter")).toBe("3Jafter");
    });

    it("strips purge sequences split across replay and first live chunk", () => {
        clearPaneReplayBoundary("pane-boundary");
        expect(setPaneReplayBoundaryPrefix("pane-boundary", "\x1b[?3")).toBe(true);

        expect(applyPaneReplayBoundary("pane-boundary", "Jafter")).toBe("after");
        expect(applyPaneReplayBoundary("pane-boundary", "\x1b[3Jlive")).toBe("\x1b[3Jlive");
    });

    it("preserves later live purge sequences after consuming a replay boundary", () => {
        clearPaneReplayBoundary("pane-multiple-purge");
        expect(setPaneReplayBoundaryPrefix("pane-multiple-purge", "\x1b[3")).toBe(true);

        expect(applyPaneReplayBoundary("pane-multiple-purge", "Jafter\x1b[?3Jmore")).toBe(
            "after\x1b[?3Jmore",
        );
    });

    it("preserves non-purge sequences split across replay and first live chunk", () => {
        clearPaneReplayBoundary("pane-non-purge");
        expect(setPaneReplayBoundaryPrefix("pane-non-purge", "\x1b[")).toBe(true);

        expect(applyPaneReplayBoundary("pane-non-purge", "2Jafter")).toBe("\x1b[2Jafter");
    });

    it("does not arm replay boundary state after live output already started", () => {
        clearPaneReplayBoundary("pane-live-first");
        expect(applyPaneReplayBoundary("pane-live-first", "already-live")).toBe("already-live");

        expect(canWritePaneReplay("pane-live-first")).toBe(false);
        expect(setPaneReplayBoundaryPrefix("pane-live-first", "\x1b[")).toBe(false);
        expect(applyPaneReplayBoundary("pane-live-first", "3Jafter")).toBe("3Jafter");
    });

    it("treats empty live chunks as no-op boundary checks", () => {
        clearPaneReplayBoundary("pane-empty-live");

        expect(applyPaneReplayBoundary("pane-empty-live", "")).toBe("");
        expect(canWritePaneReplay("pane-empty-live")).toBe(true);
        expect(setPaneReplayBoundaryPrefix("pane-empty-live", "\x1b[")).toBe(true);
    });
});
