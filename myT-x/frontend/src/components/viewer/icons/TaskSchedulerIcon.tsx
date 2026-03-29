interface TaskSchedulerIconProps {
    size?: number;
}

export function TaskSchedulerIcon({size = 20}: TaskSchedulerIconProps) {
    return (
        <svg
            width={size}
            height={size}
            viewBox="0 0 20 20"
            fill="none"
            xmlns="http://www.w3.org/2000/svg"
        >
            {/* Checklist lines */}
            <rect x="3" y="4" width="2" height="2" rx="0.5" stroke="currentColor" strokeWidth="1.2"/>
            <line x1="7" y1="5" x2="17" y2="5" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round"/>
            <rect x="3" y="9" width="2" height="2" rx="0.5" stroke="currentColor" strokeWidth="1.2"/>
            <line x1="7" y1="10" x2="17" y2="10" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round"/>
            <rect x="3" y="14" width="2" height="2" rx="0.5" stroke="currentColor" strokeWidth="1.2"/>
            <line x1="7" y1="15" x2="14" y2="15" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round"/>
            {/* Small play arrow */}
            <path
                d="M15.5 13.5l2.5 1.5-2.5 1.5z"
                fill="currentColor"
            />
        </svg>
    );
}
