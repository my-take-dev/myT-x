import {describe, expect, it} from "vitest";
import {
    isCustomMcp,
    isOrchMcp,
    isStrMcp,
    isLspMcp,
    normalizeActiveSessionName,
    selectRepresentativeMcp,
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
        it("does not treat explicit custom kinds as legacy lsp MCPs", () => {
            expect(isLspMcp({...makeSnapshot("running", "lsp-gopls"), kind: "memory-server"})).toBe(false);
            expect(isLspMcp({...makeSnapshot("running", "lsp-custom"), kind: "custom"})).toBe(false);
        });
    });

    describe("isCustomMcp", () => {
        it("classifies non-built-in snapshots as custom", () => {
            expect(isCustomMcp(makeSnapshot("running", "memory"))).toBe(true);
            expect(isCustomMcp(makeSnapshot("running", "lsp-gopls"))).toBe(false);
            expect(isCustomMcp({
                ...makeSnapshot("running", "orch-agent-orchestrator"),
                kind: "orchestrator"
            })).toBe(false);
            expect(isCustomMcp({
                ...makeSnapshot("running", "orch-agent-orchestrator"),
                kind: "memory-server",
            })).toBe(true);
        });
    });

    describe("isOrchMcp", () => {
        it("matches orchestrator kind or legacy orch id prefix", () => {
            expect(isOrchMcp({...makeSnapshot("running", "agent-orchestrator"), kind: "orchestrator"})).toBe(true);
            expect(isOrchMcp(makeSnapshot("running", "orch-agent-orchestrator"))).toBe(true);
            expect(isOrchMcp({...makeSnapshot("running", "orch-agent-orchestrator"), kind: "memory-server"})).toBe(false);
            expect(isOrchMcp(makeSnapshot("running", "single-task-runner"))).toBe(false);
        });
    });

    describe("isStrMcp", () => {
        it("matches single-task-runner kind or the built-in MCP id", () => {
            expect(isStrMcp({...makeSnapshot("running", "str-1"), kind: "single-task-runner"})).toBe(true);
            expect(isStrMcp(makeSnapshot("running", "single-task-runner"))).toBe(true);
            expect(isStrMcp({...makeSnapshot("running", "single-task-runner"), kind: "memory-server"})).toBe(false);
            expect(isStrMcp(makeSnapshot("running", "single-task-runner-helper"))).toBe(false);
            expect(isStrMcp(makeSnapshot("running", "orch-1"))).toBe(false);
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

    describe("selectRepresentativeMcp", () => {
        it("prefers the first snapshot that already has bridge metadata", () => {
            const withoutBridge = makeSnapshot("running", "lsp-gopls");
            const withBridge = {...makeSnapshot("running", "lsp-pyright"), bridge_command: "myT-x.exe"};

            expect(selectRepresentativeMcp([withoutBridge, withBridge])).toBe(withBridge);
        });

        it("falls back to the first snapshot when bridge metadata is missing everywhere", () => {
            const first = makeSnapshot("running", "lsp-gopls");
            const second = makeSnapshot("running", "lsp-pyright");

            expect(selectRepresentativeMcp([first, second])).toBe(first);
        });

        it("returns null for an empty snapshot list", () => {
            expect(selectRepresentativeMcp([])).toBeNull();
        });
    });
});
