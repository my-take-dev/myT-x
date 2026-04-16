export function formatNumber(value: number): string {
    if (!Number.isFinite(value)) return "0";
    return value.toLocaleString("en-US");
}
