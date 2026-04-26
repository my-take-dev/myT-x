import {useCallback, useEffect, useRef} from "react";
import {api} from "../api";
import {useTmuxStore} from "../stores/tmuxStore";
import {
    asRecoverableTextEntryTarget,
    consumeTerminalFocusImeRecoverySuppression,
    dispatchTerminalImeRecovery,
    getTerminalTextEntryPaneId,
    IME_RECOVERY_AUTO_COOLDOWN_MS,
    isTerminalTextEntryElement,
    resolveImeRecoveryTarget,
    runImeRecoverySurfaceCycle,
    type TerminalImeRecoveryReason,
} from "../utils/imeRecovery";

interface UseAppImeRecoveryOptions {
    activePaneId: string | null;
}

interface RecoveryOptions {
    callBackend: boolean;
    allowTerminalFallback: boolean;
    cooldownKey?: string;
    paneId?: string | null;
}

const GLOBAL_AUTO_RECOVERY_COOLDOWN_KEY = "__global__";

export function useAppImeRecovery({activePaneId}: UseAppImeRecoveryOptions) {
    const imeResetSignal = useTmuxStore((state) => state.imeResetSignal);
    const recoverySurfaceRef = useRef<HTMLTextAreaElement | null>(null);
    const lastImeResetSignalRef = useRef(imeResetSignal);
    const lastTextEntryTargetRef = useRef<HTMLElement | null>(null);
    const pendingAutoRecoveryRef = useRef(false);
    const recoveryInProgressRef = useRef(false);
    const cooldownUntilRef = useRef<Map<string, number> | null>(null);
    const composingTextEntryTargetRef = useRef<HTMLElement | null>(null);
    const activePaneIdRef = useRef(activePaneId);
    const disposedRef = useRef(false);
    const recoveryRequestIdRef = useRef(0);
    const performRecoveryRef = useRef<(reason: TerminalImeRecoveryReason, options: RecoveryOptions) => Promise<void>>(
        async () => {
            // Initialized below on each render.
        },
    );

    activePaneIdRef.current = activePaneId;
    const getCooldownUntilMap = useCallback((): Map<string, number> => {
        if (cooldownUntilRef.current === null) {
            cooldownUntilRef.current = new Map<string, number>();
        }
        return cooldownUntilRef.current;
    }, []);

    performRecoveryRef.current = async (reason: TerminalImeRecoveryReason, options: RecoveryOptions) => {
        if (recoveryInProgressRef.current || disposedRef.current) {
            return;
        }
        const cooldownKey = options.cooldownKey ?? GLOBAL_AUTO_RECOVERY_COOLDOWN_KEY;
        const cooldownUntil = getCooldownUntilMap();
        if (!options.callBackend && Date.now() < (cooldownUntil.get(cooldownKey) ?? 0)) {
            return;
        }

        const target = resolveImeRecoveryTarget(document.activeElement, lastTextEntryTargetRef.current);
        if (!options.callBackend && target === null) {
            return;
        }

        recoveryInProgressRef.current = true;
        const recoveryRequestId = ++recoveryRequestIdRef.current;
        const targetPaneId = getTerminalTextEntryPaneId(target);
        const targetIsTerminal = isTerminalTextEntryElement(target);
        const dispatchPaneId = targetPaneId
            ?? options.paneId
            ?? (targetIsTerminal || options.allowTerminalFallback ? activePaneIdRef.current : null);
        try {
            if (options.callBackend) {
                try {
                    await api.RecoverIMEWindowFocus();
                } catch (err: unknown) {
                    console.warn("[IME-RECOVERY] native focus recovery failed", err);
                }
            }

            if (disposedRef.current || recoveryRequestId !== recoveryRequestIdRef.current) {
                return;
            }

            await runImeRecoverySurfaceCycle(recoverySurfaceRef.current, target);

            if (disposedRef.current || recoveryRequestId !== recoveryRequestIdRef.current) {
                return;
            }

            if (dispatchPaneId !== null && (targetIsTerminal || (options.allowTerminalFallback && target === null))) {
                dispatchTerminalImeRecovery({paneId: dispatchPaneId, reason});
            }

            pendingAutoRecoveryRef.current = false;
            if (!options.callBackend) {
                cooldownUntil.set(cooldownKey, Date.now() + IME_RECOVERY_AUTO_COOLDOWN_MS);
            }
        } finally {
            recoveryInProgressRef.current = false;
        }
    };

    useEffect(() => {
        disposedRef.current = false;
        return () => {
            disposedRef.current = true;
        };
    }, [getCooldownUntilMap]);

    useEffect(() => {
        const getCooldownKey = (target: HTMLElement | null): string => {
            return getTerminalTextEntryPaneId(target) ?? GLOBAL_AUTO_RECOVERY_COOLDOWN_KEY;
        };

        /**
         * @param requiresPending - true for re-entry paths that must follow a
         * focusout/window blur signal. false for direct terminal textarea focus,
         * where the focused terminal pane itself is the recovery trigger.
         */
        const canRunAutoRecovery = (requiresPending: boolean, cooldownKey: string): boolean => {
            return (!requiresPending || pendingAutoRecoveryRef.current)
                && !recoveryInProgressRef.current
                && !document.hidden
                && document.hasFocus()
                && Date.now() >= (getCooldownUntilMap().get(cooldownKey) ?? 0);
        };

        const markPendingAutoRecovery = (): void => {
            const target = resolveImeRecoveryTarget(document.activeElement, lastTextEntryTargetRef.current);
            if (target === null) {
                return;
            }
            lastTextEntryTargetRef.current = target;
            pendingAutoRecoveryRef.current = true;
        };

        const onFocusIn = (event: FocusEvent): void => {
            const target = asRecoverableTextEntryTarget(event.target);
            if (target === null) {
                return;
            }
            // Keep the latest text entry target before branching so skipped
            // terminal-focus attempts can still be used by later re-entry paths.
            lastTextEntryTargetRef.current = target;
            const terminalPaneId = getTerminalTextEntryPaneId(target);
            if (terminalPaneId !== null) {
                if (
                    composingTextEntryTargetRef.current === target
                    || consumeTerminalFocusImeRecoverySuppression(terminalPaneId)
                ) {
                    return;
                }
                if (!canRunAutoRecovery(false, terminalPaneId)) {
                    return;
                }
                void performRecoveryRef.current("terminal-focus", {
                    callBackend: false,
                    allowTerminalFallback: false,
                    cooldownKey: terminalPaneId,
                    paneId: terminalPaneId,
                });
                return;
            }
            const cooldownKey = getCooldownKey(target);
            if (!canRunAutoRecovery(true, cooldownKey)) {
                return;
            }
            void performRecoveryRef.current("text-entry-reentry", {
                callBackend: false,
                allowTerminalFallback: false,
                cooldownKey,
            });
        };

        const onFocusOut = (event: FocusEvent): void => {
            const target = asRecoverableTextEntryTarget(event.target);
            if (target === null) {
                return;
            }
            const nextTarget = asRecoverableTextEntryTarget(event.relatedTarget);
            if (nextTarget !== null) {
                lastTextEntryTargetRef.current = nextTarget;
                return;
            }
            lastTextEntryTargetRef.current = target;
            pendingAutoRecoveryRef.current = true;
        };

        const onWindowBlur = (): void => {
            markPendingAutoRecovery();
        };

        const onWindowFocus = (): void => {
            const cooldownKey = getCooldownKey(resolveImeRecoveryTarget(document.activeElement, lastTextEntryTargetRef.current));
            if (!canRunAutoRecovery(true, cooldownKey)) {
                return;
            }
            void performRecoveryRef.current("window-focus", {callBackend: false, allowTerminalFallback: false, cooldownKey});
        };

        const onVisibilityChange = (): void => {
            const cooldownKey = getCooldownKey(resolveImeRecoveryTarget(document.activeElement, lastTextEntryTargetRef.current));
            if (document.hidden || !canRunAutoRecovery(true, cooldownKey)) {
                return;
            }
            void performRecoveryRef.current("visibility-change", {
                callBackend: false,
                allowTerminalFallback: false,
                cooldownKey,
            });
        };

        const onCompositionStart = (event: Event): void => {
            composingTextEntryTargetRef.current = asRecoverableTextEntryTarget(event.target);
        };

        const onCompositionEnd = (event: Event): void => {
            if (composingTextEntryTargetRef.current === event.target) {
                composingTextEntryTargetRef.current = null;
            }
        };

        document.addEventListener("focusin", onFocusIn);
        document.addEventListener("focusout", onFocusOut);
        document.addEventListener("compositionstart", onCompositionStart);
        document.addEventListener("compositionend", onCompositionEnd);
        document.addEventListener("compositioncancel", onCompositionEnd);
        window.addEventListener("blur", onWindowBlur);
        window.addEventListener("focus", onWindowFocus);
        document.addEventListener("visibilitychange", onVisibilityChange);

        return () => {
            document.removeEventListener("focusin", onFocusIn);
            document.removeEventListener("focusout", onFocusOut);
            document.removeEventListener("compositionstart", onCompositionStart);
            document.removeEventListener("compositionend", onCompositionEnd);
            document.removeEventListener("compositioncancel", onCompositionEnd);
            window.removeEventListener("blur", onWindowBlur);
            window.removeEventListener("focus", onWindowFocus);
            document.removeEventListener("visibilitychange", onVisibilityChange);
        };
    }, [getCooldownUntilMap]);

    useEffect(() => {
        if (imeResetSignal === lastImeResetSignalRef.current) {
            return;
        }
        lastImeResetSignalRef.current = imeResetSignal;
        void performRecoveryRef.current("manual", {callBackend: true, allowTerminalFallback: true});
    }, [imeResetSignal]);

    return recoverySurfaceRef;
}
