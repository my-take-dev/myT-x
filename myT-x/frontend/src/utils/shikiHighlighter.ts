/**
 * Shiki syntax highlighter singleton with lazy initialization,
 * on-demand language loading, and token caching.
 *
 * Uses shiki/core + JS regex engine to avoid bundling the WASM-based oniguruma engine.
 */
import type {HighlighterCore, ThemedToken} from "shiki/core";

// ── Singleton state ──

let highlighterPromise: Promise<HighlighterCore> | null = null;
let highlighter: HighlighterCore | null = null;
const loadedLangs = new Set<string>();

interface LangRetryState {
    attempts: number;
    nextRetryAtMs: number;
}

const unknownLangRetryUntil = new Map<string, number>();
const failedLangRetryState = new Map<string, LangRetryState>();
const langLoadingPromises = new Map<string, Promise<boolean>>();
const LANG_RETRY_BASE_COOLDOWN_MS = 30_000;
const LANG_RETRY_MAX_COOLDOWN_MS = 5 * 60_000;

// ── Token cache (insertion-order eviction) ──

interface TokenCacheEntry {
    lang: string;
    code: string;
    tokens: ThemedToken[][];
}

const tokenCache = new Map<string, TokenCacheEntry>();
/** Max cached highlight results. Sized for typical browsing session (10-20 open tabs + buffer). */
const CACHE_ENTRY_MAX = 24;
// Keep this above MAX_HIGHLIGHT_SIZE so at least one max-size source can be cached.
const CACHE_SOURCE_CHAR_BUDGET = 200_000;
let cachedSourceChars = 0;

// ── Size guard ──

/** Max source size (chars) for highlighting. Above this, show plain text to keep the UI responsive. */
export const MAX_HIGHLIGHT_SIZE = 20_000;
/** Max line count for highlighting. Limits tokenizer cost on deeply structured files. */
const MAX_HIGHLIGHT_LINE_COUNT = 1_200;
/** Max single-line length. Prevents regex backtracking on minified/generated code. */
const MAX_HIGHLIGHT_LINE_LENGTH = 2_000;

export type HighlightSkipReason = "size-limit" | "line-count-limit" | "line-length-limit";

export interface HighlightSkipInfo {
    readonly reason: HighlightSkipReason;
    readonly limit: number;
    readonly actual: number;
}

// ── Theme ──

const THEME = "github-dark";

// ── Extension -> Shiki language mapping ──

const EXT_MAP: Readonly<Record<string, string>> = {
    ts: "typescript",
    tsx: "tsx",
    js: "javascript",
    jsx: "jsx",
    mjs: "javascript",
    cjs: "javascript",
    go: "go",
    py: "python",
    rs: "rust",
    rb: "ruby",
    json: "json",
    jsonc: "jsonc",
    yaml: "yaml",
    yml: "yaml",
    toml: "toml",
    md: "markdown",
    markdown: "markdown",
    mdx: "mdx",
    env: "ini",
    bashrc: "shellscript",
    bash_profile: "shellscript",
    zshrc: "shellscript",
    profile: "shellscript",
    css: "css",
    scss: "scss",
    less: "less",
    html: "html",
    htm: "html",
    xml: "xml",
    svg: "xml",
    sh: "shellscript",
    bash: "shellscript",
    zsh: "shellscript",
    ps1: "powershell",
    bat: "bat",
    cmd: "bat",
    sql: "sql",
    graphql: "graphql",
    java: "java",
    kt: "kotlin",
    scala: "scala",
    c: "c",
    cpp: "cpp",
    h: "c",
    hpp: "cpp",
    cs: "csharp",
    fs: "fsharp",
    php: "php",
    lua: "lua",
    r: "r",
    swift: "swift",
    dart: "dart",
    dockerfile: "dockerfile",
    makefile: "makefile",
    diff: "diff",
    ini: "ini",
    conf: "ini",
    vue: "vue",
    svelte: "svelte",
    zig: "zig",
    elixir: "elixir",
    haskell: "haskell",
};

const BASENAME_MAP: Readonly<Record<string, string>> = {
    dockerfile: "dockerfile",
    makefile: "makefile",
    ".env": "ini",
    ".env.local": "ini",
    ".env.development": "ini",
    ".env.production": "ini",
    ".bashrc": "shellscript",
    ".bash_profile": "shellscript",
    ".zshrc": "shellscript",
    ".profile": "shellscript",
    ".npmrc": "ini",
    ".editorconfig": "ini",
};

/**
 * Override mapping from Shiki language ID to the actual module name in shiki/langs.
 * Add entries here only when a language's module filename differs from its language ID
 * (e.g., if shiki renamed "typescript" module to "ts", add: { typescript: "ts" }).
 */
const SHIKI_LANG_MODULE_OVERRIDES: Readonly<Record<string, string>> = {};

const SHIKI_LANG_MODULE_MAP: Readonly<Record<string, string>> = (() => {
    const map: Record<string, string> = {};
    const languageIds = new Set([
        ...Object.values(EXT_MAP),
        ...Object.values(BASENAME_MAP),
    ]);
    languageIds.forEach((lang) => {
        map[lang] = SHIKI_LANG_MODULE_OVERRIDES[lang] ?? lang;
    });
    return map;
})();

/**
 * Map a file extension (without dot) to a Shiki language ID.
 * Returns null for unknown extensions.
 */
export function extToShikiLang(ext: string): string | null {
    const normalizedExt = ext.trim().toLowerCase();
    return EXT_MAP[normalizedExt] ?? null;
}

/**
 * Extract language from a file path.
 * Handles both extension-based and filename-based detection.
 */
export function pathToShikiLang(filePath: string): string | null {
    const segments = filePath.split(/[/\\]/);
    const filename = segments[segments.length - 1] ?? "";
    const lower = filename.toLowerCase();

    const byName = BASENAME_MAP[lower];
    if (byName) return byName;

    const extIndex = lower.lastIndexOf(".");
    if (extIndex <= 0 || extIndex >= lower.length - 1) return null;
    const ext = lower.slice(extIndex + 1);
    return extToShikiLang(ext);
}

/**
 * Check whether a Shiki language ID represents a markdown variant.
 * Used by FileContentViewer to decide whether to offer the preview toggle.
 */
export function isMarkdownLang(lang: string | null): boolean {
    return lang === "markdown" || lang === "mdx";
}

// ── Sampled hash for cache keys ──

const CACHE_HASH_WINDOW = 128;
const CACHE_HASH_POINTS = 12;

function djb2Slice(str: string, start: number, length: number): number {
    // DJB2 hash over a substring window for cache fingerprinting.
    // Uses charCodeAt iteration to avoid temporary substring allocations.
    let hash = 5381;
    const end = Math.min(str.length, start + length);
    for (let i = start; i < end; i++) {
        hash = ((hash << 5) + hash + str.charCodeAt(i)) | 0;
    }
    return hash >>> 0;
}

function sampledPointHash(str: string, points: number): number {
    if (!str.length || points <= 0) return 0;
    // Sample evenly spaced points to bound hashing cost on large files.
    // Accepts slightly higher collision probability vs. full-content hash.
    let hash = 5381;
    const step = Math.max(1, Math.floor(str.length / points));
    for (let i = 0, index = 0; i < points && index < str.length; i++, index += step) {
        hash = ((hash << 5) + hash + str.charCodeAt(index)) | 0;
    }
    return hash >>> 0;
}

function codeFingerprint(code: string): string {
    const length = code.length;
    if (!length) return "0:0:0:0:0";

    const window = Math.min(CACHE_HASH_WINDOW, length);
    const middleStart = Math.max(0, Math.floor((length - window) / 2));
    const tailStart = Math.max(0, length - window);

    const headHash = djb2Slice(code, 0, window);
    const middleHash = djb2Slice(code, middleStart, window);
    const tailHash = djb2Slice(code, tailStart, window);
    const pointHash = sampledPointHash(code, CACHE_HASH_POINTS);

    return `${length}:${headHash}:${middleHash}:${tailHash}:${pointHash}`;
}

function cacheKey(lang: string, code: string): string {
    return `${lang}:${codeFingerprint(code)}`;
}

/**
 * Guard against expensive highlight jobs that can stall the main thread.
 * This is the size/complexity guard (sometimes referred to as 'skip highlight' check).
 */
export function getHighlightSkipInfo(code: string): HighlightSkipInfo | null {
    const codeLength = code.length;
    if (codeLength > MAX_HIGHLIGHT_SIZE) {
        return {reason: "size-limit", limit: MAX_HIGHLIGHT_SIZE, actual: codeLength};
    }

    let lineCount = 1;
    let currentLineLength = 0;
    let maxLineLength = 0;
    for (let i = 0; i < codeLength; i++) {
        const charCode = code.charCodeAt(i);
        if (charCode === 10) {
            if (currentLineLength > maxLineLength) {
                maxLineLength = currentLineLength;
            }
            lineCount++;
            currentLineLength = 0;
            continue;
        }
        if (charCode === 13) {
            continue;
        }
        currentLineLength++;
    }
    if (currentLineLength > maxLineLength) {
        maxLineLength = currentLineLength;
    }

    if (lineCount > MAX_HIGHLIGHT_LINE_COUNT) {
        return {reason: "line-count-limit", limit: MAX_HIGHLIGHT_LINE_COUNT, actual: lineCount};
    }
    if (maxLineLength > MAX_HIGHLIGHT_LINE_LENGTH) {
        return {reason: "line-length-limit", limit: MAX_HIGHLIGHT_LINE_LENGTH, actual: maxLineLength};
    }
    return null;
}

// ── Highlighter creation ──

function getOrCreateHighlighter(): Promise<HighlighterCore> {
    if (highlighterPromise) return highlighterPromise;

    highlighterPromise = Promise.all([
        import("shiki/core"),
        import("shiki/engine/javascript"),
        import("shiki/themes"),
    ]).then(async ([{createHighlighterCore}, {createJavaScriptRegexEngine}, themes]) => {
        const themeObj = themes.bundledThemes[THEME];
        if (!themeObj) throw new Error(`[shiki] Theme not found: ${THEME}`);
        const h = await createHighlighterCore({
            themes: [themeObj],
            langs: [], // Load languages on demand
            engine: createJavaScriptRegexEngine(),
        });
        highlighter = h;
        return h;
    }).catch((err: unknown) => {
        // Allow retry after initialization failure.
        highlighterPromise = null;
        highlighter = null;
        loadedLangs.clear();
        unknownLangRetryUntil.clear();
        failedLangRetryState.clear();
        langLoadingPromises.clear();
        console.error("[DEBUG-shiki] Highlighter initialization failed:", err);
        throw err;
    });

    return highlighterPromise;
}

type ShikiLanguageInput = Parameters<HighlighterCore["loadLanguage"]>[0];
type ShikiBundledLanguageLoaders = Record<string, ShikiLanguageInput>;

let bundledLanguageLoadersPromise: Promise<ShikiBundledLanguageLoaders> | null = null;

function getBundledLanguageLoaders(): Promise<ShikiBundledLanguageLoaders> {
    if (bundledLanguageLoadersPromise) return bundledLanguageLoadersPromise;

    bundledLanguageLoadersPromise = import("shiki/langs")
        .then(({bundledLanguages}) => (
            // Type boundary: shiki exports concrete language loaders while loadLanguage()
            // accepts a broader union type. Keep this cast localized here.
            bundledLanguages as ShikiBundledLanguageLoaders
        ))
        .catch((err: unknown) => {
            bundledLanguageLoadersPromise = null;
            console.error("[DEBUG-shiki] Failed to load bundled language loaders:", err);
            throw err;
        });

    return bundledLanguageLoadersPromise;
}

async function importLanguageModule(lang: string): Promise<ShikiLanguageInput | null> {
    const moduleName = SHIKI_LANG_MODULE_MAP[lang];
    if (!moduleName) {
        return null;
    }

    const bundledLanguageLoaders = await getBundledLanguageLoaders();
    return bundledLanguageLoaders[moduleName] ?? null;
}

/**
 * Ensure a language is loaded in the highlighter.
 * Loads grammar modules on demand to avoid a monolithic shiki/langs bundle.
 * Safe to call multiple times - tracks loaded languages.
 */
async function ensureLang(h: HighlighterCore, lang: string): Promise<boolean> {
    if (loadedLangs.has(lang)) return true;

    const now = Date.now();
    const unknownRetryUntil = unknownLangRetryUntil.get(lang) ?? 0;
    if (unknownRetryUntil > now) {
        return false;
    }

    const failedState = failedLangRetryState.get(lang);
    if (failedState && failedState.nextRetryAtMs > now) {
        return false;
    }

    const pending = langLoadingPromises.get(lang);
    if (pending) return pending;

    const loadPromise = (async () => {
        try {
            const langModule = await importLanguageModule(lang);
            if (!langModule) {
                unknownLangRetryUntil.set(lang, now + LANG_RETRY_MAX_COOLDOWN_MS);
                failedLangRetryState.delete(lang);
                if (import.meta.env.DEV) {
                    console.warn("[DEBUG-shiki] Unknown language:", lang);
                }
                return false;
            }
            await h.loadLanguage(langModule);
            loadedLangs.add(lang);
            unknownLangRetryUntil.delete(lang);
            failedLangRetryState.delete(lang);
            return true;
        } catch (err: unknown) {
            const previousAttempts = failedState?.attempts ?? 0;
            const attempts = previousAttempts + 1;
            const cooldown = Math.min(
                LANG_RETRY_BASE_COOLDOWN_MS * (2 ** Math.max(0, attempts - 1)),
                LANG_RETRY_MAX_COOLDOWN_MS,
            );
            const failedAt = Date.now();
            failedLangRetryState.set(lang, {
                attempts,
                nextRetryAtMs: failedAt + cooldown,
            });
            console.warn(`[DEBUG-shiki] Failed to load language "${lang}". Retrying in ${Math.ceil(cooldown / 1000)}s.`, err);
            return false;
        } finally {
            langLoadingPromises.delete(lang);
        }
    })();

    langLoadingPromises.set(lang, loadPromise);
    return loadPromise;
}

// ── Cache eviction ──

function evictOldest(): void {
    while (tokenCache.size > CACHE_ENTRY_MAX || cachedSourceChars > CACHE_SOURCE_CHAR_BUDGET) {
        if (tokenCache.size === 0) {
            cachedSourceChars = 0;
            break;
        }
        // Map iterates in insertion order - delete the oldest inserted key.
        const firstKey = tokenCache.keys().next().value;
        if (firstKey === undefined) {
            return;
        }
        const removedEntry = tokenCache.get(firstKey);
        tokenCache.delete(firstKey);
        if (removedEntry) {
            cachedSourceChars = Math.max(0, cachedSourceChars - removedEntry.code.length);
        }
    }
}

function removeCacheEntry(key: string): void {
    const removed = tokenCache.get(key);
    if (!removed) return;
    tokenCache.delete(key);
    cachedSourceChars = Math.max(0, cachedSourceChars - removed.code.length);
}

function upsertCacheEntry(key: string, entry: TokenCacheEntry): void {
    // Replace existing entry so insertion order reflects the most recent write.
    removeCacheEntry(key);
    tokenCache.set(key, entry);
    cachedSourceChars += entry.code.length;
    evictOldest();
}

function getCachedTokens(key: string, code: string, lang: string): ThemedToken[][] | null {
    const entry = tokenCache.get(key);
    if (!entry) return null;

    if (entry.lang === lang && entry.code === code) {
        return entry.tokens;
    }

    // Defensive: sampled fingerprints can collide, so never return mismatched tokens.
    if (import.meta.env.DEV) {
        console.warn(
            `[DEBUG-shiki] Cache key collision detected for lang="${lang}". ` +
            `Cached: lang="${entry.lang}", code.length=${entry.code.length}. ` +
            `Requested: code.length=${code.length}. Evicting stale entry.`
        );
    }
    removeCacheEntry(key);
    return null;
}

// ── Public API ──

/**
 * Highlight code and return an array of token lines.
 * Returns null if:
 * - Code exceeds size/line-count/line-length limits
 * - Language is unknown or failed to load (cooldown active)
 *
 * @throws When the Shiki `codeToTokens` API fails unexpectedly.
 *         The caller is responsible for catching and logging/notifying.
 *
 * This function is async but returns quickly for cached results.
 */
export async function highlightCode(
    code: string,
    lang: string,
): Promise<ThemedToken[][] | null> {
    // Intentionally duplicated with hook-level guard: direct callers are protected too.
    if (getHighlightSkipInfo(code)) return null;

    // Check cache
    const key = cacheKey(lang, code);
    const cached = getCachedTokens(key, code, lang);
    if (cached) return cached;

    // Get or create highlighter
    const h = await getOrCreateHighlighter();

    // Load language on demand
    const loaded = await ensureLang(h, lang);
    if (!loaded) return null;

    const result = h.codeToTokens(code, {lang, theme: THEME});
    upsertCacheEntry(key, {lang, code, tokens: result.tokens});
    return result.tokens;
}

/**
 * Pre-warm the Shiki highlighter in the background.
 * Call once at app startup. Fire-and-forget.
 */
export function preWarmHighlighter(): void {
    void getOrCreateHighlighter().catch((err: unknown) => {
        console.error("[DEBUG-shiki] preWarm failed:", err);
    });
}
