import type {MCPSnapshot} from "../../../../types/mcp";

interface McpDetailPanelProps {
    mcp: MCPSnapshot | null;
}

export function McpDetailPanel({mcp}: McpDetailPanelProps) {
    if (!mcp) {
        return <div className="mcp-detail-empty">Select an MCP to view details</div>;
    }

    const hasConfigParams = mcp.config_params != null && mcp.config_params.length > 0;

    return (
        <div className="mcp-detail-panel">
            <div className="mcp-detail-left">
                <h3 className="mcp-detail-title">{mcp.name}</h3>
                <p className="mcp-detail-description">{mcp.description}</p>

                {mcp.error && (
                    <div className="mcp-detail-error">
                        <span className="mcp-detail-error-label">Error:</span> {mcp.error}
                    </div>
                )}

                {mcp.usage_sample && (
                    <div className="mcp-detail-section">
                        <h4 className="mcp-detail-section-title">Usage Sample</h4>
                        <pre className="mcp-detail-usage-pre">
                            <code>{mcp.usage_sample}</code>
                        </pre>
                    </div>
                )}

                {hasConfigParams && (
                    <div className="mcp-detail-section">
                        <h4 className="mcp-detail-section-title">Configuration</h4>
                        {mcp.config_params?.map((p) => (
                            <div key={p.key} className="mcp-config-param">
                                <div className="mcp-config-param-header">
                                    <span className="mcp-config-param-label">{p.label}</span>
                                    <span className="mcp-config-param-value">{p.default_value}</span>
                                </div>
                                {p.description && (
                                    <span className="mcp-config-param-desc">{p.description}</span>
                                )}
                            </div>
                        ))}
                    </div>
                )}
            </div>

            <div className="mcp-detail-right">
                {/* Placeholder for future MCP-specific editors (Memory Explorer, etc.) */}
                <div className="mcp-detail-placeholder">
                    Editor area (future)
                </div>
            </div>
        </div>
    );
}
