export function McpIcon({size = 18}: { size?: number }) {
    return (
        <svg
            width={size}
            height={size}
            viewBox="0 0 16 16"
            fill="none"
            xmlns="http://www.w3.org/2000/svg"
            aria-hidden="true"
            focusable="false"
        >
            {/* Server stack icon representing MCP protocol */}
            <rect x="3" y="2" width="10" height="4" rx="1" stroke="currentColor" strokeWidth="1.2"/>
            <rect x="3" y="10" width="10" height="4" rx="1" stroke="currentColor" strokeWidth="1.2"/>
            <path d="M8 6V10" stroke="currentColor" strokeWidth="1.2" strokeLinecap="round"/>
            <circle cx="5.5" cy="4" r="0.8" fill="currentColor"/>
            <circle cx="5.5" cy="12" r="0.8" fill="currentColor"/>
        </svg>
    );
}
