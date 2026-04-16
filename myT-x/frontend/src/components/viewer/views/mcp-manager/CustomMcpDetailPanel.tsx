import {useI18n} from "../../../../i18n";
import type {MCPSnapshot} from "../../../../types/mcp";

interface CustomMcpDetailPanelProps {
    representativeMCP: MCPSnapshot | null;
    activeSession: string | null;
    totalCustomCount: number;
}

export function CustomMcpDetailPanel({
                                         representativeMCP,
                                         activeSession,
                                         totalCustomCount,
                                     }: CustomMcpDetailPanelProps) {
    const {language, t} = useI18n();
    const tr = (key: string, jaText: string, enText: string) =>
        t(key, language === "ja" ? jaText : enText);

    const normalizedSession = activeSession?.trim() ?? "";
    if (normalizedSession === "") {
        return (
            <div className="mcp-detail-empty">
                {tr(
                    "viewer.customMcpDetail.selectSession",
                    "アクティブなセッションを選択して、Custom MCPの詳細を確認してください。",
                    "Select an active session to inspect custom MCP details.",
                )}
            </div>
        );
    }
    if (totalCustomCount === 0) {
        return (
            <div className="mcp-detail-empty">
                {tr(
                    "viewer.customMcpDetail.noProfiles",
                    "このセッションで利用可能なCustom MCPプロファイルはありません。",
                    "No custom MCP profiles are available in this session.",
                )}
            </div>
        );
    }

    return (
        <div className="mcp-detail-panel">
            <div className="mcp-detail-left">
                <h3 className="mcp-detail-title">Custom MCP</h3>
                <p className="mcp-detail-description">
                    {tr(
                        "viewer.customMcpDetail.description",
                        "このカテゴリには、LSP-MCP / Agent Orchestrator / Single Task Runner 以外のMCPプロファイルが含まれます。",
                        "This category contains MCP profiles that are not LSP-MCP, Agent Orchestrator, or Single Task Runner.",
                    )}
                </p>
                <div className="mcp-detail-section">
                    <h4 className="mcp-detail-section-title">
                        {tr("viewer.customMcpDetail.section.summary", "概要", "Summary")}
                    </h4>
                    <p className="mcp-detail-description">
                        {tr(
                            "viewer.customMcpDetail.profileCount",
                            "検出されたCustom MCP数",
                            "Detected custom MCP count",
                        )}: <code>{String(totalCustomCount)}</code>
                    </p>
                    {representativeMCP != null && (
                        <>
                            <p className="mcp-detail-description">
                                {tr("viewer.customMcpDetail.profileName", "代表プロファイル名", "Representative profile")}:{" "}
                                <code>{representativeMCP.name}</code>
                            </p>
                            <p className="mcp-detail-description">
                                {tr("viewer.customMcpDetail.profileID", "プロファイルID", "Profile ID")}:{" "}
                                <code>{representativeMCP.id}</code>
                            </p>
                        </>
                    )}
                </div>
                {representativeMCP?.description && (
                    <div className="mcp-detail-section">
                        <h4 className="mcp-detail-section-title">
                            {tr("viewer.customMcpDetail.section.description", "説明", "Description")}
                        </h4>
                        <p className="mcp-detail-description">{representativeMCP.description}</p>
                    </div>
                )}
                {representativeMCP?.usage_sample && (
                    <div className="mcp-detail-section">
                        <h4 className="mcp-detail-section-title">
                            {tr("viewer.customMcpDetail.section.usage", "利用例", "Usage sample")}
                        </h4>
                        <pre className="mcp-detail-usage-pre">
                            <code>{representativeMCP.usage_sample}</code>
                        </pre>
                    </div>
                )}
                {representativeMCP?.bridge_command && (
                    <div className="mcp-detail-section">
                        <h4 className="mcp-detail-section-title">
                            {tr("viewer.customMcpDetail.section.bridge", "ブリッジコマンド", "Bridge command")}
                        </h4>
                        <pre className="mcp-detail-usage-pre">
                            <code>{representativeMCP.bridge_command}</code>
                        </pre>
                    </div>
                )}
                <div className="mcp-detail-section">
                    <h4 className="mcp-detail-section-title">
                        {tr("viewer.customMcpDetail.section.notes", "備考", "Notes")}
                    </h4>
                    <pre className="mcp-detail-usage-pre">
                        <code>
                            {tr(
                                "viewer.customMcpDetail.notesBody",
                                "Custom MCP には LSP 専用の起動ガイダンスは適用されません。必要な接続方法や起動条件は、各プロファイルの説明・usage sample・bridge metadata を確認してください。",
                                "Custom MCPs do not use the built-in LSP launch guidance. Check each profile's description, usage sample, and bridge metadata for the correct connection flow.",
                            )}
                        </code>
                    </pre>
                </div>
            </div>
        </div>
    );
}
