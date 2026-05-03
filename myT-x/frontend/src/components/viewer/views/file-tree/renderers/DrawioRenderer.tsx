import {memo, useEffect, useId, useMemo, useRef, useState} from "react";
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

interface DrawioPreviewShape {
    readonly id: string;
    readonly label: string;
    readonly x: number;
    readonly y: number;
    readonly width: number;
    readonly height: number;
    readonly kind: "rect" | "ellipse" | "rhombus";
    readonly rounded: boolean;
    readonly fill: string;
    readonly stroke: string;
}

interface DrawioPoint {
    readonly x: number;
    readonly y: number;
}

interface DrawioPreviewEdge {
    readonly id: string;
    readonly label: string;
    readonly sourceId: string | null;
    readonly targetId: string | null;
    readonly sourcePoint: DrawioPoint | null;
    readonly targetPoint: DrawioPoint | null;
    readonly stroke: string;
}

interface DrawioPreviewModel {
    readonly shapes: readonly DrawioPreviewShape[];
    readonly edges: readonly DrawioPreviewEdge[];
    readonly viewBox: string;
    readonly aspectRatio: string;
}

type DrawioXmlPreviewState =
    | { readonly status: "loading" }
    | { readonly status: "ready"; readonly model: DrawioPreviewModel; readonly message: string | null }
    | { readonly status: "source"; readonly message: string };

interface ExtractedDrawioModelXml {
    readonly modelXml: string;
    readonly totalPages: number;
    readonly displayedPageIndex: number;
}

type DrawioPreviewFailureReason =
    | "empty-source"
    | "invalid-xml"
    | "missing-model"
    | "no-renderable-nodes"
    | "compressed-preview-decode-failed";

interface DrawioPreviewFailure {
    readonly status: "error";
    readonly phase: "extract" | "decompress" | "parse";
    readonly reason: DrawioPreviewFailureReason;
    readonly message: string;
    readonly cause?: unknown;
}

type DrawioModelExtractionResult =
    | ({ readonly status: "ok" } & ExtractedDrawioModelXml)
    | DrawioPreviewFailure;

type DrawioModelParseResult =
    | { readonly status: "ok"; readonly model: DrawioPreviewModel }
    | DrawioPreviewFailure;

const DRAWIO_IMAGE_ALT = "draw.io diagram preview";
const DRAWIO_PREVIEW_PADDING = 48;
const DRAWIO_TEXT_LINE_HEIGHT = 14;
const DRAWIO_TEXT_MAX_CHARS = 34;
const DRAWIO_TEXT_MAX_LINES = 4;
const DRAWIO_DECOMPRESSED_TEXT_LIMIT_BYTES = 10 * 1024 * 1024;
const DRAWIO_RENDER_FALLBACK_MESSAGE = "Unable to render this draw.io diagram. Showing source XML instead.";
const DRAWIO_EMPTY_SOURCE_FALLBACK_MESSAGE = "Empty draw.io document. Showing source XML instead.";
const DRAWIO_INVALID_XML_FALLBACK_MESSAGE = "Invalid draw.io XML. Showing source XML instead.";
const DRAWIO_MISSING_MODEL_FALLBACK_MESSAGE = "No draw.io graph model was found. Showing source XML instead.";
const DRAWIO_NO_RENDERABLE_NODES_FALLBACK_MESSAGE = "No renderable draw.io shapes were found. Showing source XML instead.";
const DRAWIO_COMPRESSED_DECODE_FALLBACK_MESSAGE = "Unable to decode this compressed draw.io diagram. Showing source XML instead.";
// TODO: Remove the cast when the project's DOM typings consistently include "deflate-raw".
const DRAWIO_DEFLATE_RAW_FORMAT = "deflate-raw" as CompressionFormat;
const htmlParser = new DOMParser();
const xmlParser = new DOMParser();

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

function parseStyle(style: string | null): Map<string, string> {
    const entries = new Map<string, string>();
    if (!style) {
        return entries;
    }

    for (const segment of style.split(";")) {
        if (segment === "") {
            continue;
        }
        const separatorIndex = segment.indexOf("=");
        if (separatorIndex < 0) {
            entries.set(segment, "1");
            continue;
        }
        entries.set(segment.slice(0, separatorIndex), segment.slice(separatorIndex + 1));
    }
    return entries;
}

function getStyleColor(style: Map<string, string>, key: string, fallback: string): string {
    const value = style.get(key);
    if (!value || value === "none") {
        return fallback;
    }
    const sanitized = sanitizeCssColor(value);
    return sanitized === "inherit" ? fallback : sanitized;
}

function isDrawioObjectElement(element: Element | null): boolean {
    return element?.localName === "UserObject" || element?.localName === "Object";
}

function firstNonEmptyAttribute(element: Element | null, names: readonly string[]): string | null {
    if (!element) {
        return null;
    }
    for (const name of names) {
        const value = element.getAttribute(name);
        if (value) {
            return value;
        }
    }
    return null;
}

function htmlLabelToText(value: string): string {
    const htmlDoc = htmlParser.parseFromString(value, "text/html");
    htmlDoc.body.querySelectorAll("br").forEach((br) => br.replaceWith("\n"));
    htmlDoc.body.querySelectorAll("p, div").forEach((block) => {
        if (block.nextSibling) {
            block.append("\n");
        }
    });
    return htmlDoc.body.textContent?.trim() ?? "";
}

function getElementLabel(cell: Element): string {
    const parent = cell.parentElement;
    const directValue = firstNonEmptyAttribute(cell, ["value"]);
    const inheritedValue = isDrawioObjectElement(parent)
        ? firstNonEmptyAttribute(parent, ["label", "value"])
        : null;
    const value = directValue ?? inheritedValue ?? "";
    if (value === "") {
        return "";
    }

    return htmlLabelToText(value);
}

function getCellId(cell: Element): string {
    return cell.getAttribute("id")
        ?? (isDrawioObjectElement(cell.parentElement) ? cell.parentElement?.getAttribute("id") : null)
        ?? "";
}

function getGeometry(cell: Element): Element | null {
    for (const child of Array.from(cell.children)) {
        if (child.localName === "mxGeometry") {
            return child;
        }
    }
    return null;
}

function getNumericAttribute(element: Element, name: string, fallback: number): number {
    const value = Number.parseFloat(element.getAttribute(name) ?? "");
    return Number.isFinite(value) ? value : fallback;
}

function getEdgePoint(geometry: Element | null, pointName: "sourcePoint" | "targetPoint"): DrawioPoint | null {
    if (!geometry) {
        return null;
    }
    for (const child of Array.from(geometry.children)) {
        if (child.localName !== "mxPoint" || child.getAttribute("as") !== pointName) {
            continue;
        }
        const x = Number.parseFloat(child.getAttribute("x") ?? "");
        const y = Number.parseFloat(child.getAttribute("y") ?? "");
        return Number.isFinite(x) && Number.isFinite(y) ? {x, y} : null;
    }
    return null;
}

function getShapeKind(style: Map<string, string>): DrawioPreviewShape["kind"] {
    const shape = style.get("shape");
    if (shape === "ellipse" || style.has("ellipse")) {
        return "ellipse";
    }
    if (shape === "rhombus" || style.has("rhombus")) {
        return "rhombus";
    }
    return "rect";
}

function hasXmlParserError(doc: Document): boolean {
    if (doc.documentElement.localName === "parsererror") {
        return true;
    }
    return Array.from(doc.getElementsByTagName("*")).some((element) => element.localName === "parsererror");
}

function isAbortError(err: unknown): boolean {
    return err instanceof DOMException && err.name === "AbortError";
}

function createDrawioPreviewFailure(
    phase: DrawioPreviewFailure["phase"],
    reason: DrawioPreviewFailureReason,
    message: string,
    cause?: unknown,
): DrawioPreviewFailure {
    return {status: "error", phase, reason, message, cause};
}

function getCompressedPreviewFailureMessage(err: unknown): string {
    const message = toErrorMessage(err, DRAWIO_COMPRESSED_DECODE_FALLBACK_MESSAGE);
    if (message === DRAWIO_COMPRESSED_DECODE_FALLBACK_MESSAGE || message.startsWith("Decompressed draw.io XML exceeds")) {
        return message;
    }
    return `${DRAWIO_COMPRESSED_DECODE_FALLBACK_MESSAGE} ${message}`;
}

function parseDrawioModel(modelXml: string): DrawioModelParseResult {
    if (modelXml.trim() === "") {
        return createDrawioPreviewFailure("parse", "empty-source", DRAWIO_EMPTY_SOURCE_FALLBACK_MESSAGE);
    }
    const doc = xmlParser.parseFromString(modelXml, "application/xml");
    if (hasXmlParserError(doc)) {
        return createDrawioPreviewFailure("parse", "invalid-xml", DRAWIO_INVALID_XML_FALLBACK_MESSAGE);
    }

    const shapes: DrawioPreviewShape[] = [];
    const edges: DrawioPreviewEdge[] = [];
    const cells = Array.from(doc.getElementsByTagName("mxCell"));

    for (const cell of cells) {
        const id = getCellId(cell);
        if (id === "") {
            continue;
        }

        const style = parseStyle(cell.getAttribute("style"));
        if (cell.getAttribute("vertex") === "1") {
            const geometry = getGeometry(cell);
            if (!geometry) {
                continue;
            }
            const width = getNumericAttribute(geometry, "width", 0);
            const height = getNumericAttribute(geometry, "height", 0);
            if (width <= 0 || height <= 0) {
                continue;
            }
            shapes.push({
                id,
                label: getElementLabel(cell),
                x: getNumericAttribute(geometry, "x", 0),
                y: getNumericAttribute(geometry, "y", 0),
                width,
                height,
                kind: getShapeKind(style),
                rounded: style.get("rounded") === "1",
                fill: getStyleColor(style, "fillColor", "var(--bg-panel)"),
                stroke: getStyleColor(style, "strokeColor", "var(--line)"),
            });
            continue;
        }

        if (cell.getAttribute("edge") === "1") {
            const geometry = getGeometry(cell);
            edges.push({
                id,
                label: getElementLabel(cell),
                sourceId: cell.getAttribute("source"),
                targetId: cell.getAttribute("target"),
                sourcePoint: getEdgePoint(geometry, "sourcePoint"),
                targetPoint: getEdgePoint(geometry, "targetPoint"),
                stroke: getStyleColor(style, "strokeColor", "var(--fg-dim)"),
            });
        }
    }

    if (shapes.length === 0) {
        return createDrawioPreviewFailure("parse", "no-renderable-nodes", DRAWIO_NO_RENDERABLE_NODES_FALLBACK_MESSAGE);
    }

    let minX = Number.POSITIVE_INFINITY;
    let minY = Number.POSITIVE_INFINITY;
    let maxX = Number.NEGATIVE_INFINITY;
    let maxY = Number.NEGATIVE_INFINITY;
    for (const shape of shapes) {
        minX = Math.min(minX, shape.x);
        minY = Math.min(minY, shape.y);
        maxX = Math.max(maxX, shape.x + shape.width);
        maxY = Math.max(maxY, shape.y + shape.height);
    }
    for (const edge of edges) {
        for (const point of [edge.sourcePoint, edge.targetPoint]) {
            if (!point) {
                continue;
            }
            minX = Math.min(minX, point.x);
            minY = Math.min(minY, point.y);
            maxX = Math.max(maxX, point.x);
            maxY = Math.max(maxY, point.y);
        }
    }

    minX -= DRAWIO_PREVIEW_PADDING;
    minY -= DRAWIO_PREVIEW_PADDING;
    maxX += DRAWIO_PREVIEW_PADDING;
    maxY += DRAWIO_PREVIEW_PADDING;
    const viewBoxWidth = Math.max(maxX - minX, 1);
    const viewBoxHeight = Math.max(maxY - minY, 1);

    return {
        status: "ok",
        model: {
            shapes,
            edges,
            viewBox: `${minX} ${minY} ${viewBoxWidth} ${viewBoxHeight}`,
            aspectRatio: `${viewBoxWidth} / ${viewBoxHeight}`,
        },
    };
}

function throwIfAborted(signal?: AbortSignal): void {
    if (signal?.aborted) {
        throw new DOMException("Draw.io preview parsing was aborted.", "AbortError");
    }
}

async function readLimitedTextStream(stream: ReadableStream<Uint8Array>, signal?: AbortSignal): Promise<string> {
    const reader = stream.getReader();
    const decoder = new TextDecoder();
    let decoded = "";
    let totalBytes = 0;
    const cancelReaderOnAbort = () => {
        void reader.cancel().catch((err: unknown) => {
            console.warn("[drawio] failed to cancel aborted preview stream:", err);
        });
    };

    try {
        signal?.addEventListener("abort", cancelReaderOnAbort, {once: true});
        while (true) {
            throwIfAborted(signal);
            const {done, value} = await reader.read();
            throwIfAborted(signal);
            if (done) {
                break;
            }
            totalBytes += value.byteLength;
            if (totalBytes > DRAWIO_DECOMPRESSED_TEXT_LIMIT_BYTES) {
                try {
                    await reader.cancel();
                } catch (err: unknown) {
                    console.warn("[drawio] failed to cancel oversized preview stream:", err);
                }
                throw new Error("Decompressed draw.io XML exceeds the 10 MB render limit. Showing source XML instead.");
            }
            decoded += decoder.decode(value, {stream: true});
        }
        return decoded + decoder.decode();
    } finally {
        signal?.removeEventListener("abort", cancelReaderOnAbort);
        reader.releaseLock();
    }
}

async function decompressDiagramContent(encoded: string, signal?: AbortSignal): Promise<string> {
    if (!("DecompressionStream" in globalThis)) {
        throw new Error(DRAWIO_COMPRESSED_DECODE_FALLBACK_MESSAGE);
    }

    throwIfAborted(signal);
    const normalizedEncoded = encoded.replace(/\s+/g, "");
    const compressed = Uint8Array.from(atob(normalizedEncoded), (character) => character.charCodeAt(0));
    const compressedStream = new Response(compressed).body;
    if (!compressedStream) {
        throw new Error(DRAWIO_COMPRESSED_DECODE_FALLBACK_MESSAGE);
    }
    const stream = compressedStream.pipeThrough(new DecompressionStream(DRAWIO_DEFLATE_RAW_FORMAT), {signal});
    const inflated = await readLimitedTextStream(stream, signal);
    if (inflated.includes("<mxGraphModel") || inflated.includes("<mxfile")) {
        return inflated;
    }
    try {
        return decodeURIComponent(inflated);
    } catch {
        return inflated;
    }
}

function serializeFirstMxGraphModel(element: Element): string | null {
    if (element.localName === "mxGraphModel") {
        return new XMLSerializer().serializeToString(element);
    }
    const embeddedModel = element.getElementsByTagName("mxGraphModel")[0];
    return embeddedModel ? new XMLSerializer().serializeToString(embeddedModel) : null;
}

async function extractDrawioModelXml(content: string, signal?: AbortSignal): Promise<DrawioModelExtractionResult> {
    if (content.trim() === "") {
        return createDrawioPreviewFailure("extract", "empty-source", DRAWIO_EMPTY_SOURCE_FALLBACK_MESSAGE);
    }
    const doc = xmlParser.parseFromString(content, "application/xml");
    if (hasXmlParserError(doc)) {
        return createDrawioPreviewFailure("extract", "invalid-xml", DRAWIO_INVALID_XML_FALLBACK_MESSAGE);
    }

    const rootModelXml = serializeFirstMxGraphModel(doc.documentElement);
    if (doc.documentElement.localName === "mxGraphModel" && rootModelXml) {
        return {status: "ok", modelXml: rootModelXml, totalPages: 1, displayedPageIndex: 0};
    }

    const diagrams = Array.from(doc.getElementsByTagName("diagram"));
    for (const [displayedPageIndex, diagram] of diagrams.entries()) {
        const diagramModelXml = serializeFirstMxGraphModel(diagram);
        if (diagramModelXml) {
            return {status: "ok", modelXml: diagramModelXml, totalPages: diagrams.length, displayedPageIndex};
        }
        const encodedDiagram = diagram.textContent?.trim();
        if (encodedDiagram) {
            try {
                return {
                    status: "ok",
                    modelXml: await decompressDiagramContent(encodedDiagram, signal),
                    totalPages: diagrams.length,
                    displayedPageIndex,
                };
            } catch (err: unknown) {
                if (isAbortError(err)) {
                    throw err;
                }
                return createDrawioPreviewFailure(
                    "decompress",
                    "compressed-preview-decode-failed",
                    getCompressedPreviewFailureMessage(err),
                    err,
                );
            }
        }
    }

    return rootModelXml
        ? {status: "ok", modelXml: rootModelXml, totalPages: 1, displayedPageIndex: 0}
        : createDrawioPreviewFailure("extract", "missing-model", DRAWIO_MISSING_MODEL_FALLBACK_MESSAGE);
}

function splitLabel(label: string): string[] {
    if (label === "") {
        return [];
    }

    const lines = label
        .split(/\r?\n/)
        .flatMap((line) => {
            const characters = Array.from(line);
            if (characters.length <= DRAWIO_TEXT_MAX_CHARS) {
                return [line];
            }
            const chunks: string[] = [];
            for (let offset = 0; offset < characters.length; offset += DRAWIO_TEXT_MAX_CHARS) {
                chunks.push(characters.slice(offset, offset + DRAWIO_TEXT_MAX_CHARS).join(""));
            }
            return chunks;
        });
    if (lines.length <= DRAWIO_TEXT_MAX_LINES) {
        return lines;
    }
    const visibleLines = lines.slice(0, DRAWIO_TEXT_MAX_LINES);
    const lastIndex = visibleLines.length - 1;
    const lastLine = Array.from(visibleLines[lastIndex] ?? "");
    const suffix = "...";
    if (DRAWIO_TEXT_MAX_CHARS > suffix.length) {
        visibleLines[lastIndex] = `${lastLine.slice(0, DRAWIO_TEXT_MAX_CHARS - suffix.length).join("")}${suffix}`;
    }
    return visibleLines;
}

function firstEdgeLabelLine(label: string): string {
    const firstLine = label.split(/\r?\n/, 1)[0] ?? "";
    return Array.from(firstLine).slice(0, DRAWIO_TEXT_MAX_CHARS).join("");
}

function getShapeCenter(shape: DrawioPreviewShape): DrawioPoint {
    return {
        x: shape.x + shape.width / 2,
        y: shape.y + shape.height / 2,
    };
}

const DrawioShapeView = memo(function DrawioShapeView({shape}: { readonly shape: DrawioPreviewShape }) {
    const labelLines = useMemo(() => splitLabel(shape.label), [shape.label]);
    const centerX = shape.x + shape.width / 2;
    const centerY = shape.y + shape.height / 2;
    const firstLineY = centerY - ((labelLines.length - 1) * DRAWIO_TEXT_LINE_HEIGHT) / 2;

    return (
        <g>
            {shape.label ? <title>{shape.label}</title> : null}
            {shape.kind === "ellipse" ? (
                <ellipse
                    cx={centerX}
                    cy={centerY}
                    rx={shape.width / 2}
                    ry={shape.height / 2}
                    fill={shape.fill}
                    stroke={shape.stroke}
                />
            ) : shape.kind === "rhombus" ? (
                <polygon
                    points={[
                        `${centerX},${shape.y}`,
                        `${shape.x + shape.width},${centerY}`,
                        `${centerX},${shape.y + shape.height}`,
                        `${shape.x},${centerY}`,
                    ].join(" ")}
                    fill={shape.fill}
                    stroke={shape.stroke}
                />
            ) : (
                <rect
                    x={shape.x}
                    y={shape.y}
                    width={shape.width}
                    height={shape.height}
                    rx={shape.rounded ? Math.min(12, shape.width / 8, shape.height / 8) : 0}
                    fill={shape.fill}
                    stroke={shape.stroke}
                />
            )}
            {labelLines.length > 0 ? (
                <text
                    x={centerX}
                    y={firstLineY}
                    textAnchor="middle"
                    dominantBaseline="middle"
                    className="file-view-drawio-label"
                >
                    {labelLines.map((line, index) => (
                        <tspan key={`${shape.id}-${index}`} x={centerX} dy={index === 0 ? 0 : DRAWIO_TEXT_LINE_HEIGHT}>
                            {line}
                        </tspan>
                    ))}
                </text>
            ) : null}
        </g>
    );
});

const DrawioEdgeView = memo(function DrawioEdgeView({
    edge,
    markerId,
    shapesById,
}: {
    readonly edge: DrawioPreviewEdge;
    readonly markerId: string;
    readonly shapesById: ReadonlyMap<string, DrawioPreviewShape>;
}) {
    const source = edge.sourceId ? shapesById.get(edge.sourceId) : null;
    const target = edge.targetId ? shapesById.get(edge.targetId) : null;
    const labelLine = useMemo(() => firstEdgeLabelLine(edge.label), [edge.label]);
    const sourcePoint = source ? getShapeCenter(source) : edge.sourcePoint;
    const targetPoint = target ? getShapeCenter(target) : edge.targetPoint;
    if (!sourcePoint || !targetPoint) {
        return null;
    }

    return (
        <g>
            {edge.label ? <title>{edge.label}</title> : null}
            <line
                x1={sourcePoint.x}
                y1={sourcePoint.y}
                x2={targetPoint.x}
                y2={targetPoint.y}
                stroke={edge.stroke}
                markerEnd={`url(#${markerId})`}
            />
            {labelLine ? (
                <text
                    x={(sourcePoint.x + targetPoint.x) / 2}
                    y={(sourcePoint.y + targetPoint.y) / 2 - 6}
                    textAnchor="middle"
                    className="file-view-drawio-label"
                >
                    {labelLine}
                </text>
            ) : null}
        </g>
    );
});

function DrawioXmlPreview({content, filePath}: { readonly content: string; readonly filePath?: string }) {
    const [state, setState] = useState<DrawioXmlPreviewState>({status: "loading"});
    const previewId = useId().replace(/[^a-zA-Z0-9_-]/g, "");
    const markerPrefix = `drawio-arrowhead-${previewId || "preview"}`;

    useEffect(() => {
        let disposed = false;
        const abortController = new AbortController();
        setState({status: "loading"});

        void extractDrawioModelXml(content, abortController.signal)
            .then((extracted) => {
                if (disposed) {
                    return;
                }
                if (extracted.status === "error") {
                    console.warn("[drawio] failed to render xml preview", {
                        phase: extracted.phase,
                        reason: extracted.reason,
                        filePath,
                        cause: extracted.cause,
                    });
                    setState({
                        status: "source",
                        message: extracted.message,
                    });
                    return;
                }

                const parsed = parseDrawioModel(extracted.modelXml);
                if (parsed.status === "error") {
                    console.warn("[drawio] failed to render xml preview", {
                        phase: parsed.phase,
                        reason: parsed.reason,
                        filePath,
                        cause: parsed.cause,
                    });
                    setState({
                        status: "source",
                        message: parsed.message,
                    });
                    return;
                }

                setState({
                    status: "ready",
                    model: parsed.model,
                    message: extracted.totalPages > 1
                        ? `Showing page ${extracted.displayedPageIndex + 1} of ${extracted.totalPages}.`
                        : null,
                });
            })
            .catch((err: unknown) => {
                if (disposed || isAbortError(err)) {
                    return;
                }
                console.warn("[drawio] failed to render xml preview", {
                    phase: "extract",
                    reason: "missing-model",
                    filePath,
                    cause: err,
                });
                setState({
                    status: "source",
                    message: toErrorMessage(err, DRAWIO_RENDER_FALLBACK_MESSAGE),
                });
            });

        return () => {
            disposed = true;
            abortController.abort();
        };
    }, [content, filePath]);

    const shapesById = useMemo(
        () => new Map(state.status === "ready" ? state.model.shapes.map((shape) => [shape.id, shape]) : []),
        [state],
    );

    if (state.status === "loading") {
        return <div className="file-content-empty">Loading draw.io preview...</div>;
    }

    if (state.status === "source") {
        return (
            <div className="md-preview-body file-view-drawio">
                <div className="file-view-drawio-warning">{state.message}</div>
                <HighlightedCodeBlock className="language-xml" code={content}/>
            </div>
        );
    }

    const imageLabel = filePath ? `${DRAWIO_IMAGE_ALT}: ${filePath}` : DRAWIO_IMAGE_ALT;

    return (
        <div className="md-preview-body file-view-drawio">
            {state.message ? <div className="file-view-drawio-warning">{state.message}</div> : null}
            <div className="file-view-drawio-svg-frame">
                <svg
                    className="file-view-drawio-svg"
                    viewBox={state.model.viewBox}
                    style={{aspectRatio: state.model.aspectRatio}}
                    role="img"
                    aria-label={imageLabel}
                >
                    <title>{imageLabel}</title>
                    <defs>
                        {state.model.edges.map((edge, index) => (
                            <marker
                                key={edge.id}
                                id={`${markerPrefix}-${index}`}
                                viewBox="0 0 10 10"
                                refX="9"
                                refY="5"
                                markerWidth="8"
                                markerHeight="8"
                                orient="auto-start-reverse"
                            >
                                <path d="M 0 0 L 10 5 L 0 10 z" fill={edge.stroke}/>
                            </marker>
                        ))}
                    </defs>
                    <g className="file-view-drawio-edges">
                        {state.model.edges.map((edge, index) => (
                            <DrawioEdgeView
                                key={edge.id}
                                edge={edge}
                                markerId={`${markerPrefix}-${index}`}
                                shapesById={shapesById}
                            />
                        ))}
                    </g>
                    <g className="file-view-drawio-shapes">
                        {state.model.shapes.map((shape) => (
                            <DrawioShapeView key={shape.id} shape={shape}/>
                        ))}
                    </g>
                </svg>
            </div>
        </div>
    );
}

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
        return <DrawioXmlPreview content={content} filePath={filePath}/>;
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
