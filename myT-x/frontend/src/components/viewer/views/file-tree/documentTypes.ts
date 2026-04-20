export type DocumentKind =
    | "markdown"
    | "mermaid"
    | "drawio-svg"
    | "drawio-xml"
    | "swagger"
    | "sqlite"
    | "graphviz"
    | "markmap"
    | "wavedrom"
    | "vega-lite"
    | "vega"
    | "yaml-json-raw";

export type RenderMode = "raw" | "preview";

export interface BinaryFileContent {
    readonly path: string;
    readonly data: string;
    readonly mime: string;
}

export function canPreviewDocumentKind(kind: DocumentKind | null): boolean {
    return kind === "markdown"
        || kind === "mermaid"
        || kind === "swagger"
        || kind === "drawio-svg"
        || kind === "drawio-xml"
        || kind === "sqlite"
        || kind === "graphviz"
        || kind === "markmap"
        || kind === "wavedrom"
        || kind === "vega-lite"
        || kind === "vega";
}

export function canPreviewBinaryDocumentKind(kind: DocumentKind | null): boolean {
    return kind === "sqlite";
}

export function getDefaultRenderModeForDocumentKind(kind: DocumentKind | null): RenderMode {
    return canPreviewDocumentKind(kind) ? "preview" : "raw";
}

export function getUncontrolledDefaultRenderModeForDocumentKind(kind: DocumentKind | null): RenderMode {
    return canPreviewBinaryDocumentKind(kind) ? "preview" : "raw";
}
