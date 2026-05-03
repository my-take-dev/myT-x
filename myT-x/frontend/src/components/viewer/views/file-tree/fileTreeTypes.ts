import type {devpanel} from "../../../../../wailsjs/go/models";

/** Backend FileEntry returned by DevPanelListDir. */
export type FileEntry = devpanel.FileEntry;

/** Backend FileContent returned by DevPanelReadFile. */
export type FileContentResult = devpanel.FileContent;

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

/** Hierarchical file-tree node used by frontend state. */
export interface FileNode {
    readonly name: string;
    readonly path: string;
    readonly isDir: boolean;
    readonly hasChildren: boolean;
    /** Files: supported document target. Directories: may contain a supported descendant. */
    readonly hasViewTarget: boolean;
    readonly children?: readonly FileNode[];
    readonly size?: number;
}

/**
 * Flattened directory node for react-window virtualized rendering.
 * Note: directory size is intentionally omitted as it is not displayed in the tree view.
 */
export interface FlatDirNode extends FlatNodeBase {
    readonly isDir: true;
    readonly hasChildren: boolean;
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
