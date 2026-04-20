import type {FileNode} from "./fileTreeTypes";
import type {DocumentKind} from "./documentTypes";

export const DOCUMENT_EXTENSIONS = new Set<string>([
    "db",
    "md",
    "mmd",
    "drawio",
    "sqlite",
    "sqlite3",
    "yaml",
    "yml",
    "json",
    "dot",
    "gv",
    "mm",
    "wavedrom",
    "vega",
]);

export const COMPOUND_SUFFIXES: readonly string[] = [
    ".drawio.svg",
    ".drawio.xml",
    ".vl.json",
    ".vg.json",
];

const DOCUMENT_KEY_SCAN_BYTES = 1024;
const YAML_OPENAPI_KEY_PATTERN = /(?:^|[\r\n])\s*(openapi|swagger)\s*:/m;

function normalizeDocumentName(name: string): string {
    return name.trim().toLowerCase();
}

function hasSupportedDocumentExtension(name: string): boolean {
    const normalizedName = normalizeDocumentName(name);
    if (normalizedName === "") {
        return false;
    }

    for (const suffix of COMPOUND_SUFFIXES) {
        if (normalizedName.endsWith(suffix)) {
            return true;
        }
    }

    const lastDotIndex = normalizedName.lastIndexOf(".");
    if (lastDotIndex < 0 || lastDotIndex === normalizedName.length - 1) {
        return false;
    }

    const extension = normalizedName.slice(lastDotIndex + 1);
    return DOCUMENT_EXTENSIONS.has(extension);
}

function detectDocumentKindByName(name: string): DocumentKind | null {
    const normalizedName = normalizeDocumentName(name);
    if (normalizedName.endsWith(".drawio.svg")) {
        return "drawio-svg";
    }
    if (normalizedName.endsWith(".drawio.xml") || normalizedName.endsWith(".drawio")) {
        return "drawio-xml";
    }
    if (normalizedName.endsWith(".vl.json")) {
        return "vega-lite";
    }
    if (normalizedName.endsWith(".vg.json")) {
        return "vega";
    }
    if (
        normalizedName.endsWith(".db")
        || normalizedName.endsWith(".sqlite")
        || normalizedName.endsWith(".sqlite3")
    ) {
        return "sqlite";
    }
    if (normalizedName.endsWith(".mmd")) {
        return "mermaid";
    }
    if (normalizedName.endsWith(".md")) {
        return "markdown";
    }
    if (normalizedName.endsWith(".dot") || normalizedName.endsWith(".gv")) {
        return "graphviz";
    }
    if (normalizedName.endsWith(".mm")) {
        return "markmap";
    }
    if (normalizedName.endsWith(".wavedrom")) {
        return "wavedrom";
    }
    if (normalizedName.endsWith(".vega")) {
        return "vega";
    }
    if (
        normalizedName.endsWith(".yaml")
        || normalizedName.endsWith(".yml")
        || normalizedName.endsWith(".json")
    ) {
        return "yaml-json-raw";
    }
    return null;
}

function trimLeadingByteOrderMark(content: string): string {
    return content.startsWith("\uFEFF") ? content.slice(1) : content;
}

function hasTopLevelSwaggerJSONKey(content: string): boolean {
    let depth = 0;
    let inString = false;
    let escaping = false;
    let currentString: string | null = null;
    let candidateKey: string | null = null;

    for (const character of content) {
        if (inString) {
            if (escaping) {
                escaping = false;
                if (currentString !== null) {
                    currentString += character;
                }
                continue;
            }
            if (character === "\\") {
                escaping = true;
                continue;
            }
            if (character === "\"") {
                inString = false;
                if (depth === 1) {
                    candidateKey = currentString;
                }
                currentString = null;
                continue;
            }
            if (currentString !== null) {
                currentString += character;
            }
            continue;
        }

        if (candidateKey !== null) {
            if (/\s/.test(character)) {
                continue;
            }
            if (character === ":") {
                if (candidateKey === "openapi" || candidateKey === "swagger") {
                    return true;
                }
                candidateKey = null;
                continue;
            }
            candidateKey = null;
        }

        switch (character) {
            case "\"":
                inString = true;
                currentString = depth === 1 ? "" : null;
                break;
            case "{":
            case "[":
                depth += 1;
                break;
            case "}":
            case "]":
                depth = Math.max(0, depth - 1);
                candidateKey = null;
                break;
            default:
                break;
        }
    }

    return false;
}

export function isDocumentFile(node: FileNode): boolean {
    if (node.isDir) {
        return false;
    }
    return hasSupportedDocumentExtension(node.name);
}

export function filterDocumentTree(
    nodes: readonly FileNode[],
): readonly FileNode[] {
    const filteredNodes: FileNode[] = [];

    for (const node of nodes) {
        if (!node.isDir) {
            if (isDocumentFile(node)) {
                filteredNodes.push(node);
            }
            continue;
        }

        if (!node.children) {
            // Lazy-loaded directories are preserved until their children are known.
            // Hiding them eagerly would make reachable document files undiscoverable.
            if (node.hasChildren) {
                filteredNodes.push(node);
            }
            continue;
        }

        const filteredChildren = filterDocumentTree(node.children);
        if (filteredChildren.length === 0) {
            continue;
        }

        filteredNodes.push({
            ...node,
            hasChildren: true,
            children: filteredChildren,
        });
    }

    return filteredNodes;
}

export function classifyDocument(
    name: string,
    rawContent: string,
): DocumentKind {
    const kind = detectDocumentKindByName(name);
    if (kind === null) {
        return "yaml-json-raw";
    }
    if (kind !== "yaml-json-raw") {
        return kind;
    }

    const normalizedName = normalizeDocumentName(name);
    const scannedContent = trimLeadingByteOrderMark(rawContent).slice(0, DOCUMENT_KEY_SCAN_BYTES);
    if (normalizedName.endsWith(".json")) {
        return hasTopLevelSwaggerJSONKey(scannedContent) ? "swagger" : "yaml-json-raw";
    }
    return YAML_OPENAPI_KEY_PATTERN.test(scannedContent) ? "swagger" : "yaml-json-raw";
}
