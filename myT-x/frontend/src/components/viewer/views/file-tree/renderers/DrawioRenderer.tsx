import {memo, useEffect, useRef, useState} from "react";
import {api} from "../../../../../api";
import {useShikiHighlight} from "../../../../../hooks/useShikiHighlight";
import {sanitizeCssColor} from "../../../../../utils/cssUtils";
import {matchesCapturedSessionKey} from "../../../../../utils/sessionGuard";
import {toErrorMessage} from "../../../../../utils/errorUtils";
import {createBinaryBlob} from "../binaryContentUtils";
import type {DocumentKind} from "../documentTypes";

interface DrawioRendererProps {
    readonly kind: Extract<DocumentKind, "drawio-svg" | "drawio-xml">;
    readonly content: string;
    readonly filePath?: string;
    readonly sessionKey?: string;
    readonly sessionName?: string | null;
}

interface HighlightedCodeBlockProps {
    readonly className: string;
    readonly code: string;
}

const DRAWIO_IMAGE_ALT = "draw.io diagram preview";

const HighlightedCodeBlock = memo(function HighlightedCodeBlock({className, code}: HighlightedCodeBlockProps) {
    const {tokens} = useShikiHighlight(code || undefined, undefined, "xml");

    return (
        <pre className="md-preview-code-block file-view-drawio-code">
            <code className={className}>
                {tokens
                    ? tokens.map((line, lineIndex) => (
                        <span key={`line-${lineIndex}`}>
                            {line.map((token, tokenIndex) => (
                                <span
                                    key={`token-${lineIndex}-${tokenIndex}`}
                                    style={{color: sanitizeCssColor(token.color)}}
                                >
                                    {token.content}
                                </span>
                            ))}
                            {lineIndex < tokens.length - 1 ? "\n" : null}
                        </span>
                    ))
                    : code}
            </code>
        </pre>
    );
});

export const DrawioRenderer = memo(function DrawioRenderer({
    kind,
    content,
    filePath,
    sessionKey,
    sessionName,
}: DrawioRendererProps) {
    const [imageURL, setImageURL] = useState<string | null>(null);
    const [loadError, setLoadError] = useState<string | null>(null);
    const latestSessionKeyRef = useRef(sessionKey ?? "");

    latestSessionKeyRef.current = sessionKey ?? "";

    useEffect(() => {
        if (kind !== "drawio-svg") {
            return;
        }

        const normalizedPath = filePath?.trim() ?? "";
        const normalizedSessionName = sessionName?.trim() ?? "";
        const normalizedSessionKey = sessionKey?.trim() ?? "";
        if (normalizedPath === "" || normalizedSessionName === "" || normalizedSessionKey === "") {
            setImageURL(null);
            setLoadError("Draw.io SVG preview requires an active session and file path.");
            return;
        }

        let disposed = false;
        let nextImageURL: string | null = null;
        const capturedSessionKey = normalizedSessionKey;

        setImageURL((previous) => {
            if (previous) {
                URL.revokeObjectURL(previous);
            }
            return null;
        });
        setLoadError(null);

        void api.DevPanelReadBinary(normalizedSessionName, normalizedPath)
            .then((binaryContent) => {
                if (disposed || !matchesCapturedSessionKey(capturedSessionKey, latestSessionKeyRef.current)) {
                    return;
                }

                nextImageURL = URL.createObjectURL(createBinaryBlob(binaryContent));
                setImageURL(nextImageURL);
            })
            .catch((err: unknown) => {
                if (disposed || !matchesCapturedSessionKey(capturedSessionKey, latestSessionKeyRef.current)) {
                    return;
                }

                console.warn("[drawio] failed to load svg preview", {
                    path: normalizedPath,
                    session: normalizedSessionName,
                    err,
                });
                setLoadError(toErrorMessage(err, "Failed to load draw.io SVG preview."));
            });

        return () => {
            disposed = true;
            if (nextImageURL) {
                URL.revokeObjectURL(nextImageURL);
            }
        };
    }, [content, filePath, kind, sessionKey, sessionName]);

    if (kind === "drawio-xml") {
        return (
            <div className="md-preview-body file-view-drawio">
                <HighlightedCodeBlock className="language-xml" code={content}/>
            </div>
        );
    }

    if (loadError) {
        return <div className="file-content-empty">{loadError}</div>;
    }

    if (imageURL === null) {
        return <div className="file-content-empty">Loading draw.io preview...</div>;
    }

    return (
        <div className="md-preview-body file-view-drawio">
            <div className="file-view-drawio-image-frame">
                <img
                    className="file-view-drawio-image"
                    src={imageURL}
                    alt={DRAWIO_IMAGE_ALT}
                />
            </div>
        </div>
    );
});
