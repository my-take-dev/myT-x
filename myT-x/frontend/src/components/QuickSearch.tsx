import {
    type CSSProperties,
    type KeyboardEvent as ReactKeyboardEvent,
    useCallback,
    useEffect,
    useId,
    useMemo,
    useRef,
    useState,
} from "react";
import {api} from "../api";
import {useI18n} from "../i18n";
import {useTmuxStore} from "../stores/tmuxStore";
import type {AppConfigMessageTemplate, SessionSnapshot} from "../types/tmux";
import {toErrorMessage} from "../utils/errorUtils";
import {isImeTransitionalEvent} from "../utils/ime";
import {logFrontendEventSafe} from "../utils/logFrontendEventSafe";
import {notifyOperationFailure} from "../utils/notifyUtils";
import {resolveActivePaneID} from "../utils/session";
import {QUICK_SEARCH_DIALOG_ID, type QuickSearchTriggerMode} from "./quickSearchShared";
import {getViewerShortcutValue} from "./viewer/viewerShortcutDefinitions";
import {useRegisteredViews} from "./viewer/useRegisteredViews";
import {analyzeViewerShortcuts} from "./viewer/viewerShortcutAnalysis";
import {formatShortcutForDisplay, getEffectiveViewerShortcut} from "./viewer/viewerShortcutUtils";
import type {OpenViewWithContext, TaskSchedulerTemplateViewContext} from "./viewer/viewerContext";
import {useViewerStore} from "./viewer/viewerStore";

interface QuickSearchProps {
    open: boolean;
    onClose: () => void;
    onOpenNewSession: () => void;
    onOpenSettings: () => void;
    triggerMode?: QuickSearchTriggerMode;
    dropdownAnchorRef?: { current: HTMLElement | null };
}

interface SearchEntry {
    id: string;
    label: string;
    groupLabel: string;
    meta: string[];
    searchTargets: string[];
    rank: number;
    operationLabel: string;
    logLabel: string;
    execute: () => Promise<void> | void;
}

interface SearchResult extends SearchEntry {
    score: number;
}

const EMPTY_TASK_TEMPLATES = Object.freeze([] as AppConfigMessageTemplate[]);
const ENTRY_RANK_SESSION = 0;
const ENTRY_RANK_COMMAND = 1;
const ENTRY_RANK_VIEWER = 2;
const ENTRY_RANK_TEMPLATE = 3;
const DROPDOWN_EDGE_PADDING = 12;
const DROPDOWN_GAP = 8;
const DROPDOWN_MAX_HEIGHT = 420;
const DROPDOWN_MAX_WIDTH = 480;
const DROPDOWN_MIN_HEIGHT = 220;
const DROPDOWN_MIN_WIDTH = 320;

function fuzzyScore(query: string, target: string): number {
    if (!query || !target) return 0;
    const normalizedQuery = query.toLowerCase();
    const normalizedTarget = target.toLowerCase();
    if (normalizedTarget === normalizedQuery) return 100;
    if (normalizedTarget.startsWith(normalizedQuery)) return 80;
    if (normalizedTarget.includes(normalizedQuery)) return 60;
    let queryIndex = 0;
    for (let targetIndex = 0; targetIndex < normalizedTarget.length && queryIndex < normalizedQuery.length; targetIndex++) {
        if (normalizedTarget[targetIndex] === normalizedQuery[queryIndex]) {
            queryIndex++;
        }
    }
    if (queryIndex === normalizedQuery.length) return 40;
    return 0;
}

function scoreEntry(query: string, entry: SearchEntry): number {
    if (query === "") {
        return 1;
    }
    return entry.searchTargets.reduce((best, target) => Math.max(best, fuzzyScore(query, target)), 0);
}

function sortResults(left: SearchResult, right: SearchResult, hasQuery: boolean): number {
    if (hasQuery && right.score !== left.score) {
        return right.score - left.score;
    }
    if (left.rank !== right.rank) {
        return left.rank - right.rank;
    }
    return left.label.localeCompare(right.label);
}

function extractRepoName(session: SessionSnapshot): string {
    return session.worktree?.repo_path?.split(/[\\/]/).filter(Boolean).pop() ?? "";
}

export function QuickSearch({
    open,
    onClose,
    onOpenNewSession,
    onOpenSettings,
    triggerMode = "palette",
    dropdownAnchorRef,
}: QuickSearchProps) {
    const {language, t} = useI18n();
    const tr = useCallback(
        (key: string, jaText: string, enText: string, vars?: Record<string, string | number>) =>
            t(key, language === "ja" ? jaText : enText, vars),
        [language, t],
    );
    const sessions = useTmuxStore((s) => s.sessions);
    const activeSession = useTmuxStore((s) => s.activeSession);
    const config = useTmuxStore((s) => s.config);
    const setActiveSession = useTmuxStore((s) => s.setActiveSession);
    const openView = useViewerStore((s) => s.openView);
    const openViewWithContext = useViewerStore((s) => s.openViewWithContext);
    const [query, setQuery] = useState("");
    const [selectedIndex, setSelectedIndex] = useState(0);
    const [executing, setExecuting] = useState(false);
    const executingRef = useRef(false);
    const mountedRef = useRef(true);
    const panelRef = useRef<HTMLDivElement>(null);
    const templateLaunchTokenRef = useRef(0);
    const inputRef = useRef<HTMLInputElement>(null);
    const resultItemRefs = useRef<Map<string, HTMLDivElement> | null>(null);
    const getResultItemRefs = useCallback((): Map<string, HTMLDivElement> => {
        if (resultItemRefs.current === null) {
            resultItemRefs.current = new Map<string, HTMLDivElement>();
        }
        return resultItemRefs.current;
    }, []);
    const listboxId = useId();
    const views = useRegisteredViews();
    const [dropdownPanelStyle, setDropdownPanelStyle] = useState<CSSProperties | undefined>(undefined);
    const isDropdown = triggerMode === "dropdown";

    const currentSession = useMemo(
        () => sessions.find((session) => session.name === activeSession) ?? null,
        [activeSession, sessions],
    );
    const activePaneId = useMemo(() => resolveActivePaneID(currentSession), [currentSession]);
    const viewerShortcutsConfig = config?.viewer_shortcuts ?? null;
    const taskTemplates = config?.task_scheduler?.message_templates ?? EMPTY_TASK_TEMPLATES;

    useEffect(() => {
        mountedRef.current = true;
        return () => {
            mountedRef.current = false;
        };
    }, []);

    useEffect(() => {
        if (!open) {
            return undefined;
        }
        setQuery("");
        setSelectedIndex(0);
        setExecuting(false);
        executingRef.current = false;
        const frameID = requestAnimationFrame(() => inputRef.current?.focus());
        return () => cancelAnimationFrame(frameID);
    }, [open]);

    const updateDropdownPanelPosition = useCallback(() => {
        if (!open || !isDropdown) {
            setDropdownPanelStyle(undefined);
            return;
        }
        const anchor = dropdownAnchorRef?.current;
        if (anchor === undefined || anchor === null) {
            setDropdownPanelStyle(undefined);
            return;
        }
        const rect = anchor.getBoundingClientRect();
        const maxAvailableWidth = Math.max(DROPDOWN_MIN_WIDTH, window.innerWidth - (DROPDOWN_EDGE_PADDING * 2));
        const width = Math.min(
            DROPDOWN_MAX_WIDTH,
            maxAvailableWidth,
            Math.max(DROPDOWN_MIN_WIDTH, Math.round(rect.width) + 96),
        );
        const left = Math.min(
            Math.max(DROPDOWN_EDGE_PADDING, rect.left + ((rect.width - width) / 2)),
            Math.max(DROPDOWN_EDGE_PADDING, window.innerWidth - width - DROPDOWN_EDGE_PADDING),
        );
        const top = rect.bottom + DROPDOWN_GAP;
        const maxHeight = Math.min(
            DROPDOWN_MAX_HEIGHT,
            Math.max(DROPDOWN_MIN_HEIGHT, window.innerHeight - top - DROPDOWN_EDGE_PADDING),
        );
        setDropdownPanelStyle({
            top: `${Math.round(top)}px`,
            left: `${Math.round(left)}px`,
            width: `${Math.round(width)}px`,
            maxHeight: `${Math.round(maxHeight)}px`,
            transform: "none",
        });
    }, [dropdownAnchorRef, isDropdown, open]);

    useEffect(() => {
        if (!open || !isDropdown) {
            setDropdownPanelStyle(undefined);
            return undefined;
        }
        updateDropdownPanelPosition();
        window.addEventListener("resize", updateDropdownPanelPosition);
        window.addEventListener("scroll", updateDropdownPanelPosition, true);
        return () => {
            window.removeEventListener("resize", updateDropdownPanelPosition);
            window.removeEventListener("scroll", updateDropdownPanelPosition, true);
        };
    }, [isDropdown, open, updateDropdownPanelPosition]);

    useEffect(() => {
        if (!open || !isDropdown) {
            return undefined;
        }
        const handleDocumentMouseDown = (event: MouseEvent) => {
            const target = event.target;
            if (!(target instanceof Node)) {
                return;
            }
            if (panelRef.current?.contains(target)) {
                return;
            }
            if (dropdownAnchorRef?.current?.contains(target)) {
                return;
            }
            onClose();
        };
        document.addEventListener("mousedown", handleDocumentMouseDown);
        return () => {
            document.removeEventListener("mousedown", handleDocumentMouseDown);
        };
    }, [dropdownAnchorRef, isDropdown, onClose, open]);

    const sessionEntries = useMemo<SearchEntry[]>(() => (
        sessions.map((session) => {
            const repoName = extractRepoName(session);
            const branchName = session.worktree?.branch_name ?? "";
            return {
                id: `session:${session.name}`,
                label: session.name,
                groupLabel: tr("quickSearch.group.session", "セッション", "Session"),
                meta: [repoName, branchName].filter((value) => value !== ""),
                searchTargets: [session.name, repoName, branchName],
                rank: ENTRY_RANK_SESSION,
                operationLabel: tr(
                    "quickSearch.operation.activateSession",
                    "セッション切替: {sessionName}",
                    "Activate session: {sessionName}",
                    {sessionName: session.name},
                ),
                logLabel: "Activate session",
                execute: async () => {
                    await api.SetActiveSession(session.name);
                    setActiveSession(session.name);
                },
            };
        })
    ), [sessions, setActiveSession, tr]);

    const commonEntries = useMemo<SearchEntry[]>(() => {
        const entries: SearchEntry[] = [
            {
                id: "command:new-session",
                label: tr("quickSearch.command.newSession", "新しいセッション", "New Session"),
                groupLabel: tr("quickSearch.group.command", "コマンド", "Command"),
                meta: [],
                searchTargets: ["new session", "create session", "session", "新しいセッション"],
                rank: ENTRY_RANK_COMMAND,
                operationLabel: tr("quickSearch.command.newSession", "新しいセッション", "New Session"),
                logLabel: "Open new session",
                execute: onOpenNewSession,
            },
            {
                id: "command:open-settings",
                label: tr("quickSearch.command.openSettings", "設定を開く", "Open Settings"),
                groupLabel: tr("quickSearch.group.command", "コマンド", "Command"),
                meta: [],
                searchTargets: ["settings", "config", "preferences", "設定"],
                rank: ENTRY_RANK_COMMAND,
                operationLabel: tr("quickSearch.command.openSettings", "設定を開く", "Open Settings"),
                logLabel: "Open settings",
                execute: onOpenSettings,
            },
            {
                id: "command:toggle-viewer-sidebar-mode",
                label: tr("quickSearch.command.toggleViewerSidebarMode", "ドッキング表示を切り替え", "Toggle Docked Viewer"),
                groupLabel: tr("quickSearch.group.command", "コマンド", "Command"),
                meta: [],
                searchTargets: ["viewer dock", "toggle docked viewer", "sidebar mode", "dock", "viewer", "ドッキング"],
                rank: ENTRY_RANK_COMMAND,
                operationLabel: tr("quickSearch.command.toggleViewerSidebarMode", "ドッキング表示を切り替え", "Toggle Docked Viewer"),
                logLabel: "Toggle docked viewer",
                execute: async () => {
                    await api.ToggleViewerSidebarMode();
                },
            },
        ];

        if (activePaneId !== null) {
            entries.push(
                {
                    id: "command:split-pane-vertical",
                    label: tr("quickSearch.command.splitPaneVertical", "縦にペイン分割", "Split Pane Vertically"),
                    groupLabel: tr("quickSearch.group.command", "コマンド", "Command"),
                    meta: [activePaneId],
                    searchTargets: ["split pane vertical", "vertical split", "pane", "縦分割"],
                    rank: ENTRY_RANK_COMMAND,
                    operationLabel: tr("quickSearch.command.splitPaneVertical", "縦にペイン分割", "Split Pane Vertically"),
                    logLabel: "Split pane vertically",
                    execute: async () => {
                        await api.SplitPane(activePaneId, true);
                    },
                },
                {
                    id: "command:split-pane-horizontal",
                    label: tr("quickSearch.command.splitPaneHorizontal", "横にペイン分割", "Split Pane Horizontally"),
                    groupLabel: tr("quickSearch.group.command", "コマンド", "Command"),
                    meta: [activePaneId],
                    searchTargets: ["split pane horizontal", "horizontal split", "pane", "横分割"],
                    rank: ENTRY_RANK_COMMAND,
                    operationLabel: tr("quickSearch.command.splitPaneHorizontal", "横にペイン分割", "Split Pane Horizontally"),
                    logLabel: "Split pane horizontally",
                    execute: async () => {
                        await api.SplitPane(activePaneId, false);
                    },
                },
            );
        }

        return entries;
    }, [activePaneId, onOpenNewSession, onOpenSettings, tr]);

    const viewerShortcutAnalyses = useMemo(() => {
        return analyzeViewerShortcuts(
            views.map((view) => ({
                id: view.id,
                configuredShortcut: getViewerShortcutValue(viewerShortcutsConfig, view.id),
                defaultShortcut: view.shortcut,
            })),
        );
    }, [viewerShortcutsConfig, views]);

    const viewerEntries = useMemo<SearchEntry[]>(() => {
        return views.map((view) => {
            const shortcutAnalysis = viewerShortcutAnalyses.get(view.id);
            const shortcut = shortcutAnalysis
                ? shortcutAnalysis.bindingShortcut
                : getEffectiveViewerShortcut(
                    getViewerShortcutValue(viewerShortcutsConfig, view.id),
                    view.shortcut,
                );
            return {
                id: `viewer:${view.id}`,
                label: tr(
                    "quickSearch.command.openViewer",
                    "{label} を開く",
                    "Open {label}",
                    {label: view.label},
                ),
                groupLabel: tr("quickSearch.group.viewer", "ビュー", "Viewer"),
                meta: shortcut ? [formatShortcutForDisplay(shortcut)] : [],
                searchTargets: [
                    view.label,
                    view.id,
                    `open ${view.label}`,
                ],
                rank: ENTRY_RANK_VIEWER,
                operationLabel: tr(
                    "quickSearch.command.openViewer",
                    "{label} を開く",
                    "Open {label}",
                    {label: view.label},
                ),
                logLabel: `Open ${view.label}`,
                execute: () => {
                    openView(view.id);
                },
            };
        });
    }, [openView, tr, viewerShortcutAnalyses, viewerShortcutsConfig, views]);

    const createTaskSchedulerLaunchKey = useCallback((
        index: number,
        templateLabel: string,
        targetPaneId: string | null,
    ): string => {
        templateLaunchTokenRef.current += 1;
        return `task-template:${index}:${templateLabel}:${targetPaneId ?? "none"}:${templateLaunchTokenRef.current}`;
    }, []);

    const templateEntries = useMemo<SearchEntry[]>(() => {
        return taskTemplates
            .map((template, index) => ({template, index}))
            .filter(({template}) => template.message.trim() !== "")
            .map(({template, index}) => buildTemplateEntry(
                template,
                index,
                activePaneId,
                openViewWithContext,
                tr,
                createTaskSchedulerLaunchKey,
            ));
    }, [activePaneId, createTaskSchedulerLaunchKey, openViewWithContext, taskTemplates, tr]);

    const results = useMemo<SearchResult[]>(() => {
        const normalizedQuery = query.trim().toLowerCase();
        const hasQuery = normalizedQuery !== "";
        return [...sessionEntries, ...commonEntries, ...viewerEntries, ...templateEntries]
            .map((entry) => ({...entry, score: scoreEntry(normalizedQuery, entry)}))
            .filter((entry) => entry.score > 0)
            .sort((left, right) => sortResults(left, right, hasQuery));
    }, [commonEntries, query, sessionEntries, templateEntries, viewerEntries]);

    useEffect(() => {
        setSelectedIndex((current) => {
            if (results.length === 0) {
                return 0;
            }
            if (current < 0) {
                return 0;
            }
            if (current >= results.length) {
                return results.length - 1;
            }
            return current;
        });
    }, [results.length]);

    useEffect(() => {
        if (!open) {
            return;
        }
        const selectedResult = results[selectedIndex];
        if (!selectedResult) {
            return;
        }
        getResultItemRefs().get(selectedResult.id)?.scrollIntoView({
            block: "nearest",
            inline: "nearest",
        });
    }, [getResultItemRefs, open, results, selectedIndex]);

    const selectResult = useCallback(async (result: SearchResult) => {
        if (executingRef.current) {
            return;
        }
        executingRef.current = true;
        setExecuting(true);
        try {
            await result.execute();
            if (!mountedRef.current) {
                return;
            }
            onClose();
        } catch (err) {
            if (!mountedRef.current) {
                return;
            }
            notifyOperationFailure(result.operationLabel, "warn", err);
            logFrontendEventSafe(
                "warn",
                `${result.logLabel} failed: ${toErrorMessage(err, "unknown error")}`,
                "QuickSearch",
            );
        } finally {
            executingRef.current = false;
            if (mountedRef.current) {
                setExecuting(false);
            }
        }
    }, [onClose]);

    const handleKeyDown = useCallback((event: ReactKeyboardEvent<HTMLInputElement>) => {
        if (isImeTransitionalEvent(event.nativeEvent)) {
            return;
        }
        if (event.key === "Escape") {
            event.preventDefault();
            onClose();
            return;
        }
        if (event.key === "ArrowDown") {
            event.preventDefault();
            setSelectedIndex((index) => Math.min(index + 1, Math.max(0, results.length - 1)));
            return;
        }
        if (event.key === "ArrowUp") {
            event.preventDefault();
            setSelectedIndex((index) => Math.max(index - 1, 0));
            return;
        }
        if (event.key === "Enter" && results[selectedIndex] && !executingRef.current) {
            event.preventDefault();
            void selectResult(results[selectedIndex]);
        }
    }, [onClose, results, selectResult, selectedIndex]);

    if (!open) return null;

    return (
        <div className={isDropdown ? "quick-search-dropdown-shell" : "quick-search-overlay"} onClick={isDropdown ? undefined : onClose}>
            <div
                ref={panelRef}
                className={`quick-search-panel${isDropdown ? " quick-search-panel--dropdown" : ""}`}
                id={QUICK_SEARCH_DIALOG_ID}
                role="dialog"
                aria-modal={isDropdown ? undefined : true}
                aria-label={tr("quickSearch.aria.dialogLabel", "コマンドパレット", "Command Palette")}
                style={dropdownPanelStyle}
                onClick={(event) => event.stopPropagation()}
            >
                <input
                    ref={inputRef}
                    className="quick-search-input"
                    placeholder={tr(
                        "quickSearch.placeholder",
                        "セッション・ビュー・コマンド・テンプレートを検索...",
                        "Search sessions, viewers, commands, or templates...",
                    )}
                    value={query}
                    readOnly={executing}
                    aria-disabled={executing}
                    role="combobox"
                    aria-expanded="true"
                    aria-controls={listboxId}
                    aria-activedescendant={results[selectedIndex] ? `${listboxId}-option-${selectedIndex}` : undefined}
                    onChange={(event) => {
                        setQuery(event.target.value);
                        setSelectedIndex(0);
                    }}
                    onKeyDown={handleKeyDown}
                />
                {executing && (
                    <div className="quick-search-loading">
                        {tr("quickSearch.executing", "実行中...", "Running...")}
                    </div>
                )}
                <div
                    className="quick-search-results"
                    id={listboxId}
                    role="listbox"
                    aria-label={tr("quickSearch.aria.resultsLabel", "検索結果", "Search results")}
                >
                    {results.map((result, index) => (
                        <div
                            key={result.id}
                            ref={(node) => {
                                if (node === null) {
                                    getResultItemRefs().delete(result.id);
                                    return;
                                }
                                getResultItemRefs().set(result.id, node);
                            }}
                            id={`${listboxId}-option-${index}`}
                            className={`quick-search-item ${index === selectedIndex ? "selected" : ""}`}
                            role="option"
                            aria-selected={index === selectedIndex}
                            onClick={() => {
                                if (!executingRef.current) {
                                    void selectResult(result);
                                }
                            }}
                            onMouseEnter={() => setSelectedIndex(index)}
                        >
                            <span className="quick-search-name">{result.label}</span>
                            <span className="quick-search-meta">{result.groupLabel}</span>
                            {result.meta.map((value) => (
                                <span key={`${result.id}:${value}`} className="quick-search-meta">{value}</span>
                            ))}
                        </div>
                    ))}
                    {results.length === 0 && (
                        <div className="quick-search-empty">
                            {tr("quickSearch.empty", "該当なし", "No results")}
                        </div>
                    )}
                </div>
            </div>
        </div>
    );
}

function buildTemplateEntry(
    template: AppConfigMessageTemplate,
    index: number,
    activePaneId: string | null,
    openViewWithContext: OpenViewWithContext,
    tr: (key: string, jaText: string, enText: string, vars?: Record<string, string | number>) => string,
    createTaskSchedulerLaunchKey: (index: number, templateLabel: string, activePaneId: string | null) => string,
): SearchEntry {
    const templateName = template.name.trim();
    const templateLabel = templateName === ""
        ? tr("quickSearch.command.insertTemplateDefaultName", "無題テンプレート", "Untitled Template")
        : templateName;

    return {
        id: `task-template:${index}:${templateLabel}`,
        label: tr(
            "quickSearch.command.openTaskTemplateDraft",
            "テンプレート下書きを開く: {name}",
            "Open Task Template Draft: {name}",
            {name: templateLabel},
        ),
        groupLabel: tr("quickSearch.group.template", "テンプレート", "Template"),
        meta: [tr("quickSearch.group.taskScheduler", "Task Scheduler", "Task Scheduler")],
        searchTargets: [
            templateLabel,
            template.message,
            `task scheduler ${templateLabel}`,
            "template",
        ],
        rank: ENTRY_RANK_TEMPLATE,
        operationLabel: tr(
            "quickSearch.command.openTaskTemplateDraft",
            "テンプレート下書きを開く: {name}",
            "Open Task Template Draft: {name}",
            {name: templateLabel},
        ),
        logLabel: "Open task template draft",
        execute: () => {
            const nextContext: TaskSchedulerTemplateViewContext = {
                kind: "task-scheduler-template",
                key: createTaskSchedulerLaunchKey(index, templateLabel, activePaneId),
                name: templateName,
                message: template.message,
                targetPaneID: activePaneId ?? "",
                clearBefore: false,
                clearCommand: "",
            };
            openViewWithContext("task-scheduler", nextContext);
        },
    };
}
