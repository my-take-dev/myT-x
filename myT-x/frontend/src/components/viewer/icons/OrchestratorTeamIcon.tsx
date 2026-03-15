interface OrchestratorTeamIconProps {
    size?: number;
}

export function OrchestratorTeamIcon({size = 18}: OrchestratorTeamIconProps) {
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
            <rect x="2.25" y="3" width="5" height="5" rx="1.25" stroke="currentColor" strokeWidth="1.2"/>
            <rect x="10.75" y="3" width="5" height="5" rx="1.25" stroke="currentColor" strokeWidth="1.2"/>
            <rect x="6.5" y="10" width="5" height="5" rx="1.25" stroke="currentColor" strokeWidth="1.2"/>
            <path d="M7.25 5.5H10.75" stroke="currentColor" strokeWidth="1.2" strokeLinecap="round"/>
            <path d="M9 8V10" stroke="currentColor" strokeWidth="1.2" strokeLinecap="round"/>
        </svg>
    );
}
