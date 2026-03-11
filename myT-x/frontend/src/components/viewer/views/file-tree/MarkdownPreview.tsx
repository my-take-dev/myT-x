import {isValidElement, memo, type MouseEvent, type ReactNode} from "react";
import {getLanguage, translate} from "../../../../i18n";
import {sanitizeCssColor} from "../../../../utils/cssUtils";
import {notifyLinkOpenFailure} from "../../../../utils/notifyUtils";
import {sanitizeHref, SCHEME_PATTERN} from "../../../../utils/sanitizeHref";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import type {PluggableList} from "unified";
import type {Components} from "react-markdown";
import {useShikiHighlight} from "../../../../hooks/useShikiHighlight";
import {BrowserOpenURL} from "../../../../../wailsjs/runtime/runtime";

interface MarkdownPreviewProps {
    content: string;
}

const MARKDOWN_REMARK_PLUGINS: PluggableList = [remarkGfm];

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
        void Promise.resolve(BrowserOpenURL(href)).catch((err: unknown) => {
            logExternalLinkOpenError(href, err);
        });
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
}

const ShikiCodeBlock = memo(function ShikiCodeBlock({className, children}: ShikiCodeBlockProps) {
    // Extract language from className (e.g., "language-typescript" -> "typescript").
    const lang = className?.match(/\blanguage-(\S+)/)?.[1] ?? "";
    const code = extractTextContent(children).replace(/\r?\n$/, "");

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

// Module-level constant: all MarkdownPreview instances share the same component
// definitions. This is safe because the components are stateless (no props-dependent
// closures). If future components need access to instance props (e.g., file path for
// navigation), move this inside the component body wrapped in useMemo.
const markdownComponents: Components = {
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

        const shouldSetHref = isExternal;

        // data-original-href: preserves the original href for debugging and potential future
        // features (e.g., file-tree navigation). Not referenced by CSS or JS currently.
        return (
            <a
                href={shouldSetHref ? safeHref : undefined}
                {...domProps}
                onClick={handleLinkClick}
                rel={isExternal ? "noopener noreferrer nofollow" : undefined}
                target={isExternal ? "_blank" : undefined}
                data-original-href={isInternalNav ? safeHref : undefined}
                aria-label={
                    isInternalNav
                        ? `File link: ${safeHref}`
                        : isExternal
                            ? `External link: ${safeHref}`
                            : `Unsupported link: ${safeHref}`
                }
                title={disabledLinkTitle}
                className={isInternalNav ? "md-preview-disabled-link" : undefined}
            >
                {children}
            </a>
        );
    },
    img({src, alt, ...props}) {
        const {node: _node, ...domProps} = props as typeof props & { node?: unknown };
        const safeSrc = sanitizeHref(src);
        const hasExplicitScheme = safeSrc ? SCHEME_PATTERN.test(safeSrc) : false;
        // SECURITY: Only local/relative image paths are allowed. Any URI scheme
        // (http:, https:, file:, data:, ftp:, blob:, ws:, etc.; RFC 3986 forms)
        // is blocked by SCHEME_PATTERN to prevent network requests from markdown content.
        // Relative paths (./foo, ../bar)
        // resolve within Wails WebView2's app asset boundary.
        // Fragment-only URLs (#anchor) make no sense as image sources and are blocked.
        // While sanitizeHref passes them through for regular links, images need an actual path.
        if (!safeSrc || hasExplicitScheme || safeSrc.startsWith("#")) {
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
            return <ShikiCodeBlock className={className}>{children}</ShikiCodeBlock>;
        }
        // Inline code -> plain rendering.
        return <code className={className} {...domProps}>{children}</code>;
    },
    pre({children, ...props}) {
        const {node: _node, ...domProps} = props as typeof props & { node?: unknown };
        return <pre className="md-preview-code-block" {...domProps}>{children}</pre>;
    },
};

/**
 * Renders markdown content with GFM support and Shiki-highlighted code blocks.
 * Memoized to prevent unnecessary re-renders when parent state changes.
 */
export const MarkdownPreview = memo(function MarkdownPreview({content}: MarkdownPreviewProps) {
    return (
        <div className="md-preview-body">
            <ReactMarkdown remarkPlugins={MARKDOWN_REMARK_PLUGINS} components={markdownComponents}>
                {content}
            </ReactMarkdown>
        </div>
    );
});
