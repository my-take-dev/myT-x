import type {DiffReviewComment} from "../../../../stores/diffReviewStore";
import {formatDiffReviewRangeLabel} from "./diffReviewRange";

const extensionLanguageMap: Record<string, string> = {
    cjs: "js",
    cpp: "cpp",
    css: "css",
    go: "go",
    htm: "html",
    html: "html",
    java: "java",
    js: "js",
    jsx: "jsx",
    json: "json",
    md: "md",
    mjs: "js",
    py: "python",
    rs: "rust",
    sh: "bash",
    sql: "sql",
    ts: "ts",
    tsx: "tsx",
    xml: "xml",
    yaml: "yaml",
    yml: "yaml",
};

function pickFence(content: string): string {
    const matches = content.match(/`{3,}/g);
    const longestMatch = matches == null ? 2 : Math.max(...matches.map((match) => match.length));
    return "`".repeat(Math.max(3, longestMatch + 1));
}

function pickFenceLanguage(filePath: string): string {
    const dotIndex = filePath.lastIndexOf(".");
    if (dotIndex < 0 || dotIndex === filePath.length - 1) {
        return "";
    }
    return extensionLanguageMap[filePath.slice(dotIndex + 1).toLowerCase()] ?? "";
}

export function buildReviewMarkdown(comments: readonly DiffReviewComment[]): string {
    if (comments.length === 0) return "";

    const byFile = new Map<string, DiffReviewComment[]>();
    for (const c of comments) {
        const fileGroupKey = `${c.oldFilePath ?? ""}\u0000${c.filePath}`;
        const group = byFile.get(fileGroupKey) ?? [];
        group.push(c);
        byFile.set(fileGroupKey, group);
    }

    const sections: string[] = ["# Code Review Comments"];

    for (const [_, fileComments] of byFile) {
        const firstComment = fileComments[0];
        if (firstComment == null) {
            continue;
        }
        const fileLabel =
            firstComment.oldFilePath && firstComment.oldFilePath !== firstComment.filePath
                ? `${firstComment.oldFilePath} -> ${firstComment.filePath}`
                : firstComment.filePath;
        for (const c of fileComments) {
            const lineLabel = formatDiffReviewRangeLabel(
                {lineNum: c.startLineNum, lineType: c.startLineType},
                {lineNum: c.endLineNum, lineType: c.endLineType},
            );
            const fence = pickFence(c.lineContent);
            const fenceLanguage = pickFenceLanguage(c.filePath);
            const blockquote = c.commentText
                .split("\n")
                .map((line) => `> ${line}`)
                .join("\n");
            sections.push(
                [`## \`${fileLabel}\` (${lineLabel})`, `${fence}${fenceLanguage}`, c.lineContent, fence, blockquote].join("\n"),
            );
        }
    }

    return sections.join("\n\n---\n\n");
}
