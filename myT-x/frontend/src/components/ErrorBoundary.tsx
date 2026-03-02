import React from "react";
import {logFrontendEventSafe} from "../utils/logFrontendEventSafe";
import {sliceByCodePoints} from "../utils/codePointUtils";
import {splitLines} from "../utils/textLines";

interface ErrorBoundaryProps {
    children: React.ReactNode;
}

interface ErrorBoundaryState {
    hasError: boolean;
    message: string;
    retryCount: number;
}

function toErrorMessage(error: unknown): string {
    return error instanceof Error ? error.message : String(error ?? "Unknown error");
}

/**
 * Catches unhandled React render/lifecycle errors and persists them to the
 * session log via LogFrontendEvent. Prevents the entire UI from going blank
 * on component tree failures and provides a retry mechanism.
 */
export class ErrorBoundary extends React.Component<ErrorBoundaryProps, ErrorBoundaryState> {
    private static readonly MAX_RETRIES = 3;

    constructor(props: ErrorBoundaryProps) {
        super(props);
        this.state = {hasError: false, message: "", retryCount: 0};
    }

    static getDerivedStateFromError(error: unknown): Partial<ErrorBoundaryState> {
        const message = toErrorMessage(error);
        return {hasError: true, message};
    }

    componentDidCatch(error: unknown, info: React.ErrorInfo): void {
        const message = toErrorMessage(error);
        // Clip component stack to avoid oversized log entries.
        // The Go-side LogFrontendEvent enforces its own rune-count cap independently.
        const stack =
            typeof info.componentStack === "string"
                ? sliceByCodePoints(info.componentStack.trim(), 0, 300)
                : "";
        // splitLines: codebase-wide utility for consistent LF/CRLF handling.
        const firstNonEmptyLine = stack !== "" ? splitLines(stack).find((line) => line.trim() !== "") ?? "" : "";
        const rawSource = firstNonEmptyLine !== "" ? `frontend/react ${firstNonEmptyLine}` : "frontend/react";
        // Use sliceByCodePoints for Unicode-safe rune-level slice (avoid splitting surrogate pairs).
        const source = sliceByCodePoints(rawSource, 0, 200);

        logFrontendEventSafe("error", message, source);
        if (this.state.retryCount >= ErrorBoundary.MAX_RETRIES) {
            logFrontendEventSafe("warn", "ErrorBoundary retry limit reached", source);
        }
    }

    private handleReset = (): void => {
        this.setState((state) => ({
            hasError: false,
            message: "",
            retryCount: state.retryCount + 1,
        }));
    };

    render(): React.ReactNode {
        if (!this.state.hasError) {
            return this.props.children;
        }
        const reachedRetryLimit = this.state.retryCount >= ErrorBoundary.MAX_RETRIES;
        return (
            <div className="error-boundary-fallback" role="alert">
                <p className="error-boundary-title">予期しないエラーが発生しました。</p>
                {this.state.message && (
                    <pre className="error-boundary-message">{this.state.message}</pre>
                )}
                {reachedRetryLimit ? (
                    <>
                        <p className="error-boundary-exhaust">再試行の上限に達しました。アプリケーションを再読み込みしてください。</p>
                        <button type="button" className="error-boundary-retry" onClick={() => window.location.reload()}>
                            再読み込み
                        </button>
                    </>
                ) : (
                    <button type="button" className="error-boundary-retry" onClick={this.handleReset}>
                        再試行
                    </button>
                )}
            </div>
        );
    }
}
