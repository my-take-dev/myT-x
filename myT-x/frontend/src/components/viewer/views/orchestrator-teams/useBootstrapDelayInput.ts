import {useCallback, useEffect, useMemo, useState, type ChangeEvent} from "react";
import {
    bootstrapDelaySecToMs,
    DEFAULT_BOOTSTRAP_DELAY_MS,
    formatBootstrapDelaySec,
    getBootstrapDelayValidationError,
    normalizeBootstrapDelaySecInput,
} from "./orchestratorTeamUtils";

interface UseBootstrapDelayInputOptions {
    readonly initialMs?: number;
    readonly sourceMs?: number;
    readonly enabled?: boolean;
}

export function useBootstrapDelayInput({
    initialMs = DEFAULT_BOOTSTRAP_DELAY_MS,
    sourceMs,
    enabled = true,
}: UseBootstrapDelayInputOptions = {}) {
    const [delaySec, setDelaySec] = useState(() => formatBootstrapDelaySec(sourceMs ?? initialMs));

    useEffect(() => {
        if (sourceMs === undefined) {
            return;
        }
        setDelaySec(formatBootstrapDelaySec(sourceMs));
    }, [sourceMs]);

    const delayMs = useMemo(() => bootstrapDelaySecToMs(delaySec), [delaySec]);
    const validationError = useMemo(
        () => getBootstrapDelayValidationError(delayMs, enabled),
        [delayMs, enabled],
    );

    const handleChange = useCallback((event: ChangeEvent<HTMLInputElement>) => {
        const nextValue = normalizeBootstrapDelaySecInput(event.target.value);
        if (nextValue !== null) {
            setDelaySec(nextValue);
        }
    }, []);

    return {
        delaySec,
        delayMs,
        validationError,
        isValid: validationError === null,
        handleChange,
    } as const;
}
