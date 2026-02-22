/** Backend FileEntry returned by DevPanelListDir. */
export interface FileEntry {
  name: string;
  path: string;
  is_dir: boolean;
  size: number;
}

/** Backend FileContent returned by DevPanelReadFile. */
export interface FileContentResult {
  path: string;
  content: string;
  line_count: number;
  size: number;
  truncated: boolean;
  binary: boolean;
}

/** Flattened node for react-window virtualized rendering. */
export interface FlatNode {
  path: string;
  name: string;
  isDir: boolean;
  depth: number;
  isExpanded: boolean;
  isLoading: boolean;
  size: number;
}
