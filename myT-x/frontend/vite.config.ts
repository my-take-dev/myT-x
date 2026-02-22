import react from "@vitejs/plugin-react";
import {defineConfig} from "vite";

export default defineConfig({
    plugins: [react()],
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
                manualChunks: {
                    react: ["react", "react-dom"],
                    xterm: [
                        "@xterm/xterm",
                        "@xterm/addon-fit",
                        "@xterm/addon-search",
                        "@xterm/addon-webgl",
                        "@xterm/addon-web-links",
                    ],
                },
            },
            // NOTE: Do NOT set `treeshake: { moduleSideEffects: false }` here.
            // It strips CSS side-effect imports, xterm.js addon initialisers,
            // and polyfills. Rollup's default treeshake behaviour is sufficient.
        },
    },
});
