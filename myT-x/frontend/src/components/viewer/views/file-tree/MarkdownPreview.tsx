import {isValidElement, memo, useMemo, type MouseEvent, type ReactNode} from "react";
import {getLanguage, translate} from "../../../../i18n";
import {sanitizeCssColor} from "../../../../utils/cssUtils";
import {notifyLinkOpenFailure} from "../../../../utils/notifyUtils";
import {sanitizeHref, SCHEME_PATTERN} from "../../../../utils/sanitizeHref";
import ReactMarkdown, {defaultUrlTransform} from "react-markdown";
import remarkGfm from "remark-gfm";
import type {PluggableList} from "unified";
import type {Components} from "react-markdown";
import {useShikiHighlight} from "../../../../hooks/useShikiHighlight";
import {BrowserOpenURL} from "../../../../../wailsjs/runtime";

interface MarkdownPreviewProps {
    readonly content: string;
    readonly rehypePlugins?: PluggableList;
    readonly codeBlockRenderer?: (props: MarkdownCodeBlockRendererProps) => ReactNode;
}

const MARKDOWN_REMARK_PLUGINS: PluggableList = [remarkGfm];
const EMPTY_REHYPE_PLUGINS: PluggableList = [];

function translateViewerText(key: string, jaText: string, enText: string): string {
    return translate(key, getLanguage() === "ja" ? jaText : enText);
}

// ── Shiki-highlighted code block for fenced code ──

function logExternalLinkOpenError(href: string, err: unknown): void {
    notifyLinkOpenFailure();
    console.warn("[markdown] failed to open external link", href, err);
}

function openExternalLink(href: string): void {
    try {
        BrowserOpenURL(href);
    } catch (err: unknown) {
        logExternalLinkOpenError(href, err);
    }
}

// Maximum recursion depth for extractTextContent (depth starts at 0, so max traversed depth is 49).
// react-markdown output is typically shallow, but this guards against
// pathologically nested markdown structures.
const EXTRACT_TEXT_MAX_DEPTH = 50;

// Recursively flatten markdown code children into plain text.
// Handles nested fragments/spans produced by remark/rehype plugins.
function extractTextContent(node: ReactNode, depth: number = 0): string {
    if (depth >= EXTRACT_TEXT_MAX_DEPTH) {
        // In production, silently returns "" (truncated text).
        // Depth 50 is well above typical markdown nesting (rarely exceeds 10 levels).
        // If this limit is reached, the Shiki code block will receive truncated source,
        // resulting in partial highlighting — acceptable degradation for pathological input.
        console.warn("[MarkdownPreview] extractTextContent: recursion depth limit reached, text may be truncated");
        return "";
    }
    if (typeof node === "string" || typeof node === "number") {
        return String(node);
    }
    if (typeof node === "boolean" || node == null) {
        return "";
    }
    if (Array.isArray(node)) {
        return node.map((item) => extractTextContent(item, depth + 1)).join("");
    }
    if (isValidElement<{ children?: ReactNode }>(node)) {
        return extractTextContent(node.props.children, depth + 1);
    }
    return "";
}

interface ShikiCodeBlockProps {
    className?: string;
    children?: ReactNode;
    code?: string;
}

const ShikiCodeBlock = memo(function ShikiCodeBlock({className, children, code: providedCode}: ShikiCodeBlockProps) {
    // Extract language from className (e.g., "language-typescript" -> "typescript").
    const lang = className?.match(/\blanguage-(\S+)/)?.[1] ?? "";
    const code = providedCode ?? extractTextContent(children).replace(/\r?\n$/, "");

    // Reuse the shared hook instead of reimplementing the async highlight lifecycle.
    // Pass lang directly (3rd arg) since we already have the language ID from the CSS class.
    const {tokens} = useShikiHighlight(code || undefined, undefined, lang || undefined);

    if (tokens) {
        return (
            <code className={className}>
                {tokens.map((line, lineIndex) => (
                    <span key={`line-${lineIndex}`}>
                        {line.map((token, tokenIndex) => (
                            <span key={`token-${lineIndex}-${tokenIndex}`}
                                  style={{color: sanitizeCssColor(token.color)}}>
                                {token.content}
                            </span>
                        ))}
                        {lineIndex < tokens.length - 1 ? "\n" : null}
                    </span>
                ))}
            </code>
        );
    }

    // Fallback: plain text while Shiki loads.
    return <code className={className}>{children}</code>;
});

// ── Custom components for react-markdown ──

export interface MarkdownCodeBlockRendererProps {
    readonly className?: string;
    readonly code: string;
}

function createMarkdownComponents(
    codeBlockRenderer?: (props: MarkdownCodeBlockRendererProps) => ReactNode,
): Components {
    return {
        a({href, children, ...props}) {
            const {node: _node, rel: _rel, target: _target, ...domProps} = props as typeof props & {
                node?: unknown;
                rel?: string;
                target?: string;
            };

            const safeHref = sanitizeHref(href);
            if (!safeHref) {
                return <span className="md-preview-unsafe-link">{children}</span>;
            }

            // Relative paths (./foo, ../bar, #anchor) have no meaningful target
            // in a file preview context. Suppress default navigation.
            const isAnchor = safeHref.startsWith("#");
            const isRelative = safeHref.startsWith("/") || safeHref.startsWith("./") || safeHref.startsWith("../");
            const isInternalNav = isAnchor || isRelative;
            const isExternal = SCHEME_PATTERN.test(safeHref);
            const handleLinkClick = (event: MouseEvent<HTMLAnchorElement>) => {
                if (isInternalNav) {
                    event.preventDefault();
                    // DEV-only: internal-nav suppression is routine and only useful for debugging.
                    // External-link failures (logExternalLinkOpenError) are always logged because
                    // they represent user-visible action failures.
                    if (import.meta.env.DEV) {
                        console.warn("[DEBUG-markdown] Internal link navigation suppressed:", safeHref);
                    }
                    return;
                }
                if (isExternal) {
                    event.preventDefault();
                    openExternalLink(safeHref);
                    return;
                }
                // Scheme-less URLs (e.g., "foo/bar") that are neither internal nav nor external.
                // Prevent default navigation to avoid unintended Wails webview navigation.
                event.preventDefault();
                if (import.meta.env.DEV) {
                    console.warn("[DEBUG-markdown] Scheme-less link navigation suppressed:", safeHref);
                }
            };

            // #111: Provide visual feedback for links that cannot be navigated in the desktop app,
            // instead of silently swallowing the click with no indication.
            const disabledLinkTitle = isInternalNav
                ? translateViewerText(
                    "viewer.markdown.link.desktopBlocked",
                    "このリンクはデスクトップアプリ内では開けません (This link cannot be opened in the desktop app)",
                    "This link cannot be opened in the desktop app",
                )
                : isExternal
                    ? undefined
                    : translateViewerText(
                        "viewer.markdown.link.unsupportedFormat",
                        "このリンク形式はプレビュー内では開けません (Unsupported link format in preview)",
                        "Unsupported link format in preview",
                    );

            // data-original-href: preserves the original href for debugging and potential future
            // features (e.g., file-tree navigation). Not referenced by CSS or JS currently.
            return (
                <a
                    href={isExternal ? safeHref : undefined}
                    {...domProps}
                    onClick={handleLinkClick}
                    rel={isExternal ? "noopener noreferrer nofollow" : undefined}
                    target={isExternal ? "_blank" : undefined}
                    data-original-href={isInternalNav ? safeHref : undefined}
                    aria-label={
                        isInternalNav
                            ? `Blocked in-app link: ${safeHref}`
                            : isExternal
                                ? `External link: ${safeHref}`
                                : `Blocked unsupported link: ${safeHref}`
                    }
                    title={disabledLinkTitle}
                    className={isInternalNav ? "md-preview-disabled-link" : undefined}
                >
                    {children}
                </a>
            );
        },
        img({src, alt, ...props}) {
            const trustedBlobSrc = typeof src === "string" && src.startsWith("blob:")
                ? src
                : null;
            const {node: _node, ...domProps} = props as typeof props & { node?: unknown };
            const safeSrc = trustedBlobSrc ?? sanitizeHref(src);
            const hasExplicitScheme = safeSrc ? SCHEME_PATTERN.test(safeSrc) : false;
            const isResolvedLocalBlob = safeSrc?.startsWith("blob:") === true;
            // SECURITY: Only local/relative image paths are allowed. URI schemes
            // (http:, https:, file:, data:, ftp:, ws:, etc.; RFC 3986 forms) are
            // blocked to prevent network requests from markdown content.
            // blob: URLs are also allowed because they are origin-local object URLs
            // rather than network fetches, and the file-view image pipeline relies
            // on them for session-local image previews.
            // Relative paths (./foo, ../bar)
            // resolve within Wails WebView2's app asset boundary.
            // Fragment-only URLs (#anchor) make no sense as image sources and are blocked.
            // While sanitizeHref passes them through for regular links, images need an actual path.
            if (!safeSrc || (hasExplicitScheme && !isResolvedLocalBlob) || safeSrc.startsWith("#")) {
                return <span className="md-preview-unsafe-img" role="img" aria-label="Image blocked for security">[image blocked]</span>;
            }
            return <img src={safeSrc} alt={alt ?? ""} {...domProps}/>;
        },
        code({className, children, ...props}) {
            const {node: _node, inline: _inline, ...domProps} = props as typeof props & {
                node?: unknown;
                inline?: unknown;
            };
            // Block code (fenced with language) -> Shiki highlighting.
            const isBlock = className?.startsWith("language-");
            if (isBlock) {
                const code = extractTextContent(children).replace(/\r?\n$/, "");
                const customBlock = codeBlockRenderer?.({className, code});
                if (customBlock) {
                    return customBlock;
                }
                return <ShikiCodeBlock className={className} code={code}>{children}</ShikiCodeBlock>;
            }
            // Inline code -> plain rendering.
            return <code className={className} {...domProps}>{children}</code>;
        },
        pre({children, ...props}) {
            const {node: _node, ...domProps} = props as typeof props & { node?: unknown };
            return <pre className="md-preview-code-block" {...domProps}>{children}</pre>;
        },
    };
}

/**
 * Renders markdown content with GFM support and Shiki-highlighted code blocks.
 * Memoized to prevent unnecessary re-renders when parent state changes.
 */
export const MarkdownPreview = memo(function MarkdownPreview({
    content,
    rehypePlugins = EMPTY_REHYPE_PLUGINS,
    codeBlockRenderer,
}: MarkdownPreviewProps) {
    const markdownComponents = useMemo(
        () => createMarkdownComponents(codeBlockRenderer),
        [codeBlockRenderer],
    );
    const markdownUrlTransform = (url: string) => (
        url.startsWith("blob:") ? url : defaultUrlTransform(url)
    );

    return (
        <div className="md-preview-body">
            <ReactMarkdown
                remarkPlugins={MARKDOWN_REMARK_PLUGINS}
                rehypePlugins={rehypePlugins}
                components={markdownComponents}
                urlTransform={markdownUrlTransform}
            >
                {content}
            </ReactMarkdown>
        </div>
    );
});
