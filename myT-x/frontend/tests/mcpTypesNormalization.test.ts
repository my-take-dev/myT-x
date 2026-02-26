import {describe, expect, it} from "vitest";
import {normalizeMCPStatus, normalizeMCPSnapshot, normalizeMCPSnapshots} from "../src/types/mcp";

describe("normalizeMCPStatus", () => {
    it("accepts known status values", () => {
        expect(normalizeMCPStatus("stopped")).toBe("stopped");
        expect(normalizeMCPStatus("starting")).toBe("starting");
        expect(normalizeMCPStatus("running")).toBe("running");
        expect(normalizeMCPStatus("error")).toBe("error");
    });

    it("falls back to stopped for unknown values", () => {
        expect(normalizeMCPStatus("unknown")).toBe("stopped");
        expect(normalizeMCPStatus(null)).toBe("stopped");
        expect(normalizeMCPStatus(1)).toBe("stopped");
    });
});

describe("normalizeMCPSnapshot", () => {
    it("returns null when required fields are missing", () => {
        expect(normalizeMCPSnapshot(null)).toBeNull();
        expect(normalizeMCPSnapshot({})).toBeNull();
        expect(normalizeMCPSnapshot({id: "a"})).toBeNull();
        expect(normalizeMCPSnapshot({name: "server"})).toBeNull();
    });

    it("normalizes payload and filters invalid config params", () => {
        const snapshot = normalizeMCPSnapshot({
            id: "  test-id  ",
            name: "  Test MCP  ",
            description: 123,
            enabled: true,
            status: "running",
            error: " test error ",
            usage_sample: " use this ",
            config_params: [
                {key: "mode", label: "Mode", default_value: "safe", description: "Execution mode"},
                {key: "", label: "invalid", default_value: "x"},
                "broken",
            ],
        });

        expect(snapshot).toEqual({
            id: "test-id",
            name: "Test MCP",
            description: "",
            enabled: true,
            status: "running",
            error: " test error ",
            usage_sample: " use this ",
            config_params: [{key: "mode", label: "Mode", default_value: "safe", description: "Execution mode"}],
        });
    });

    it("coerces unknown status to stopped", () => {
        const snapshot = normalizeMCPSnapshot({
            id: "id",
            name: "name",
            description: "desc",
            enabled: false,
            status: "unexpected",
        });
        expect(snapshot?.status).toBe("stopped");
    });
});

describe("normalizeMCPSnapshots", () => {
    it("returns empty array for non-array inputs", () => {
        expect(normalizeMCPSnapshots(null)).toEqual([]);
        expect(normalizeMCPSnapshots({})).toEqual([]);
    });

    it("keeps only valid entries", () => {
        const snapshots = normalizeMCPSnapshots([
            {id: "ok", name: "valid", description: "", enabled: true, status: "running"},
            {id: "", name: "invalid", description: "", enabled: true, status: "running"},
            "broken",
        ]);
        expect(snapshots).toEqual([
            {id: "ok", name: "valid", description: "", enabled: true, status: "running"},
        ]);
    });
});
