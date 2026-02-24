import React from 'react'
import {createRoot} from 'react-dom/client'
import './style.css'
import App from './App'
import {ErrorBoundary} from './components/ErrorBoundary'
import {logFrontendEventSafe} from './utils/logFrontendEventSafe'

// Register global uncaught-error handlers exactly once.
// Symbol.for() persists across HMR module reloads, preventing duplicate listener registration.
const GLOBAL_HANDLERS_KEY = Symbol.for("mytx.global.error.handlers");
type GlobalWithHandlers = typeof globalThis & { [key: symbol]: boolean };
if (!(globalThis as GlobalWithHandlers)[GLOBAL_HANDLERS_KEY]) {
    (globalThis as GlobalWithHandlers)[GLOBAL_HANDLERS_KEY] = true;

    window.addEventListener("error", (event) => {
        const message =
            typeof event.message === "string" && event.message !== ""
                ? event.message
                : "Unhandled error";
        logFrontendEventSafe("error", message, "frontend/unhandled");
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
