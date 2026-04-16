function isRecord(value: unknown): value is Record<string, unknown> {
    return typeof value === "object" && value !== null;
}

export function stoppedMessage(value: unknown): string | null {
    if (typeof value === "string") {
        const reason = value.trim();
        return reason === "" ? null : reason;
    }
    if (!isRecord(value)) {
        return null;
    }
    const reason = typeof value.reason === "string" ? value.reason.trim() : "";
    return reason === "" ? null : reason;
}

export function normalizeGenerationId(value: string | null | undefined): string | null {
    if (typeof value !== "string") {
        return null;
    }
    const normalized = value.trim();
    return normalized === "" ? null : normalized;
}
