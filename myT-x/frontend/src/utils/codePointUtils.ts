/** Count code points without allocating an intermediate array. */
export function codePointLength(value: string): number {
    let count = 0;
    for (const _ of value) {
        count++;
    }
    return count;
}

/**
 * Slice by code point indices using for-of with early break.
 * Negative start values are clamped to 0.
 */
export function sliceByCodePoints(value: string, start: number, end?: number): string {
    const normalizedStart = Math.max(0, start);
    if (end !== undefined && end <= normalizedStart) return "";
    if (normalizedStart === 0 && end === undefined) return value;
    const chars: string[] = [];
    let i = 0;
    for (const ch of value) {
        if (end !== undefined && i >= end) break;
        if (i >= normalizedStart) chars.push(ch);
        i++;
    }
    return chars.join("");
}
