interface SchedulerIconProps {
    size?: number;
}

export function SchedulerIcon({size = 20}: SchedulerIconProps) {
    return (
        <svg
            width={size}
            height={size}
            viewBox="0 0 20 20"
            fill="none"
            xmlns="http://www.w3.org/2000/svg"
        >
            {/* Clock circle */}
            <circle cx="10" cy="10" r="7" stroke="currentColor" strokeWidth="1.5"/>
            {/* Clock hands */}
            <path
                d="M10 6v4l2.5 2.5"
                stroke="currentColor"
                strokeWidth="1.5"
                strokeLinecap="round"
                strokeLinejoin="round"
            />
            {/* Small send arrow */}
            <path
                d="M15 3l2 2-2 2"
                stroke="currentColor"
                strokeWidth="1.2"
                strokeLinecap="round"
                strokeLinejoin="round"
            />
        </svg>
    );
}
