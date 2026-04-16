interface PromptPresetsIconProps {
    size?: number;
}

export function PromptPresetsIcon({size = 18}: PromptPresetsIconProps) {
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
            <rect x="3" y="2.5" width="12" height="13" rx="2" stroke="currentColor" strokeWidth="1.3"/>
            <line x1="6" y1="6" x2="12" y2="6" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round"/>
            <line x1="6" y1="9" x2="12" y2="9" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round"/>
            <line x1="6" y1="12" x2="10" y2="12" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round"/>
        </svg>
    );
}
