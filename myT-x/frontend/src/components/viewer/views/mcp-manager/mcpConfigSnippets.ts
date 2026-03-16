import type {MCPSnapshot} from "../../../../types/mcp";

export type CliExampleID = "claude" | "codex" | "gemini" | "copilot";

export interface CliExample {
    id: CliExampleID;
    title: string;
    configPath: string;
    snippet: string;
}

export interface BridgeLaunchRecommendation {
    command: string;
    args: string[];
    commandPreview: string;
}

export const lspMcpConfigServerName = "mytx-lsp-mcp";
export const orchMcpConfigServerName = "mytx-agent-orchestrator";
export const lspMcpNamePlaceholder = "$LSP_NAME";

export function escapeTomlBasicString(value: string): string {
    const out: string[] = [];
    for (const char of value) {
        switch (char) {
            case "\\":
                out.push("\\\\");
                break;
            case "\"":
                out.push("\\\"");
                break;
            case "\b":
                out.push("\\b");
                break;
            case "\t":
                out.push("\\t");
                break;
            case "\n":
                out.push("\\n");
                break;
            case "\f":
                out.push("\\f");
                break;
            case "\r":
                out.push("\\r");
                break;
            default: {
                const code = char.charCodeAt(0);
                if (code < 0x20 || code === 0x7f) {
                    out.push(`\\u${code.toString(16).padStart(4, "0")}`);
                } else {
                    out.push(char);
                }
                break;
            }
        }
    }
    return out.join("");
}

function normalizeBridgeCommand(command: string | null | undefined): string {
    return command?.trim() ?? "";
}

export function resolveBridgeCommand(mcp: MCPSnapshot | null): string {
    return normalizeBridgeCommand(mcp?.bridge_command);
}

export function buildLspMcpBridgeArgs(
    lspName: string = lspMcpNamePlaceholder,
): string[] {
    return [
        "mcp",
        "stdio",
        "--mcp",
        lspName,
    ];
}

export function escapeCommandPreviewArg(value: string): string {
    return `"${value.replaceAll("\\", "\\\\").replaceAll("\"", "\\\"")}"`;
}

export function buildCommandPreview(command: string, args: string[]): string {
    // This preview is human-readable guidance, not a shell-specific escaping contract.
    return [command, ...args].map((arg) => escapeCommandPreviewArg(arg)).join(" ");
}

export function buildLspMcpLaunchRecommendation(
    bridgeCommand: string | null | undefined,
    lspName: string = lspMcpNamePlaceholder,
): BridgeLaunchRecommendation | null {
    const command = normalizeBridgeCommand(bridgeCommand);
    if (command === "") {
        return null;
    }
    const args = buildLspMcpBridgeArgs(lspName);
    return {
        command,
        args,
        commandPreview: buildCommandPreview(command, args),
    };
}

function buildTomlStringArray(values: string[]): string {
    if (values.length === 0) {
        return "[]";
    }
    return `[${values.map((value) => `"${escapeTomlBasicString(value)}"`).join(", ")}]`;
}

export function buildCodexConfigSnippet(serverName: string, command: string, args: string[]): string {
    return [
        `[mcp_servers."${escapeTomlBasicString(serverName)}"]`,
        `command = "${escapeTomlBasicString(command)}"`,
        `args = ${buildTomlStringArray(args)}`,
    ].join("\n");
}

export function buildClaudeCodeConfigSnippet(serverName: string, command: string, args: string[]): string {
    return JSON.stringify(
        {
            mcpServers: {
                [serverName]: {
                    type: "stdio",
                    command,
                    args,
                    env: {},
                },
            },
        },
        null,
        2,
    );
}

export function buildGeminiCLIConfigSnippet(serverName: string, command: string, args: string[]): string {
    return JSON.stringify(
        {
            mcpServers: {
                [serverName]: {
                    command,
                    args,
                },
            },
        },
        null,
        2,
    );
}

export function buildCopilotCLIConfigSnippet(serverName: string, command: string, args: string[]): string {
    return JSON.stringify(
        {
            mcpServers: {
                [serverName]: {
                    type: "stdio",
                    command,
                    args,
                    env: {},
                    tools: ["*"],
                },
            },
        },
        null,
        2,
    );
}

export function buildOrchMcpBridgeArgs(): string[] {
    return [
        "mcp",
        "stdio",
        "--mcp",
        "agent-orchestrator",
    ];
}

export function buildOrchMcpLaunchRecommendation(
    bridgeCommand: string | null | undefined,
): BridgeLaunchRecommendation | null {
    const command = normalizeBridgeCommand(bridgeCommand);
    if (command === "") {
        return null;
    }
    const args = buildOrchMcpBridgeArgs();
    return {
        command,
        args,
        commandPreview: buildCommandPreview(command, args),
    };
}

export function buildCliExamples(
    recommendation: BridgeLaunchRecommendation,
    serverName: string = lspMcpConfigServerName,
): CliExample[] {
    const {command, args} = recommendation;
    return [
        {
            id: "claude",
            title: "Claude Code",
            configPath: ".mcp.json (project) or ~/.claude.json (user/local)",
            snippet: buildClaudeCodeConfigSnippet(serverName, command, args),
        },
        {
            id: "codex",
            title: "Codex CLI",
            configPath: "~/.codex/config.toml",
            snippet: buildCodexConfigSnippet(serverName, command, args),
        },
        {
            id: "gemini",
            title: "Gemini CLI",
            configPath: "~/.gemini/settings.json or .gemini/settings.json",
            snippet: buildGeminiCLIConfigSnippet(serverName, command, args),
        },
        {
            id: "copilot",
            title: "Copilot CLI",
            configPath: "~/.copilot/mcp-config.json (default)",
            snippet: buildCopilotCLIConfigSnippet(serverName, command, args),
        },
    ];
}
