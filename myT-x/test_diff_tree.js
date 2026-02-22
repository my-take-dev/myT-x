function buildDiffTree(files, expandedDirs) {
    const sortedFiles = [...files].sort((a, b) => a.path.localeCompare(b.path));
    const nodes = [];
    const addedDirs = new Set();

    for (const file of sortedFiles) {
        const parts = file.path.split("/");
        for (let i = 1; i < parts.length; i++) {
            const dirPath = parts.slice(0, i).join("/");
            if (addedDirs.has(dirPath)) continue;

            addedDirs.add(dirPath);

            const parentPath = parts.slice(0, i - 1).join("/");
            if (i > 1 && !expandedDirs.has(parentPath)) continue;

            nodes.push({
                name: parts[i - 1],
                path: dirPath,
                isDir: true,
                depth: i - 1,
                isExpanded: expandedDirs.has(dirPath),
            });
        }

        const parentDir = parts.length > 1 ? parts.slice(0, -1).join("/") : "";
        if (parentDir === "" || expandedDirs.has(parentDir)) {
            nodes.push({
                name: parts[parts.length - 1],
                path: file.path,
                isDir: false,
                depth: parts.length - 1,
                isExpanded: false,
                file,
            });
        }
    }
    return nodes;
}

const files = [
    { path: "fileA" },
    { path: "folderB/fileC" },
    { path: "folderB/subFolder/fileD" }
];
const expandedDirs = new Set(["folderB", "folderB/subFolder"]);
const nodes = buildDiffTree(files, expandedDirs);
console.log(JSON.stringify(nodes, null, 2));
