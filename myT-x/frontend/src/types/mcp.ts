/**
 * MCP (Model Context Protocol) type definitions.
 *
 * These interfaces mirror the Go mcp.MCPSnapshot and mcp.MCPConfigParam types.
 * Field names use snake_case to match the JSON serialization from the Go backend.
 */

export interface MCPConfigParam {
    key: string;
    label: string;
    default_value: string;
    description?: string;
}

export type MCPStatus = "stopped" | "starting" | "running" | "error";

export interface MCPSnapshot {
    id: string;
    name: string;
    description: string;
    enabled: boolean;
    /** Runtime status: "stopped" | "starting" | "running" | "error". */
    status: MCPStatus;
    error?: string;
    usage_sample?: string;
    config_params?: MCPConfigParam[];
    /** Named Pipe path when running. Empty when the MCP instance is not running. */
    pipe_path?: string;
    /** Bridge executable path or fallback name for stdio clients (for example "C:\\Program Files\\myT-x\\myT-x.exe"). */
    bridge_command?: string;
    /** Bridge arguments for stdio clients. */
    bridge_args?: string[];
}

function isRecord(value: unknown): value is Record<string, unknown> {
    return value != null && typeof value === "object" && !Array.isArray(value);
}

export function normalizeMCPStatus(status: unknown): MCPStatus {
    switch (status) {
        case "running":
        case "starting":
        case "error":
        case "stopped":
            return status;
        default:
            return "stopped";
    }
}

function normalizeConfigParam(param: unknown): MCPConfigParam | null {
    if (!isRecord(param)) {
        return null;
    }

    const key = typeof param.key === "string" ? param.key : "";
    const label = typeof param.label === "string" ? param.label : "";
    if (key === "" || label === "") {
        return null;
    }

    const normalized: MCPConfigParam = {
        key,
        label,
        default_value: typeof param.default_value === "string" ? param.default_value : "",
    };

    if (typeof param.description === "string" && param.description.trim() !== "") {
        normalized.description = param.description;
    }
    return normalized;
}

export function normalizeMCPSnapshot(snapshot: unknown): MCPSnapshot | null {
    if (!isRecord(snapshot)) {
        return null;
    }

    const id = typeof snapshot.id === "string" ? snapshot.id.trim() : "";
    const name = typeof snapshot.name === "string" ? snapshot.name.trim() : "";
    if (id === "" || name === "") {
        return null;
    }

    const configParamsArray = Array.isArray(snapshot.config_params) ? snapshot.config_params : [];
    const configParams = configParamsArray
        .map((param) => normalizeConfigParam(param))
        .filter((param): param is MCPConfigParam => param != null);

    const normalized: MCPSnapshot = {
        id,
        name,
        description: typeof snapshot.description === "string" ? snapshot.description : "",
        enabled: snapshot.enabled === true,
        status: normalizeMCPStatus(snapshot.status),
    };

    if (typeof snapshot.error === "string" && snapshot.error.trim() !== "") {
        normalized.error = snapshot.error;
    }
    if (typeof snapshot.usage_sample === "string" && snapshot.usage_sample.trim() !== "") {
        normalized.usage_sample = snapshot.usage_sample;
    }
    if (configParams.length > 0) {
        normalized.config_params = configParams;
    }
    if (typeof snapshot.pipe_path === "string" && snapshot.pipe_path.trim() !== "") {
        normalized.pipe_path = snapshot.pipe_path;
    }
    if (typeof snapshot.bridge_command === "string" && snapshot.bridge_command.trim() !== "") {
        normalized.bridge_command = snapshot.bridge_command;
    }
    if (Array.isArray(snapshot.bridge_args)) {
        const bridgeArgs = snapshot.bridge_args.filter(
            (arg): arg is string => typeof arg === "string" && arg.trim() !== "",
        );
        if (bridgeArgs.length > 0) {
            normalized.bridge_args = bridgeArgs;
        }
    }

    return normalized;
}

export function normalizeMCPSnapshots(snapshots: unknown): MCPSnapshot[] {
    if (!Array.isArray(snapshots)) {
        return [];
    }
    return snapshots
        .map((snapshot) => normalizeMCPSnapshot(snapshot))
        .filter((snapshot): snapshot is MCPSnapshot => snapshot != null);
}
