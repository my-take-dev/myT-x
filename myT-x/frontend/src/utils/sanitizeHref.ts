const SAFE_LINK_PROTOCOLS: ReadonlySet<string> = new Set(["http:", "https:", "mailto:", "tel:"]);
/** URI scheme detector (e.g., "https:", "mailto:"). */
const SCHEME_PATTERN = /^[A-Za-z][A-Za-z0-9+.-]*:/;
const STRIP_INVISIBLE_AND_CONTROL_CHARS_PATTERN =
    /[\r\n\t\0\u00AD\u200B-\u200F\u2028\u2029\u202A-\u202E\u2066-\u2069\uFEFF]/g;

export {SCHEME_PATTERN};

/**
 * Sanitize a raw href value to prevent XSS via dangerous URI schemes.
 *
 * Allowed:
 * - Fragment-only links (#anchor)
 * - Relative paths (/, ./, ../)
 * - Scheme-less strings (treated as relative)
 * - http:, https:, mailto:, tel: schemes
 *
 * Rejected (returns null):
 * - Empty / whitespace-only
 * - Protocol-relative URLs (//)
 * - javascript:, data:, vbscript:, and any other non-safe schemes
 * - Control/format separator characters are stripped before evaluation
 */
export function sanitizeHref(rawHref: string | undefined): string | null {
    if (!rawHref) return null;
    const href = rawHref.trim().replace(STRIP_INVISIBLE_AND_CONTROL_CHARS_PATTERN, "");
    if (href === "") return null;
    if (href.startsWith("#")) return href;
    // SECURITY: Protocol-relative URLs must be checked BEFORE the "/" relative-path check,
    // because "//" also starts with "/". Without this ordering, "//evil.com" would pass
    // through as a relative path.
    if (href.startsWith("//")) return null;
    if (href.startsWith("/") || href.startsWith("./") || href.startsWith("../")) return href;

    const hasExplicitScheme = SCHEME_PATTERN.test(href);
    if (!hasExplicitScheme) {
        // SECURITY: Percent-encoded scheme bypass defense.
        // "%6aavascript:alert(1)" fails SCHEME_PATTERN because "%" is not [A-Za-z],
        // but after percent-decoding it becomes "javascript:alert(1)".
        try {
            const decoded = decodeURIComponent(href);
            if (decoded !== href && decoded.startsWith("//")) {
                return null;
            }
            if (decoded !== href && SCHEME_PATTERN.test(decoded)) {
                // After decoding, an explicit scheme appeared - validate it.
                try {
                    const parsed = new URL(decoded);
                    if (!SAFE_LINK_PROTOCOLS.has(parsed.protocol)) return null;
                } catch {
                    return null;
                }
            }
        } catch (err: unknown) {
            console.warn("[sanitizeHref] URL validation failed:", err);
            // decodeURIComponent can throw on malformed percent sequences - allow as relative path.
        }
        return href;
    }

    try {
        const parsed = new URL(href);
        if (!SAFE_LINK_PROTOCOLS.has(parsed.protocol)) {
            return null;
        }
        return href;
    } catch {
        return null;
    }
}
