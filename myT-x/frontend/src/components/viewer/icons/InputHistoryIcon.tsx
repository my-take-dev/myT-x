interface InputHistoryIconProps {
    size?: number;
}

export function InputHistoryIcon({size = 20}: InputHistoryIconProps) {
    return (
        <svg
            width={size}
            height={size}
            viewBox="0 0 20 20"
            fill="none"
            xmlns="http://www.w3.org/2000/svg"
        >
            {/* Terminal prompt symbol */}
            <path
                d="M3 5l4 4-4 4"
                stroke="currentColor"
                strokeWidth="1.5"
                strokeLinecap="round"
                strokeLinejoin="round"
            />
            <line
                x1="9"
                y1="13"
                x2="14"
                y2="13"
                stroke="currentColor"
                strokeWidth="1.5"
                strokeLinecap="round"
            />
            {/* Clock indicator */}
            <circle cx="15.5" cy="5.5" r="3" stroke="currentColor" strokeWidth="1.2"/>
            <path
                d="M15.5 4v1.5l1 1"
                stroke="currentColor"
                strokeWidth="1.2"
                strokeLinecap="round"
                strokeLinejoin="round"
            />
        </svg>
    );
}
