interface OrchestratorTaskIconProps {
    size?: number;
}

export function OrchestratorTaskIcon({size = 18}: OrchestratorTaskIconProps) {
    return (
        <svg
            width={size}
            height={size}
            viewBox="0 0 18 18"
            fill="none"
            xmlns="http://www.w3.org/2000/svg"
            aria-hidden="true"
            focusable="false"
        >
            {/* Three short task lines (top-left) */}
            <line x1="2" y1="4" x2="9" y2="4" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round"/>
            <line x1="2" y1="7.5" x2="8" y2="7.5" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round"/>
            <line x1="2" y1="11" x2="7" y2="11" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round"/>
            {/* "o" circle (bottom-right) */}
            <circle cx="12.5" cy="12.5" r="3.2" stroke="currentColor" strokeWidth="1.3"/>
        </svg>
    );
}
