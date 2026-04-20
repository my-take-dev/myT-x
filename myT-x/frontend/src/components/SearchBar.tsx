import {type KeyboardEvent, useEffect, useRef, useState} from "react";
import type {ISearchOptions, SearchAddon} from "@xterm/addon-search";
import {useI18n} from "../i18n";
import {isImeTransitionalEvent} from "../utils/ime";

interface SearchBarProps {
    open: boolean;
    onClose: () => void;
    searchAddon: SearchAddon | null;
}

interface SearchResultState {
    readonly current: number;
    readonly total: number;
    readonly thresholdExceeded: boolean;
}

const SEARCH_OPTIONS: ISearchOptions = {
    caseSensitive: false,
    regex: false,
    wholeWord: false,
    incremental: true,
    decorations: {
        matchBackground: "#5d4c1a",
        matchBorder: "#c89f1d",
        matchOverviewRuler: "#c89f1d",
        activeMatchBackground: "#f6d365",
        activeMatchBorder: "#fff2c7",
        activeMatchColorOverviewRuler: "#f6d365",
    },
};

function createEmptySearchResultState(): SearchResultState {
    return {
        current: 0,
        total: 0,
        thresholdExceeded: false,
    };
}

function formatSearchResultLabel(searchResult: SearchResultState): string {
    if (searchResult.thresholdExceeded) {
        return `- / ${searchResult.total}+`;
    }
    return `${searchResult.current} / ${searchResult.total}`;
}

/**
 * searchAddon に対する操作は try-catch で保護する。
 *
 * TerminalPane のクリーンアップで searchAddonRef.current が null 化されるが、
 * ペインの再マウント前後の短い窓で SearchBar がまだマウントされており、
 * 操作が呼ばれると dispose 済みの addon にアクセスしてクラッシュする可能性がある。
 * null チェック + try-catch により disposed な addon への操作を安全に無視する。
 */
function safeAddonOp(addon: SearchAddon | null, op: (a: SearchAddon) => void): void {
    if (!addon) return;
    try {
        op(addon);
    } catch (err: unknown) {
        console.warn("[DEBUG-search] searchAddon operation failed (possibly disposed)", err);
    }
}

export function SearchBar({open, onClose, searchAddon}: SearchBarProps) {
    const {t} = useI18n();
    const inputRef = useRef<HTMLInputElement | null>(null);
    const [query, setQuery] = useState("");
    const [searchResult, setSearchResult] = useState<SearchResultState>(createEmptySearchResultState);

    useEffect(() => {
        if (!searchAddon) {
            setSearchResult(createEmptySearchResultState());
            return;
        }
        const disposable = searchAddon.onDidChangeResults(({resultCount, resultIndex}) => {
            setSearchResult({
                current: resultCount > 0 && resultIndex >= 0 ? resultIndex + 1 : 0,
                total: resultCount,
                thresholdExceeded: resultCount > 0 && resultIndex === -1,
            });
        });
        return () => {
            disposable.dispose();
        };
    }, [searchAddon]);

    useEffect(() => {
        if (open) {
            // Clear the previous query and decorations every time the bar reopens
            // so users start fresh rather than seeing stale search state.
            setQuery("");
            setSearchResult(createEmptySearchResultState());
            safeAddonOp(searchAddon, (a) => a.clearDecorations());
            inputRef.current?.focus();
            inputRef.current?.select();
        }
    }, [open, searchAddon]);

    if (!open) return null;

    const findNext = () => {
        if (!query) return;
        safeAddonOp(searchAddon, (a) =>
            a.findNext(query, SEARCH_OPTIONS),
        );
    };

    const findPrev = () => {
        if (!query) return;
        // xterm applies `incremental` only to findNext. Reusing the shared options for
        // findPrevious keeps the other flags aligned and leaves incremental as a safe no-op.
        safeAddonOp(searchAddon, (a) =>
            a.findPrevious(query, SEARCH_OPTIONS),
        );
    };

    const handleKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
        if (isImeTransitionalEvent(e.nativeEvent)) {
            return;
        }
        if (e.key === "Escape") {
            e.preventDefault();
            // I-25: 上位ハンドラ（usePrefixKeyMode 等）への不要な伝播を防止する。
            e.stopPropagation();
            safeAddonOp(searchAddon, (a) => a.clearDecorations());
            onClose();
            return;
        }
        if (e.key === "Enter") {
            e.preventDefault();
            e.stopPropagation();
            if (e.shiftKey) {
                findPrev();
            } else {
                findNext();
            }
        }
    };

    return (
        <div className="terminal-search-bar">
            <input
                ref={inputRef}
                className="terminal-search-input"
                type="text"
                placeholder={t("terminalSearch.placeholder", "検索...")}
                value={query}
                onChange={(e) => {
                    const value = e.target.value;
                    setQuery(value);
                    if (!value) {
                        setSearchResult(createEmptySearchResultState());
                        safeAddonOp(searchAddon, (a) => a.clearDecorations());
                        return;
                    }
                    safeAddonOp(searchAddon, (a) => a.findNext(value, SEARCH_OPTIONS));
                }}
                onKeyDown={handleKeyDown}
            />
            <span
                className="terminal-search-results"
                aria-live="polite"
                title={searchResult.thresholdExceeded
                    ? t("terminalSearch.results.limitExceededTitle", "検索結果（上限超過）")
                    : t("terminalSearch.results.title", "検索結果")}
            >
                {formatSearchResultLabel(searchResult)}
            </span>
            <button
                type="button"
                className="terminal-search-btn"
                title={t("terminalSearch.prev.title", "前へ (Shift+Enter)")}
                onClick={findPrev}
            >
                &#x25B2;
            </button>
            <button
                type="button"
                className="terminal-search-btn"
                title={t("terminalSearch.next.title", "次へ (Enter)")}
                onClick={findNext}
            >
                &#x25BC;
            </button>
            <button
                type="button"
                className="terminal-search-btn terminal-search-btn-close"
                title={t("terminalSearch.close.title", "閉じる (Esc)")}
                onClick={() => {
                    safeAddonOp(searchAddon, (a) => a.clearDecorations());
                    onClose();
                }}
            >
                &times;
            </button>
        </div>
    );
}
