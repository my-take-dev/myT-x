import {useEffect, useMemo, useRef, useState} from "react";
import type {PluggableList} from "unified";
import {api} from "../../../../api";
import {matchesCapturedSessionKey} from "../../../../utils/sessionGuard";
import {notifyAndLog} from "../../../../utils/notifyUtils";
import {createBinaryBlob} from "./binaryContentUtils";

interface UseRehypeResolveImagesOptions {
    readonly content: string;
    readonly filePath?: string;
    readonly sessionKey?: string;
    readonly sessionName?: string | null;
}

interface ResolvedImageEntry {
    readonly blobURL: string;
}

type ResolvedImageMap = Record<string, ResolvedImageEntry>;
type FailedImageState = {
    readonly attempts: number;
    readonly retryAfter: number;
    readonly lastNotifiedAt: number;
};

type HASTNode = {
    type?: unknown;
    tagName?: unknown;
    properties?: Record<string, unknown>;
    children?: unknown;
};

const LOCAL_IMAGE_RESOLVED_FLAG = "data-local-image-resolved";
const IMAGE_RESOLVE_RETRY_DELAY_MS = 750;
const MAX_IMAGE_RESOLVE_RETRY_DELAY_MS = 10_000;
const IMAGE_RESOLVE_NOTIFY_COOLDOWN_MS = 15_000;

function isObjectRecord(value: unknown): value is Record<string, unknown> {
    return typeof value === "object" && value !== null;
}

function isImageNode(node: unknown): node is HASTNode & { properties: Record<string, unknown> } {
    return isObjectRecord(node)
        && node.type === "element"
        && node.tagName === "img"
        && isObjectRecord(node.properties);
}

function visitNodes(node: unknown, visitor: (current: HASTNode) => void): void {
    if (!isObjectRecord(node)) {
        return;
    }

    visitor(node as HASTNode);

    const children = (node as HASTNode).children;
    if (!Array.isArray(children)) {
        return;
    }
    for (const child of children) {
        visitNodes(child, visitor);
    }
}

function normalizePanelPath(path: string): string {
    const normalizedInput = path.trim().replaceAll("\\", "/");
    if (normalizedInput === "") {
        return "";
    }

    const segments: string[] = [];
    for (const segment of normalizedInput.split("/")) {
        if (segment === "" || segment === ".") {
            continue;
        }
        if (segment === "..") {
            if (segments.length > 0 && segments[segments.length - 1] !== "..") {
                segments.pop();
            } else {
                segments.push(segment);
            }
            continue;
        }
        segments.push(segment);
    }

    return segments.join("/");
}

function parentPanelPath(path: string): string {
    const normalizedPath = normalizePanelPath(path);
    if (normalizedPath === "") {
        return "";
    }

    const segments = normalizedPath.split("/");
    segments.pop();
    return segments.join("/");
}

function isRelativeImageSource(source: string): boolean {
    const normalizedSource = source.trim();
    if (normalizedSource === "") {
        return false;
    }
    if (normalizedSource.startsWith("#")) {
        return false;
    }
    return !/^[a-zA-Z][a-zA-Z\d+\-.]*:/.test(normalizedSource);
}

function resolveImagePath(markdownPath: string, imageSource: string): string | null {
    const normalizedSource = imageSource.trim().replaceAll("\\", "/");
    if (!isRelativeImageSource(normalizedSource)) {
        return null;
    }

    if (normalizedSource.startsWith("/")) {
        return normalizePanelPath(normalizedSource.slice(1));
    }

    const baseDir = parentPanelPath(markdownPath);
    const joinedPath = baseDir === ""
        ? normalizedSource
        : `${baseDir}/${normalizedSource}`;
    const normalizedPath = normalizePanelPath(joinedPath);
    return normalizedPath === "" ? null : normalizedPath;
}

function revokeResolvedImages(images: ResolvedImageMap): void {
    for (const entry of Object.values(images)) {
        URL.revokeObjectURL(entry.blobURL);
    }
}

function clearRetryTimer(timerRef: { current: number | null }): void {
    if (timerRef.current === null) {
        return;
    }
    window.clearTimeout(timerRef.current);
    timerRef.current = null;
}

function getImageResolveRetryDelay(attempts: number): number {
    const retryStep = Math.max(0, attempts - 1);
    return Math.min(IMAGE_RESOLVE_RETRY_DELAY_MS * (2 ** retryStep), MAX_IMAGE_RESOLVE_RETRY_DELAY_MS);
}

export function useRehypeResolveImages({
    content,
    filePath,
    sessionKey,
    sessionName,
}: UseRehypeResolveImagesOptions): PluggableList | null {
    const [resolvedImages, setResolvedImages] = useState<ResolvedImageMap>({});
    const [retryNonce, setRetryNonce] = useState(0);
    const discoveredSourcesRef = useRef<ReadonlySet<string>>(new Set());
    const resolvedImagesRef = useRef<ResolvedImageMap>({});
    const latestSessionKeyRef = useRef(sessionKey ?? "");
    const failedSourcesRef = useRef<Map<string, FailedImageState>>(new Map());
    const retryTimerRef = useRef<number | null>(null);

    latestSessionKeyRef.current = sessionKey ?? "";

    useEffect(() => {
        resolvedImagesRef.current = resolvedImages;
    }, [resolvedImages]);

    useEffect(() => {
        clearRetryTimer(retryTimerRef);
        failedSourcesRef.current.clear();
        setResolvedImages((previous) => {
            if (Object.keys(previous).length === 0) {
                return previous;
            }
            revokeResolvedImages(previous);
            return {};
        });
    }, [content, filePath, sessionKey, sessionName]);

    useEffect(() => {
        return () => {
            clearRetryTimer(retryTimerRef);
            revokeResolvedImages(resolvedImagesRef.current);
        };
    }, []);

    useEffect(() => {
        const currentFilePath = filePath?.trim() ?? "";
        const currentSessionName = sessionName?.trim() ?? "";
        const currentSessionKey = sessionKey?.trim() ?? "";
        if (currentFilePath === "" || currentSessionName === "" || currentSessionKey === "") {
            return;
        }
        clearRetryTimer(retryTimerRef);

        const discoveredSources = [...discoveredSourcesRef.current];
        const discoveredSourceSet = new Set(discoveredSources);
        let nextRetryDelayMs: number | null = null;

        setResolvedImages((previous) => {
            let nextImages = previous;
            for (const [source, entry] of Object.entries(previous)) {
                if (discoveredSourceSet.has(source)) {
                    continue;
                }
                if (nextImages === previous) {
                    nextImages = {...previous};
                }
                URL.revokeObjectURL(entry.blobURL);
                delete nextImages[source];
                failedSourcesRef.current.delete(source);
            }
            return nextImages;
        });

        const pendingSources = discoveredSources.filter((source) => {
            if (source in resolvedImages) {
                return false;
            }
            const failedState = failedSourcesRef.current.get(source);
            if (failedState) {
                const retryDelayMs = failedState.retryAfter - Date.now();
                if (retryDelayMs > 0) {
                    nextRetryDelayMs = nextRetryDelayMs === null
                        ? retryDelayMs
                        : Math.min(nextRetryDelayMs, retryDelayMs);
                    return false;
                }
            }
            return resolveImagePath(currentFilePath, source) !== null;
        });
        if (pendingSources.length === 0) {
            if (nextRetryDelayMs !== null) {
                retryTimerRef.current = window.setTimeout(() => {
                    retryTimerRef.current = null;
                    setRetryNonce((previous) => previous + 1);
                }, nextRetryDelayMs);
            }
            return;
        }

        const capturedSessionKey = currentSessionKey;
        let disposed = false;

        void (async () => {
            const nextImages: Record<string, ResolvedImageEntry> = {};
            const failedMessages: string[] = [];
            let nextScheduledRetryDelayMs: number | null = nextRetryDelayMs;
            const releasePendingImages = () => {
                if (Object.keys(nextImages).length === 0) {
                    return;
                }
                revokeResolvedImages(nextImages);
            };
            const shouldAbort = () => disposed || !matchesCapturedSessionKey(capturedSessionKey, latestSessionKeyRef.current);

            for (const source of pendingSources) {
                const resolvedPath = resolveImagePath(currentFilePath, source);
                if (resolvedPath === null) {
                    continue;
                }

                try {
                    const binaryContent = await api.DevPanelReadBinary(currentSessionName, resolvedPath);
                    if (shouldAbort()) {
                        releasePendingImages();
                        return;
                    }

                    nextImages[source] = {
                        blobURL: URL.createObjectURL(createBinaryBlob(binaryContent)),
                    };
                    failedSourcesRef.current.delete(source);
                } catch (err: unknown) {
                    if (shouldAbort()) {
                        releasePendingImages();
                        return;
                    }
                    const previousFailure = failedSourcesRef.current.get(source);
                    const now = Date.now();
                    const attempts = (previousFailure?.attempts ?? 0) + 1;
                    const retryDelayMs = getImageResolveRetryDelay(attempts);
                    const lastNotifiedAt = previousFailure?.lastNotifiedAt ?? 0;
                    const shouldNotify = lastNotifiedAt === 0 || now - lastNotifiedAt >= IMAGE_RESOLVE_NOTIFY_COOLDOWN_MS;
                    const nextFailure: FailedImageState = {
                        attempts,
                        retryAfter: now + retryDelayMs,
                        lastNotifiedAt: shouldNotify ? now : lastNotifiedAt,
                    };
                    failedSourcesRef.current.set(source, nextFailure);
                    nextScheduledRetryDelayMs = nextScheduledRetryDelayMs === null
                        ? retryDelayMs
                        : Math.min(nextScheduledRetryDelayMs, retryDelayMs);
                    if (shouldNotify) {
                        failedMessages.push(`${source}: ${err instanceof Error ? err.message : String(err)}`);
                        console.warn("[markdown] failed to resolve local image", {
                            filePath: currentFilePath,
                            path: resolvedPath,
                            session: currentSessionName,
                            err,
                        });
                    }
                }
            }

            if (shouldAbort()) {
                releasePendingImages();
                return;
            }

            if (Object.keys(nextImages).length > 0) {
                setResolvedImages((previous) => ({
                    ...previous,
                    ...nextImages,
                }));
            }
            if (failedMessages.length > 0) {
                notifyAndLog(
                    "Load markdown image",
                    "warn",
                    new Error(failedMessages.join("; ")),
                    "MarkdownRenderer",
                );
            }
            if (nextScheduledRetryDelayMs !== null) {
                retryTimerRef.current = window.setTimeout(() => {
                    retryTimerRef.current = null;
                    setRetryNonce((previous) => previous + 1);
                }, nextScheduledRetryDelayMs);
            }
        })();

        return () => {
            disposed = true;
        };
    }, [content, filePath, resolvedImages, retryNonce, sessionKey, sessionName]);

    return useMemo<PluggableList | null>(() => {
        const currentFilePath = filePath?.trim() ?? "";
        const currentSessionName = sessionName?.trim() ?? "";
        const currentSessionKey = sessionKey?.trim() ?? "";
        if (currentFilePath === "" || currentSessionName === "" || currentSessionKey === "") {
            discoveredSourcesRef.current = new Set();
            return null;
        }

        return [() => (tree: unknown) => {
            const discoveredSources = new Set<string>();
            visitNodes(tree, (node) => {
                if (!isImageNode(node)) {
                    return;
                }

                const source = typeof node.properties.src === "string" ? node.properties.src : "";
                if (!isRelativeImageSource(source)) {
                    return;
                }

                discoveredSources.add(source);
                const resolvedImage = resolvedImages[source];
                if (!resolvedImage) {
                    return;
                }

                node.properties = {
                    ...node.properties,
                    src: resolvedImage.blobURL,
                    [LOCAL_IMAGE_RESOLVED_FLAG]: "true",
                };
            });
            discoveredSourcesRef.current = discoveredSources;
        }];
    }, [content, filePath, resolvedImages, sessionKey, sessionName]);
}
