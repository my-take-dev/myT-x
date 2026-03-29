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
        expect(normalizeMCPStatus(undefined)).toBe("stopped");
        expect(normalizeMCPStatus(1)).toBe("stopped");
        expect(normalizeMCPStatus(true)).toBe("stopped");
        expect(normalizeMCPStatus("")).toBe("stopped");
    });
});

describe("normalizeMCPSnapshot", () => {
    it("returns null when required fields are missing or invalid", () => {
        expect(normalizeMCPSnapshot(null)).toBeNull();
        expect(normalizeMCPSnapshot(undefined)).toBeNull();
        expect(normalizeMCPSnapshot({})).toBeNull();
        expect(normalizeMCPSnapshot({id: "a"})).toBeNull();
        expect(normalizeMCPSnapshot({name: "server"})).toBeNull();
        expect(normalizeMCPSnapshot("string")).toBeNull();
        expect(normalizeMCPSnapshot([])).toBeNull();
        expect(normalizeMCPSnapshot(42)).toBeNull();
    });

    it("rejects whitespace-only or non-string id/name", () => {
        expect(normalizeMCPSnapshot({id: "   ", name: "valid", description: "", enabled: true, status: "running"})).toBeNull();
        expect(normalizeMCPSnapshot({id: 123, name: "valid", description: "", enabled: true, status: "running"})).toBeNull();
        expect(normalizeMCPSnapshot({id: "valid", name: 123, description: "", enabled: true, status: "running"})).toBeNull();
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

    it("normalizes bridge recommendation fields", () => {
        const snapshot = normalizeMCPSnapshot({
            id: "bridge-id",
            name: "Bridge MCP",
            description: "desc",
            enabled: true,
            status: "running",
            bridge_command: "C:\\Program Files\\myT-x\\myT-x.exe",
            bridge_args: ["mcp", "", 1, "stdio", "  ", "--session", "session-a"],
        });

        expect(snapshot).toEqual({
            id: "bridge-id",
            name: "Bridge MCP",
            description: "desc",
            enabled: true,
            status: "running",
            bridge_command: "C:\\Program Files\\myT-x\\myT-x.exe",
            bridge_args: ["mcp", "stdio", "--session", "session-a"],
        });
    });

    it("defaults enabled to false for non-boolean true values", () => {
        const base = {id: "1", name: "s", description: "", status: "running"};
        expect(normalizeMCPSnapshot({...base, enabled: false})!.enabled).toBe(false);
        expect(normalizeMCPSnapshot({...base, enabled: "true"})!.enabled).toBe(false);
        expect(normalizeMCPSnapshot({...base, enabled: 1})!.enabled).toBe(false);
    });

    it("omits error when empty or whitespace-only", () => {
        const base = {id: "1", name: "s", description: "", enabled: true, status: "running"};
        expect(normalizeMCPSnapshot({...base, error: ""})!.error).toBeUndefined();
        expect(normalizeMCPSnapshot({...base, error: "   "})!.error).toBeUndefined();
    });

    it("omits usage_sample when empty", () => {
        const base = {id: "1", name: "s", description: "", enabled: true, status: "running"};
        expect(normalizeMCPSnapshot({...base, usage_sample: ""})!.usage_sample).toBeUndefined();
    });

    it("includes pipe_path and kind when non-empty", () => {
        const base = {id: "1", name: "s", description: "", enabled: true, status: "running"};
        const result = normalizeMCPSnapshot({...base, pipe_path: "\\\\.\\pipe\\mcp-1", kind: "orchestrator"});
        expect(result!.pipe_path).toBe("\\\\.\\pipe\\mcp-1");
        expect(result!.kind).toBe("orchestrator");
    });

    it("omits config_params when all entries are invalid", () => {
        const base = {id: "1", name: "s", description: "", enabled: true, status: "running"};
        expect(normalizeMCPSnapshot({...base, config_params: [{key: "", label: ""}]})!.config_params).toBeUndefined();
    });

    it("omits empty bridge recommendation fields", () => {
        const snapshot = normalizeMCPSnapshot({
            id: "bridge-id",
            name: "Bridge MCP",
            description: "",
            enabled: false,
            status: "stopped",
            bridge_command: "   ",
            bridge_args: ["", "   "],
        });

        expect(snapshot).toEqual({
            id: "bridge-id",
            name: "Bridge MCP",
            description: "",
            enabled: false,
            status: "stopped",
        });
    });
});

describe("normalizeMCPSnapshots", () => {
    it("returns empty array for non-array inputs", () => {
        expect(normalizeMCPSnapshots(null)).toEqual([]);
        expect(normalizeMCPSnapshots(undefined)).toEqual([]);
        expect(normalizeMCPSnapshots({})).toEqual([]);
        expect(normalizeMCPSnapshots("hello")).toEqual([]);
        expect(normalizeMCPSnapshots(42)).toEqual([]);
    });

    it("returns empty array for empty array", () => {
        expect(normalizeMCPSnapshots([])).toEqual([]);
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
