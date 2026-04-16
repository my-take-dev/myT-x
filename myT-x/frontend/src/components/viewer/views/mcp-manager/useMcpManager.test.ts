import {describe, expect, it} from "vitest";
import type {MCPSnapshot} from "../../../../types/mcp";
import {isCustomMcp, isOrchMcp, isStrMcp} from "./useMcpManager";

function buildSnapshot(overrides: Partial<MCPSnapshot> = {}): MCPSnapshot {
    return {
        id: "memory",
        name: "Memory",
        description: "",
        enabled: false,
        status: "stopped",
        ...overrides,
    };
}

describe("useMcpManager classifiers", () => {
    it("treats explicit custom kinds as custom MCPs", () => {
        expect(isCustomMcp(buildSnapshot({kind: "custom"}))).toBe(true);
        expect(isCustomMcp(buildSnapshot({kind: "memory-server"}))).toBe(true);
    });

    it("rejects reserved embedded runtime identities from the custom group", () => {
        expect(isCustomMcp(buildSnapshot({id: "single-task-runner", kind: "custom"}))).toBe(false);
        expect(isStrMcp(buildSnapshot({id: "single-task-runner"}))).toBe(true);
        expect(isStrMcp(buildSnapshot({id: "single-task-runner-helper"}))).toBe(false);
    });

    it("continues to classify orchestrator MCPs by explicit kind", () => {
        expect(isOrchMcp(buildSnapshot({id: "custom-id", kind: "orchestrator"}))).toBe(true);
    });
});
