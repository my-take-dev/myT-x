/**
 * Sanitize a CSS color value to prevent injection. Returns "inherit" for unsafe values.
 *
 * Non-color() functions (rgb, hsl, oklch, etc.) are matched case-sensitively (lowercase only).
 * This is intentional: Shiki's tokenizer always emits lowercase function names, so uppercase
 * variants from untrusted sources are rejected. The color() function uses /i flag for its
 * color-space name per CSS Color Level 4 spec.
 *
 * Accepted formats:
 * - Hex: #rgb, #rgba, #rrggbb, #rrggbbaa
 * - Legacy: rgb(), rgba(), hsl(), hsla()
 * - CSS Color Level 4: oklch(), lch(), lab(), oklab(), hwb()
 * - CSS color() function: color(display-p3 1 0.5 0)
 *
 * Not accepted (falls back to "inherit"): named colors, relative color syntax.
 */
export function sanitizeCssColor(color: string | undefined): string {
    if (!color) return "inherit";
    if (/^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{4}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$/.test(color)) return color;
    // NOTE: Numeric value ranges are NOT validated (e.g., rgb(999,999,999) passes).
    // This function's sole purpose is CSS injection prevention, not color correctness.
    if (/^(?:rgb|rgba|hsl|hsla|oklch|lch|lab|oklab|hwb)\([\d\s,.%/\-]+\)$/.test(color)) return color;
    // color() function: color(<colorspace> <number>+ [/ <alpha>]?)
    // Case-insensitive for color space names (e.g., sRGB, display-p3).
    // NOTE: \s+ allows multiple spaces between values, matching CSS spec behavior.
    // NOT allowed: empty channel lists or comma-separated channel syntax inside color().
    // Shiki output typically uses single spaces, but this handles edge cases gracefully.
    // Supports negative values (e.g., color(xyz-d65 -0.5 0.2 0.1)) to match lch/oklch/lab
    // pattern consistency. CSS color() allows negative channel values for some color spaces.
    if (/^color\([a-z][a-z0-9-]*(?:\s+-?[\d.]+(?:[eE][+-]?\d+)?%?)+(?:\s*\/\s*[\d.]+(?:[eE][+-]?\d+)?%?)?\)$/i.test(color)) return color;
    // NOTE: Scientific notation (e.g., "oklch(0.5 0.1e+2 240)") is rejected for
    // non-color() functions. Shiki does not emit scientific notation in legacy/modern
    // function syntax, so this restriction has no practical impact. The color() function
    // regex above does support scientific notation as required by CSS Color Level 4 spec.
    return "inherit";
}
