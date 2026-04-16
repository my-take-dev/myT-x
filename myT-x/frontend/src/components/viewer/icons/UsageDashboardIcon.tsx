interface UsageDashboardIconProps {
    size?: number;
}

export function UsageDashboardIcon({size = 20}: UsageDashboardIconProps) {
    return (
        <svg
            width={size}
            height={size}
            viewBox="0 0 20 20"
            fill="none"
            xmlns="http://www.w3.org/2000/svg"
        >
            <rect x="3" y="12" width="3" height="5" stroke="currentColor" strokeWidth="1.5" rx="0.5"/>
            <rect x="8.5" y="8" width="3" height="9" stroke="currentColor" strokeWidth="1.5" rx="0.5"/>
            <rect x="14" y="4" width="3" height="13" stroke="currentColor" strokeWidth="1.5" rx="0.5"/>
        </svg>
    );
}
