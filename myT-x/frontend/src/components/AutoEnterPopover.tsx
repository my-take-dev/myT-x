import {memo, useCallback, useEffect, useRef, useState} from "react";
import type {MouseEvent as ReactMouseEvent} from "react";
import {useI18n} from "../i18n";

interface AutoEnterPopoverProps {
    readonly onStart: (intervalSeconds: number) => void;
    readonly onClose: () => void;
    readonly preventTerminalFocusSteal: (event: ReactMouseEvent<HTMLElement>) => void;
}

export const AutoEnterPopover = memo(function AutoEnterPopover({
    onStart,
    onClose,
    preventTerminalFocusSteal,
}: AutoEnterPopoverProps) {
    const {language, t} = useI18n();
    const isEn = language === "en";
    const [intervalSeconds, setIntervalSeconds] = useState("30");
    const parsedInterval = intervalSeconds === "" ? 0 : Number(intervalSeconds);
    const canStart = parsedInterval >= 10;
    const popoverRef = useRef<HTMLDivElement>(null);

    // Click-outside to close.
    useEffect(() => {
        const handle = (e: MouseEvent): void => {
            if (popoverRef.current && !popoverRef.current.contains(e.target as Node)) {
                onClose();
            }
        };
        // Defer to avoid immediate close from the triggering click.
        const timer = setTimeout(() => {
            document.addEventListener("mousedown", handle);
        }, 0);
        return () => {
            clearTimeout(timer);
            document.removeEventListener("mousedown", handle);
        };
    }, [onClose]);

    // Escape to close.
    useEffect(() => {
        const handle = (e: KeyboardEvent): void => {
            if (e.key === "Escape") {
                onClose();
            }
        };
        document.addEventListener("keydown", handle);
        return () => document.removeEventListener("keydown", handle);
    }, [onClose]);

    const handleStart = useCallback(() => {
        if (!canStart) return;
        onStart(parsedInterval);
    }, [canStart, parsedInterval, onStart]);

    return (
        <div
            ref={popoverRef}
            className="auto-enter-popover"
            onMouseDown={preventTerminalFocusSteal}
        >
            <div className="auto-enter-popover-title">
                {isEn ? "Auto Confirm" : t("autoEnter.popover.title", "自動確定")}
            </div>
            <div className="auto-enter-popover-desc">
                {isEn
                    ? "Sends Enter key to this pane at the set interval."
                    : t("autoEnter.popover.desc", "設定した間隔でこのペインにEnterキーを送信します。")}
            </div>
            <div className="auto-enter-popover-row">
                <span className="auto-enter-popover-label">
                    {isEn ? "Interval" : t("autoEnter.popover.interval", "間隔")}
                </span>
                <input
                    className="auto-enter-popover-input"
                    type="text"
                    inputMode="numeric"
                    value={intervalSeconds}
                    onChange={(e) => {
                        const v = e.target.value;
                        if (v === "" || /^\d+$/.test(v)) {
                            setIntervalSeconds(v.replace(/^0+(?=\d)/, ""));
                        }
                    }}
                    onKeyDown={(e) => {
                        if (e.key === "Enter") {
                            e.preventDefault();
                            handleStart();
                        }
                        if (e.key === "Escape") {
                            e.preventDefault();
                            onClose();
                        }
                    }}
                    autoFocus
                />
                <span className="auto-enter-popover-unit">
                    {isEn ? "sec" : t("autoEnter.popover.unit", "秒")}
                </span>
            </div>
            <button
                type="button"
                className="auto-enter-popover-start-btn"
                disabled={!canStart}
                onMouseDown={preventTerminalFocusSteal}
                onClick={handleStart}
            >
                {isEn ? "Start" : t("autoEnter.popover.start", "開始")}
            </button>
        </div>
    );
});
