import {memo, useEffect, useMemo, useRef} from "react";
import type {MouseEvent as ReactMouseEvent} from "react";
import type {AppConfigAutoStartCommand} from "../types/tmux";
import {useI18n} from "../i18n";

interface AutoStartPopoverProps {
    readonly entries: AppConfigAutoStartCommand[];
    readonly onStart: (entry: AppConfigAutoStartCommand) => void;
    readonly onClose: () => void;
    readonly startDisabled?: boolean;
    readonly preventTerminalFocusSteal: (event: ReactMouseEvent<HTMLElement>) => void;
}

function autoStartPreview(entry: AppConfigAutoStartCommand): string {
    return [entry.command.trim(), (entry.args ?? "").trim()].filter(Boolean).join(" ");
}

export const AutoStartPopover = memo(function AutoStartPopover({
    entries,
    onStart,
    onClose,
    startDisabled = false,
    preventTerminalFocusSteal,
}: AutoStartPopoverProps) {
    const {language, t} = useI18n();
    const popoverRef = useRef<HTMLDivElement>(null);
    const runnableEntries = useMemo(
        () => entries.filter((entry) => entry.command.trim()),
        [entries],
    );

    useEffect(() => {
        const handleMouseDown = (event: MouseEvent) => {
            if (popoverRef.current && !popoverRef.current.contains(event.target as Node)) {
                onClose();
            }
        };
        const handleKeyDown = (event: KeyboardEvent) => {
            if (event.key === "Escape") {
                onClose();
            }
        };
        document.addEventListener("mousedown", handleMouseDown);
        document.addEventListener("keydown", handleKeyDown);
        return () => {
            document.removeEventListener("mousedown", handleMouseDown);
            document.removeEventListener("keydown", handleKeyDown);
        };
    }, [onClose]);

    return (
        <div
            ref={popoverRef}
            className="auto-start-popover"
            onMouseDown={preventTerminalFocusSteal}
        >
            <div className="auto-enter-popover-title">
                {language === "en" ? "AutoStart" : t("autoStart.popover.title", "AutoStart")}
            </div>
            <div className="auto-start-command-list">
                {runnableEntries.map((entry, index) => {
                    const preview = autoStartPreview(entry);
                    const label = entry.name.trim() || preview;
                    return (
                        <button
                            key={`${preview}-${index}`}
                            type="button"
                            className="auto-start-command-btn"
                            title={preview}
                            disabled={startDisabled}
                            aria-busy={startDisabled}
                            onClick={(event) => {
                                event.stopPropagation();
                                if (startDisabled) {
                                    return;
                                }
                                onStart(entry);
                            }}
                        >
                            <span className="auto-start-command-name">{label}</span>
                            <span className="auto-start-command-preview">[{preview}]</span>
                        </button>
                    );
                })}
            </div>
        </div>
    );
});
