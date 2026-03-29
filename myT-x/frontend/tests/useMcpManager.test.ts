import {describe, expect, it} from "vitest";
import {
    isLspMcp,
    normalizeActiveSessionName,
    selectRepresentativeLspMcp,
} from "../src/components/viewer/views/mcp-manager/useMcpManager";
import type {MCPSnapshot} from "../src/types/mcp";

function makeSnapshot(status: string, id = "lsp-gopls"): MCPSnapshot {
    return {
        id,
        name: id,
        description: "",
        enabled: false,
        status: status as MCPSnapshot["status"],
    };
}

describe("useMcpManager helpers", () => {
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
