import {useEffect, useRef} from "react";
import {api} from "../api";
import {useTmuxStore} from "../stores/tmuxStore";
import {
    asRecoverableTextEntryTarget,
    dispatchTerminalImeRecovery,
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
}

export function useAppImeRecovery({activePaneId}: UseAppImeRecoveryOptions) {
    const imeResetSignal = useTmuxStore((state) => state.imeResetSignal);
    const recoverySurfaceRef = useRef<HTMLTextAreaElement | null>(null);
    const lastImeResetSignalRef = useRef(imeResetSignal);
    const lastTextEntryTargetRef = useRef<HTMLElement | null>(null);
    const pendingAutoRecoveryRef = useRef(false);
    const recoveryInProgressRef = useRef(false);
    const cooldownUntilRef = useRef(0);
    const activePaneIdRef = useRef(activePaneId);
    const disposedRef = useRef(false);
    const performRecoveryRef = useRef<(reason: TerminalImeRecoveryReason, options: RecoveryOptions) => Promise<void>>(
        async () => {
            // Initialized below on each render.
        },
    );

    activePaneIdRef.current = activePaneId;

    performRecoveryRef.current = async (reason: TerminalImeRecoveryReason, options: RecoveryOptions) => {
        if (recoveryInProgressRef.current || disposedRef.current) {
            return;
        }
        if (!options.callBackend && Date.now() < cooldownUntilRef.current) {
            return;
        }

        const target = resolveImeRecoveryTarget(document.activeElement, lastTextEntryTargetRef.current);
        if (!options.callBackend && target === null) {
            return;
        }

        recoveryInProgressRef.current = true;
        try {
            if (options.callBackend) {
                try {
                    await api.RecoverIMEWindowFocus();
                } catch (err) {
                    console.warn("[IME-RECOVERY] native focus recovery failed", err);
                }
            }

            if (disposedRef.current) {
                return;
            }

            await runImeRecoverySurfaceCycle(recoverySurfaceRef.current, target);

            if (disposedRef.current) {
                return;
            }

            const paneId = activePaneIdRef.current;
            if (paneId !== null && (isTerminalTextEntryElement(target) || (options.allowTerminalFallback && target === null))) {
                dispatchTerminalImeRecovery({paneId, reason});
            }

            pendingAutoRecoveryRef.current = false;
            if (!options.callBackend) {
                cooldownUntilRef.current = Date.now() + IME_RECOVERY_AUTO_COOLDOWN_MS;
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
    }, []);

    useEffect(() => {
        const canRunAutoRecovery = (): boolean => {
            return pendingAutoRecoveryRef.current
                && !recoveryInProgressRef.current
                && !document.hidden
                && document.hasFocus()
                && Date.now() >= cooldownUntilRef.current;
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
            lastTextEntryTargetRef.current = target;
            if (!canRunAutoRecovery()) {
                return;
            }
            void performRecoveryRef.current("text-entry-reentry", {callBackend: false, allowTerminalFallback: false});
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
            if (!canRunAutoRecovery()) {
                return;
            }
            void performRecoveryRef.current("window-focus", {callBackend: false, allowTerminalFallback: false});
        };

        const onVisibilityChange = (): void => {
            if (document.hidden || !canRunAutoRecovery()) {
                return;
            }
            void performRecoveryRef.current("visibility-change", {callBackend: false, allowTerminalFallback: false});
        };

        document.addEventListener("focusin", onFocusIn);
        document.addEventListener("focusout", onFocusOut);
        window.addEventListener("blur", onWindowBlur);
        window.addEventListener("focus", onWindowFocus);
        document.addEventListener("visibilitychange", onVisibilityChange);

        return () => {
            document.removeEventListener("focusin", onFocusIn);
            document.removeEventListener("focusout", onFocusOut);
            window.removeEventListener("blur", onWindowBlur);
            window.removeEventListener("focus", onWindowFocus);
            document.removeEventListener("visibilitychange", onVisibilityChange);
        };
    }, []);

    useEffect(() => {
        if (imeResetSignal === lastImeResetSignalRef.current) {
            return;
        }
        lastImeResetSignalRef.current = imeResetSignal;
        void performRecoveryRef.current("manual", {callBackend: true, allowTerminalFallback: true});
    }, [imeResetSignal]);

    return recoverySurfaceRef;
}
