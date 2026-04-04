export function parentDirOf(path: string): string {
    const separatorIndex = path.lastIndexOf("/");
    return separatorIndex >= 0 ? path.slice(0, separatorIndex) : "";
}

export function joinRelativePath(parentDir: string, name: string): string {
    return parentDir ? `${parentDir}/${name}` : name;
}

export function toWindowsPath(path: string): string {
    return path.replaceAll("/", "\\");
}

export function buildAbsoluteEditorPath(rootPath: string, relativePath: string): string {
    const normalizedRoot = rootPath.trim().replace(/[\\/]+$/u, "");
    if (normalizedRoot === "") {
        return toWindowsPath(relativePath);
    }

    const normalizedRelative = relativePath.replaceAll("\\", "/").replace(/^\/+/u, "");
    if (normalizedRelative === "") {
        return toWindowsPath(normalizedRoot);
    }

    return toWindowsPath(`${normalizedRoot}/${normalizedRelative}`);
}
