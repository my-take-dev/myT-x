import {lazy, memo, Suspense, useCallback} from "react";
import type {PluggableList} from "unified";
import {MarkdownPreview, type MarkdownCodeBlockRendererProps} from "../MarkdownPreview";
import {useRehypeResolveImages} from "../rehypeResolveImages";

interface MarkdownRendererProps {
    readonly content: string;
    readonly filePath?: string;
    readonly sessionKey?: string;
    readonly sessionName?: string | null;
}

const EMPTY_REHYPE_PLUGINS: PluggableList = [];
const LazyMermaidRenderer = lazy(async () => ({
    default: (await import("./MermaidRenderer")).MermaidRenderer,
}));

function isMermaidCodeBlock(className?: string): boolean {
    return className?.toLowerCase().split(/\s+/).includes("language-mermaid") ?? false;
}

export const MarkdownRenderer = memo(function MarkdownRenderer({
    content,
    filePath,
    sessionKey,
    sessionName,
}: MarkdownRendererProps) {
    const rehypePlugins = useRehypeResolveImages({
        content,
        filePath,
        sessionKey,
        sessionName,
    });

    const codeBlockRenderer = useCallback(({className, code}: MarkdownCodeBlockRendererProps) => {
        if (!isMermaidCodeBlock(className)) {
            return null;
        }

        return (
            <Suspense fallback={<pre className="md-preview-code-block"><code className={className}>{code}</code></pre>}>
                <LazyMermaidRenderer code={code}/>
            </Suspense>
        );
    }, []);

    return (
        <MarkdownPreview
            content={content}
            rehypePlugins={rehypePlugins ?? EMPTY_REHYPE_PLUGINS}
            codeBlockRenderer={codeBlockRenderer}
        />
    );
});
