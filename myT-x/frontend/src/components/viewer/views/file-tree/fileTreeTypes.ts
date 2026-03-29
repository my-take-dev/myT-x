/** Backend FileEntry returned by DevPanelListDir. */
export interface FileEntry {
    readonly name: string;
    readonly path: string;
    readonly is_dir: boolean;
    readonly size: number;
}

/** Backend FileContent returned by DevPanelReadFile. */
export interface FileContentResult {
    readonly path: string;
    readonly content: string;
    readonly line_count: number;
    readonly size: number;
    readonly truncated: boolean;
    readonly binary: boolean;
}

/**
 * @internal Base fields shared by FlatDirNode and FlatFileNode.
 * Not part of the public API — consumers should use FlatNode (discriminated union).
 */
interface FlatNodeBase {
    readonly path: string;
    readonly name: string;
    /** Zero-based indentation depth. Always >= 0 (enforced by flattenTree's depth parameter). */
    readonly depth: number;
}

/**
 * Flattened directory node for react-window virtualized rendering.
 * Note: directory size is intentionally omitted as it is not displayed in the tree view.
 */
export interface FlatDirNode extends FlatNodeBase {
    readonly isDir: true;
    readonly isExpanded: boolean;
    readonly isLoading: boolean;
}

/** Flattened file node for react-window virtualized rendering. */
export interface FlatFileNode extends FlatNodeBase {
    readonly isDir: false;
    readonly size: number;
}

export type FlatNode = FlatDirNode | FlatFileNode;

/** Backend SearchFileResult returned by DevPanelSearchFiles. */
export interface SearchFileResult {
    readonly path: string;
    readonly name: string;
    readonly is_name_match: boolean;
    readonly content_lines: readonly SearchContentLine[];
}

/** Backend SearchContentLine — a single matching line within a file. */
export interface SearchContentLine {
    readonly line: number;
    readonly content: string;
}
