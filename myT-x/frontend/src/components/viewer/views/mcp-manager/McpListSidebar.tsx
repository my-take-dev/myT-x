import type {MCPStatus, MCPSnapshot} from "../../../../types/mcp";

interface McpListSidebarProps {
    items: MCPSnapshot[];
    selectedId: string | null;
    onSelect: (mcpId: string | null) => void;
    onToggle: (mcpId: string, enabled: boolean) => void;
    togglingIds?: ReadonlySet<string>;
}

function statusDotClass(status: MCPStatus): string {
    switch (status) {
        case "running":
            return "mcp-status-dot running";
        case "starting":
            return "mcp-status-dot starting";
        case "error":
            return "mcp-status-dot error";
        case "stopped":
            return "mcp-status-dot stopped";
        default: {
            const _: never = status;
            return "mcp-status-dot stopped";
        }
    }
}

export function McpListSidebar({items, selectedId, onSelect, onToggle, togglingIds}: McpListSidebarProps) {
    if (items.length === 0) {
        return (
            <div className="mcp-list-sidebar">
                <div className="mcp-list-empty">No MCPs available</div>
            </div>
        );
    }

    return (
        <div className="mcp-list-sidebar" role="list" aria-label="MCP servers">
            {items.map((item) => {
                const isSelected = item.id === selectedId;
                const rowClass = `mcp-list-item${isSelected ? " selected" : ""}`;
                const toggleClass = `mcp-toggle${item.enabled ? " enabled" : ""}`;
                const itemToggleDisabled = togglingIds?.has(item.id) === true;

                return (
                    <div
                        key={item.id}
                        className={rowClass}
                        onClick={() => onSelect(isSelected ? null : item.id)}
                        role="listitem"
                        aria-current={isSelected ? "true" : undefined}
                        tabIndex={0}
                        onKeyDown={(e) => {
                            if (e.key === "Enter" || e.key === " ") {
                                e.preventDefault();
                                onSelect(isSelected ? null : item.id);
                            }
                        }}
                    >
                        <span className={statusDotClass(item.status)} title={item.status}/>
                        <span className="mcp-list-name" title={item.name}>
                            {item.name}
                        </span>
                        <button
                            type="button"
                            className={toggleClass}
                            onClick={(e) => {
                                // Prevent row selection when clicking toggle (#100).
                                e.stopPropagation();
                                onToggle(item.id, !item.enabled);
                            }}
                            disabled={itemToggleDisabled}
                            title={item.enabled ? "Disable" : "Enable"}
                            aria-label={`${item.enabled ? "Disable" : "Enable"} ${item.name}`}
                            aria-pressed={item.enabled}
                        />
                    </div>
                );
            })}
        </div>
    );
}
