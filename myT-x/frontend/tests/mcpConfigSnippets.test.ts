import {describe, expect, it} from "vitest";
import {
    buildCliExamples,
    buildCodexConfigSnippet,
    buildLspMcpBridgeArgs,
    buildLspMcpLaunchRecommendation,
    escapeTomlBasicString,
    lspMcpConfigServerName,
    lspMcpNamePlaceholder,
    resolveBridgeCommand,
} from "../src/components/viewer/views/mcp-manager/mcpConfigSnippets";
import type {MCPSnapshot} from "../src/types/mcp";

function sampleMCP(): MCPSnapshot {
    return {
        id: "lsp-gopls",
        name: "Go LSP",
        description: "",
        enabled: false,
        status: "stopped",
        bridge_command: `C:\\Program Files\\myT-x\\myT-x.exe`,
    };
}

describe("mcpConfigSnippets", () => {
    it("resolves the bridge command from the snapshot and returns an empty string when unavailable", () => {
        expect(resolveBridgeCommand(sampleMCP())).toBe(`C:\\Program Files\\myT-x\\myT-x.exe`);

        const missingCommand = sampleMCP();
        delete missingCommand.bridge_command;
        expect(resolveBridgeCommand(missingCommand)).toBe("");
    });

    it("builds bridge args with the shared LSP placeholder", () => {
        expect(buildLspMcpBridgeArgs()).toEqual([
            "mcp",
            "stdio",
            "--mcp",
            lspMcpNamePlaceholder,
        ]);
    });

    it("builds shared CLI examples for the LSP-MCP category", () => {
        const recommendation = buildLspMcpLaunchRecommendation(resolveBridgeCommand(sampleMCP()));
        expect(recommendation).not.toBeNull();
        const examples = buildCliExamples(recommendation!);
        expect(examples).toHaveLength(4);
        for (const example of examples) {
            expect(example.snippet).toContain(lspMcpConfigServerName);
            expect(example.snippet).toContain(lspMcpNamePlaceholder);
        }
    });

    it("builds a quoted command preview from the bridge recommendation", () => {
        const recommendation = buildLspMcpLaunchRecommendation(`C:\\Tools\\my "quoted".exe`);
        expect(recommendation).not.toBeNull();
        if (recommendation == null) {
            throw new Error("expected a bridge recommendation");
        }
        expect(recommendation.commandPreview).toContain(`\\"quoted\\"`);
        expect(recommendation.commandPreview).toContain(`"${lspMcpNamePlaceholder}"`);
    });

    it("returns no launch recommendation when bridge command metadata is unavailable", () => {
        const recommendation = buildLspMcpLaunchRecommendation("   ");
        expect(recommendation).toBeNull();
    });

    it("escapes control characters in the Codex TOML snippet", () => {
        const snippet = buildCodexConfigSnippet(
            lspMcpConfigServerName,
            "C:\\tmp\\myT-x\ttool\r\n\u0000",
            ["mcp", "stdio", "--mcp", "<go\u0000pls>"],
        );

        expect(snippet).toContain(`command = "C:\\\\tmp\\\\myT-x\\ttool\\r\\n\\u0000"`);
        expect(snippet).toContain(`args = ["mcp", "stdio", "--mcp", "<go\\u0000pls>"]`);
    });

    it("escapes TOML basic string content directly", () => {
        expect(escapeTomlBasicString(String.raw`C:\tmp\"quoted"`)).toBe(String.raw`C:\\tmp\\\"quoted\"`);
        expect(escapeTomlBasicString("line1\r\nline2\t\u0000")).toBe("line1\\r\\nline2\\t\\u0000");
    });
});
