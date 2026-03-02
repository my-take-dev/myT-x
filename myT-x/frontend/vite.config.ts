import react from "@vitejs/plugin-react";
import {defineConfig} from "vitest/config";

const SHIKI_COMMON_LANGS = new Set([
    "typescript",
    "tsx",
    "javascript",
    "jsx",
    "json",
    "jsonc",
    "markdown",
    "mdx",
    "diff",
    "yaml",
    "toml",
    "ini",
    "go",
    "shellscript",
    "powershell",
    "bat",
]);
const SHIKI_LANG_PATH_PATTERN = /\/node_modules\/(?:@shikijs\/langs|shiki)\/(?:dist\/langs\/|langs\/(?:dist\/)?)/;
// Matches all Shiki ecosystem packages (@shikijs/* and shiki/*).
// SHIKI_LANG_PATH_PATTERN is evaluated first, so language files are excluded from shiki-core.
const SHIKI_PACKAGE_PATH_PATTERN = /\/node_modules\/(?:@shikijs\/|shiki\/)/;

/**
 * Resolve a Shiki language module path to a chunk name.
 * @param normalizedId - Module ID with forward slashes (already normalized by caller).
 * @returns The chunk name for the resolved language module.
 */
function resolveShikiLangChunk(normalizedId: string): string {
    // Supports both:
    // - shiki/dist/langs/<lang>.mjs
    // - @shikijs/langs/dist/<lang>.mjs
    // - @shikijs/langs/langs/dist/<lang>.mjs (fallback packaging variants)
    // Pattern `\.m?js$` matches .js and .mjs — tied to Shiki v4.x distribution format
    // which ships language grammars as .mjs ESM files. If a future Shiki version changes
    // the extension (e.g., to .cjs or .ts), this pattern must be updated accordingly.
    const langMatch = normalizedId.match(/\/(?:dist\/langs|langs(?:\/dist)?)\/([^/.]+)\.m?js$/);
    const lang = langMatch?.[1]?.toLowerCase();
    if (!lang) {
        return "shiki-langs-misc";
    }
    if (SHIKI_COMMON_LANGS.has(lang)) {
        return "shiki-langs-common";
    }

    // Split remaining Shiki language modules into 3 roughly equal-sized chunks
    // by first letter: a-h, i-p, q-z (based on current Shiki language distribution).
    // FIXME: shiki v4.x specific. Re-verify chunk distribution after any shiki major version upgrade.
    const first = lang[0];
    if (first >= "a" && first <= "h") {
        return "shiki-langs-ext-a-h";
    }
    if (first >= "i" && first <= "p") {
        return "shiki-langs-ext-i-p";
    }
    return "shiki-langs-ext-q-z";
}

export default defineConfig({
    plugins: [react()],
    test: {
        environment: "jsdom",
        globals: true,
    },
    build: {
        // terser provides better minification than esbuild default (~5-10% smaller).
        // Two compression passes catch cross-reference optimization opportunities.
        minify: "terser",
        terserOptions: {
            compress: {
                drop_console: false, // Retain console.warn for production diagnostics.
                passes: 2,
            },
        },
        rollupOptions: {
            output: {
                manualChunks(id) {
                    const normalizedId = id.replace(/\\/g, "/");

                    if (SHIKI_LANG_PATH_PATTERN.test(normalizedId)) {
                        return resolveShikiLangChunk(normalizedId);
                    }
                    if (SHIKI_PACKAGE_PATH_PATTERN.test(normalizedId)) {
                        return "shiki-core";
                    }
                    // includes("/node_modules/react/") matches only the core "react" package.
                    // includes("/node_modules/react-dom/") matches react-dom AND its sub-paths
                    // (e.g., react-dom/client) because the trailing slash is part of the prefix.
                    if (normalizedId.includes("/node_modules/react/") || normalizedId.includes("/node_modules/react-dom/")) {
                        return "react";
                    }
                    if (normalizedId.includes("/node_modules/@xterm/")) {
                        return "xterm";
                    }
                    // react-markdown and remark-gfm are explicitly assigned to the markdown chunk.
                    // Include major transitive packages to keep markdown cost out of the default chunk.
                    if (
                        normalizedId.includes("/node_modules/react-markdown/")
                        || normalizedId.includes("/node_modules/remark-gfm/")
                        || normalizedId.includes("/node_modules/unified/")
                        || normalizedId.includes("/node_modules/mdast-util-")
                        || normalizedId.includes("/node_modules/micromark/")
                    ) {
                        return "markdown";
                    }
                    if (normalizedId.includes("/node_modules/react-window/")) {
                        return "react-window";
                    }
                    return undefined;
                },
            },
            // NOTE: Do NOT set `treeshake: { moduleSideEffects: false }` here.
            // It strips CSS side-effect imports, xterm.js addon initialisers,
            // and polyfills. Rollup's default treeshake behaviour is sufficient.
        },
    },
});
