/**
 * Format compact YYYYMMDDHHmmss timestamps into a human-readable form.
 * Returns the input unchanged when the format is not 14 digits.
 */
export function formatTimestamp(ts: string): string {
    if (ts.length !== 14) return ts;
    return `${ts.slice(0, 4)}-${ts.slice(4, 6)}-${ts.slice(6, 8)} ${ts.slice(8, 10)}:${ts.slice(10, 12)}:${ts.slice(12, 14)}`;
}
