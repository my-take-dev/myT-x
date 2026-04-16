export type DocumentKind =
    | "markdown"
    | "mermaid"
    | "drawio-svg"
    | "drawio-xml"
    | "swagger"
    | "sqlite"
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
        || kind === "sqlite";
}
