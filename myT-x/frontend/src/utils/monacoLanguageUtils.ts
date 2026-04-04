export const LANGUAGE_MAP: Record<string, string> = {
    bat: "bat",
    c: "c",
    cc: "cpp",
    cmd: "bat",
    cpp: "cpp",
    cs: "csharp",
    css: "css",
    go: "go",
    h: "c",
    hpp: "cpp",
    html: "html",
    ini: "ini",
    java: "java",
    js: "javascript",
    json: "json",
    jsx: "javascript",
    kt: "kotlin",
    kts: "kotlin",
    less: "less",
    lua: "lua",
    md: "markdown",
    php: "php",
    ps1: "powershell",
    py: "python",
    rb: "ruby",
    rs: "rust",
    scss: "scss",
    sh: "shell",
    sql: "sql",
    toml: "toml",
    ts: "typescript",
    tsx: "typescript",
    txt: "plaintext",
    xml: "xml",
    yaml: "yaml",
    yml: "yaml",
};

export function getLanguageFromPath(path: string): string {
    const fileName = path.split("/").pop() ?? path;
    const parts = fileName.split(".");
    if (parts.length < 2) {
        return "plaintext";
    }

    const extension = parts[parts.length - 1]?.toLowerCase() ?? "";
    return LANGUAGE_MAP[extension] ?? "plaintext";
}
