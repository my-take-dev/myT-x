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

const REVIEW_MARKDOWN_HEADINGS = {
    codeReviewComments: "# Code Review Comments",
    overallComment: "# Overall Comment",
} as const;

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

    const sections: string[] = [REVIEW_MARKDOWN_HEADINGS.codeReviewComments];

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

export interface DiffReviewSendMarkdownPayload {
    readonly message?: string;
    readonly comments?: readonly DiffReviewComment[];
}

export function buildReviewSendMarkdown(payload: DiffReviewSendMarkdownPayload): string {
    const message = (payload.message ?? "").trim();
    const comments = payload.comments ?? [];
    const reviewMarkdown = buildReviewMarkdown(comments);
    // Sent Markdown uses stable English headings because the receiver is an AI
    // review tool, not localized UI. User-authored Markdown is preserved as-is;
    // even if it starts with its own H1, it remains nested under this section by
    // position rather than rewritten.
    const overallCommentMarkdown = message === "" ? "" : `${REVIEW_MARKDOWN_HEADINGS.overallComment}\n\n${message}`;

    if (overallCommentMarkdown === "") {
        return reviewMarkdown;
    }
    if (reviewMarkdown === "") {
        return overallCommentMarkdown;
    }
    return `${overallCommentMarkdown}\n\n---\n\n${reviewMarkdown}`;
}
