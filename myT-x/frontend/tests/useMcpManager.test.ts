import {describe, expect, it} from "vitest";
import {
    aggregateLspMcpStatus,
    isLspMcp,
    normalizeActiveSessionName,
    selectRepresentativeLspMcp,
} from "../src/components/viewer/views/mcp-manager/useMcpManager";
import type {MCPSnapshot, MCPStatus} from "../src/types/mcp";

function makeSnapshot(status: MCPStatus, id = "lsp-gopls"): MCPSnapshot {
    return {
        id,
        name: id,
        description: "",
        enabled: false,
        status,
    };
}

describe("useMcpManager helpers", () => {
    describe("aggregateLspMcpStatus", () => {
        it("prefers running over every other status", () => {
            const snapshots = [
                makeSnapshot("starting"),
                makeSnapshot("error", "lsp-rust-analyzer"),
                makeSnapshot("running", "lsp-pyright"),
            ];

            expect(aggregateLspMcpStatus(snapshots)).toBe("running");
        });

        it("prefers error when nothing is running", () => {
            const snapshots = [
                makeSnapshot("stopped"),
                makeSnapshot("starting", "lsp-rust-analyzer"),
                makeSnapshot("error", "lsp-pyright"),
            ];

            expect(aggregateLspMcpStatus(snapshots)).toBe("error");
        });

        it("prefers starting when all remaining snapshots are non-running and non-error", () => {
            const snapshots = [
                makeSnapshot("stopped"),
                makeSnapshot("starting", "lsp-rust-analyzer"),
            ];

            expect(aggregateLspMcpStatus(snapshots)).toBe("starting");
        });

        it("falls back to stopped when every snapshot is stopped", () => {
            const snapshots = [
                makeSnapshot("stopped"),
                makeSnapshot("stopped", "lsp-rust-analyzer"),
            ];

            expect(aggregateLspMcpStatus(snapshots)).toBe("stopped");
        });
    });

    describe("isLspMcp", () => {
        it("matches case-insensitive lsp ids and rejects non-lsp ids", () => {
            expect(isLspMcp(makeSnapshot("running", "lsp-gopls"))).toBe(true);
            expect(isLspMcp(makeSnapshot("running", "LSP-Pyright"))).toBe(true);
            expect(isLspMcp(makeSnapshot("running", "memory"))).toBe(false);
        });
    });

    describe("normalizeActiveSessionName", () => {
        it("returns null for null and blank names", () => {
            expect(normalizeActiveSessionName(null)).toBeNull();
            expect(normalizeActiveSessionName("")).toBeNull();
            expect(normalizeActiveSessionName("   ")).toBeNull();
        });

        it("trims surrounding whitespace for non-empty names", () => {
            expect(normalizeActiveSessionName("  session-a  ")).toBe("session-a");
        });
    });

    describe("selectRepresentativeLspMcp", () => {
        it("prefers the first snapshot that already has bridge metadata", () => {
            const withoutBridge = makeSnapshot("running", "lsp-gopls");
            const withBridge = {...makeSnapshot("running", "lsp-pyright"), bridge_command: "myT-x.exe"};

            expect(selectRepresentativeLspMcp([withoutBridge, withBridge])).toBe(withBridge);
        });

        it("falls back to the first LSP snapshot when bridge metadata is missing everywhere", () => {
            const first = makeSnapshot("running", "lsp-gopls");
            const second = makeSnapshot("running", "lsp-pyright");

            expect(selectRepresentativeLspMcp([first, second])).toBe(first);
        });

        it("returns null for an empty LSP snapshot list", () => {
            expect(selectRepresentativeLspMcp([])).toBeNull();
        });
    });
});
