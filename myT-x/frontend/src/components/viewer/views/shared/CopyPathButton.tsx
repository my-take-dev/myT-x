import type {CopyNoticeState} from "../../../../utils/clipboardUtils";

interface CopyPathButtonProps {
    readonly state: CopyNoticeState;
    /**
     * Click handler. Typed as `() => void` so callers cannot pass a handler that
     * requires a MouseEvent parameter. The runtime MouseEvent is dropped internally.
     */
    readonly onClick: () => void;
    readonly className?: string;
}

function copyButtonLabel(state: CopyNoticeState): string {
    switch (state) {
        case "copied":
            return "Copied!";
        case "failed":
            return "Copy failed";
        case "idle":
            return "Copy path";
    }
}

function CopyIcon({state}: { state: CopyNoticeState }) {
    switch (state) {
        case "copied":
            return (
                <svg width="14" height="14" viewBox="0 0 16 16" fill="none">
                    <path
                        d="M3 8.5L6.5 12L13 4"
                        stroke="currentColor"
                        strokeWidth="2"
                        strokeLinecap="round"
                        strokeLinejoin="round"
                    />
                </svg>
            );
        case "failed":
            return (
                <svg width="14" height="14" viewBox="0 0 16 16" fill="none">
                    <path
                        d="M5 5l6 6M11 5l-6 6"
                        stroke="currentColor"
                        strokeWidth="1.8"
                        strokeLinecap="round"
                    />
                </svg>
            );
        case "idle":
            return (
                <svg width="14" height="14" viewBox="0 0 16 16" fill="none">
                    <rect x="5" y="3" width="8" height="10" rx="1" stroke="currentColor" strokeWidth="1.5"/>
                    <path
                        d="M3 5v8a1 1 0 001 1h6"
                        stroke="currentColor"
                        strokeWidth="1.5"
                        strokeLinecap="round"
                        strokeLinejoin="round"
                    />
                </svg>
            );
    }
}

export function CopyPathButton({state, onClick, className = "file-content-copy-path"}: CopyPathButtonProps) {
    const label = copyButtonLabel(state);
    return (
        <button
            type="button"
            className={className}
            // Lambda wrapper drops the runtime MouseEvent that React passes.
            // The prop type `() => void` prevents compile-time leakage; this
            // wrapper prevents runtime leakage.
            onClick={() => onClick()}
            title={label}
            aria-label={label}
        >
            <CopyIcon state={state}/>
        </button>
    );
}
