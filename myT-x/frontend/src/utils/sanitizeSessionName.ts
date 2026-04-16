const sessionNameSanitizer = /[.:]+/g;
const consecutiveHyphen = /-{2,}/g;

// sanitizeSessionName matches the backend tmux session-name sanitizer.
export function sanitizeSessionName(name: string): string {
    let sanitized = name.replace(sessionNameSanitizer, "-");
    sanitized = sanitized.replace(consecutiveHyphen, "-");
    sanitized = sanitized.replace(/^-+|-+$/g, "");
    return sanitized;
}

// suggestSessionName applies the backend-compatible sanitizer and fallback.
export function suggestSessionName(name: string, fallback: string): string {
    const sanitized = sanitizeSessionName(name);
    return sanitized !== "" ? sanitized : fallback;
}
