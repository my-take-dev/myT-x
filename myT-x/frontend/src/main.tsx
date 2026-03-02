import React from 'react'
import {createRoot} from 'react-dom/client'
import './style.css'
import App from './App'
import {ErrorBoundary} from './components/ErrorBoundary'
import {logFrontendEventSafe} from './utils/logFrontendEventSafe'
import {preWarmHighlighter} from './utils/shikiHighlighter'

// Register global uncaught-error handlers exactly once.
// Symbol.for() persists across HMR module reloads, preventing duplicate listener registration.
const GLOBAL_HANDLERS_KEY = Symbol.for("mytx.global.error.handlers");
const SHIKI_PREWARM_KEY = Symbol.for("mytx.shiki.prewarm.scheduled");
/** Type augmentation for myT-x-specific Symbol keys stored on globalThis (HMR dedup guards). */
type GlobalWithMyTxKeys = typeof globalThis & {
    [GLOBAL_HANDLERS_KEY]?: boolean;
    [SHIKI_PREWARM_KEY]?: boolean;
};
if (!(globalThis as GlobalWithMyTxKeys)[GLOBAL_HANDLERS_KEY]) {
    (globalThis as GlobalWithMyTxKeys)[GLOBAL_HANDLERS_KEY] = true;

    window.addEventListener("error", (event) => {
        const message =
            typeof event.message === "string" && event.message !== ""
                ? event.message
                : "Unhandled error";
        const source = event.filename
            ? `frontend/unhandled @ ${event.filename}:${event.lineno}:${event.colno}`
            : "frontend/unhandled";
        logFrontendEventSafe("error", message, source);
    });

    window.addEventListener("unhandledrejection", (event) => {
        const reason =
            event.reason instanceof Error
                ? event.reason.message
                : typeof event.reason === "string" && event.reason !== ""
                    ? event.reason
                    : "Unhandled promise rejection";
        logFrontendEventSafe("error", reason, "frontend/promise");
    });
}

const container = document.getElementById("root");
if (!container) {
    throw new Error("[main] Root element #root not found in document");
}
const root = createRoot(container);

root.render(
    <React.StrictMode>
        <ErrorBoundary>
            <App/>
        </ErrorBoundary>
    </React.StrictMode>
)

// Pre-warm Shiki after initial render to avoid startup contention.
// Fallback delay for browsers without requestIdleCallback (e.g., older WebView).
// Why 1000ms: Chosen to allow the initial React render + paint cycle to complete before
// triggering Shiki's WASM/grammar loading. Measured on development hardware (Wails WebView2):
// React initial mount + first paint typically completes within 200-400ms. 1000ms provides
// a 2-3x safety margin. Shorter values (e.g., 300ms) risk contending with React rendering;
// longer values unnecessarily delay syntax highlighting readiness.
// requestIdleCallback is preferred when available and makes this fallback less critical.
const PREWARM_FALLBACK_DELAY_MS = 1000;
if (!(globalThis as GlobalWithMyTxKeys)[SHIKI_PREWARM_KEY]) {
    (globalThis as GlobalWithMyTxKeys)[SHIKI_PREWARM_KEY] = true;
    // requestIdleCallback timeout (2000ms) > PREWARM_FALLBACK_DELAY_MS (1000ms):
    // The rIC timeout is a *maximum wait* before forced execution, allowing idle detection.
    // The fallback setTimeout fires unconditionally after 1s in environments without rIC.
    // The asymmetry is intentional: rIC environments benefit from waiting longer for an idle window,
    // while fallback environments get a fixed, shorter delay to avoid startup contention.
    if (typeof window.requestIdleCallback === "function") {
        window.requestIdleCallback(() => {
            void preWarmHighlighter();
        }, {timeout: 2000});
    } else {
        window.setTimeout(() => {
            void preWarmHighlighter();
        }, PREWARM_FALLBACK_DELAY_MS);
    }
}
