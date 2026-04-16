interface SingleTaskRunnerIconProps {
    size?: number;
}

export function SingleTaskRunnerIcon({size = 18}: SingleTaskRunnerIconProps) {
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
            {/* "s" curve fused with three horizontal lines */}
            {/* S-curve on the left */}
            <path
                d="M3.5 4.5C3.5 3.5 4.5 2.8 6 2.8C7.5 2.8 8.2 3.5 8.2 4.3C8.2 5.8 3.5 5.5 3.5 7.8C3.5 8.2 3.5 8.5 3.8 8.8"
                stroke="currentColor"
                strokeWidth="1.2"
                strokeLinecap="round"
                fill="none"
            />
            <path
                d="M3.8 8.8C4.2 9.5 5 9.8 6 9.8C7.2 9.8 8 9.8 8 9.8"
                stroke="currentColor"
                strokeWidth="1.2"
                strokeLinecap="round"
                fill="none"
            />
            <path
                d="M8 9.8C8 9.8 8.5 10.5 8.5 11.2C8.5 12.5 7 13.2 5.5 13.2C4 13.2 3 12.5 3 11.8"
                stroke="currentColor"
                strokeWidth="1.2"
                strokeLinecap="round"
                fill="none"
            />
            {/* Three horizontal lines extending right from S-curve */}
            <line x1="9" y1="4" x2="15.5" y2="4" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round"/>
            <line x1="9" y1="8.5" x2="15.5" y2="8.5" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round"/>
            <line x1="9" y1="13" x2="15.5" y2="13" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round"/>
        </svg>
    );
}
