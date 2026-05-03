interface SessionMemoIconProps {
    size?: number;
}

export function SessionMemoIcon({size = 20}: SessionMemoIconProps) {
    return (
        <svg
            width={size}
            height={size}
            viewBox="0 0 20 20"
            fill="none"
            xmlns="http://www.w3.org/2000/svg"
        >
            <path
                d="M5 3.5h7.5L16 7v9.5H5z"
                stroke="currentColor"
                strokeWidth="1.4"
                strokeLinejoin="round"
            />
            <path
                d="M12.5 3.5V7H16"
                stroke="currentColor"
                strokeWidth="1.4"
                strokeLinejoin="round"
            />
            <path
                d="M7.5 9h5M7.5 11.5h5M7.5 14h3"
                stroke="currentColor"
                strokeWidth="1.3"
                strokeLinecap="round"
            />
        </svg>
    );
}
