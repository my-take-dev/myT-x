import {useCallback, useEffect, useRef, useState} from "react";
import {api} from "../../../../api";
import {useTmuxStore} from "../../../../stores/tmuxStore";
import {toErrorMessage} from "../../../../utils/errorUtils";
import type {SearchFileResult} from "./fileTreeTypes";

export interface UseFileSearchResult {
    readonly query: string;
    readonly setQuery: (q: string) => void;
    readonly results: readonly SearchFileResult[];
    readonly isSearching: boolean;
    readonly searchError: string | null;
    readonly clearSearch: () => void;
}

/** Debounce delay for search input (ms). */
const SEARCH_DEBOUNCE_MS = 300;

/**
 * Custom hook that manages file search state for the DevPanel viewer.
 *
 * Responsibilities:
 * - Debounces search queries before calling the backend API.
 * - Guards against stale async responses using mountedRef, sessionRef, and requestRef.
 * - Resets state when the active session changes.
 */
export function useFileSearch(): UseFileSearchResult {
    const activeSession = useTmuxStore((s) => s.activeSession);

    const [query, setQueryState] = useState("");
    const [results, setResults] = useState<readonly SearchFileResult[]>([]);
    const [isSearching, setIsSearching] = useState(false);
    const [searchError, setSearchError] = useState<string | null>(null);

    // ── Refs (stale-closure prevention) ──

    const prevSessionRef = useRef<string | null>(null);
    const mountedRef = useRef(true);
    const sessionRef = useRef(activeSession);
    const searchRequestRef = useRef(0);
    const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

    // Synchronize sessionRef during render (same pattern as useFileTree).
    sessionRef.current = activeSession;

    // ── Mount / unmount lifecycle ──

    useEffect(() => {
        mountedRef.current = true;
        return () => {
            mountedRef.current = false;
            if (debounceRef.current !== null) {
                clearTimeout(debounceRef.current);
            }
        };
    }, []);

    // ── Session change reset ──

    useEffect(() => {
        if (prevSessionRef.current === activeSession) return;
        prevSessionRef.current = activeSession;
        searchRequestRef.current += 1;
        if (debounceRef.current !== null) {
            clearTimeout(debounceRef.current);
            debounceRef.current = null;
        }
        setQueryState("");
        setResults([]);
        setIsSearching(false);
        setSearchError(null);
    }, [activeSession]);

    // ── Internal search execution ──

    const executeSearch = useCallback((searchQuery: string) => {
        const capturedSession = sessionRef.current?.trim();
        if (!capturedSession) {
            setResults([]);
            setIsSearching(false);
            return;
        }

        const reqId = ++searchRequestRef.current;
        setIsSearching(true);
        setSearchError(null);

        void api.DevPanelSearchFiles(capturedSession, searchQuery)
            .then((searchResults) => {
                if (!mountedRef.current) return;
                if (sessionRef.current?.trim() !== capturedSession) return;
                if (searchRequestRef.current !== reqId) return;
                setResults(searchResults);
            })
            .catch((err: unknown) => {
                if (!mountedRef.current) return;
                if (sessionRef.current?.trim() !== capturedSession) return;
                if (searchRequestRef.current !== reqId) return;
                console.error("[file-search] DevPanelSearchFiles failed", {
                    session: capturedSession,
                    query: searchQuery,
                    err,
                });
                setResults([]);
                setSearchError(toErrorMessage(err, "Search failed."));
            })
            .finally(() => {
                if (!mountedRef.current) return;
                if (searchRequestRef.current !== reqId) return;
                setIsSearching(false);
            });
    }, []);

    // ── Public setQuery with debounce ──

    const setQuery = useCallback((q: string) => {
        setQueryState(q);

        if (debounceRef.current !== null) {
            clearTimeout(debounceRef.current);
            debounceRef.current = null;
        }

        const trimmed = q.trim();
        if (trimmed === "") {
            // Immediate reset: no API call needed.
            searchRequestRef.current += 1;
            setResults([]);
            setIsSearching(false);
            setSearchError(null);
            return;
        }

        debounceRef.current = setTimeout(() => {
            debounceRef.current = null;
            executeSearch(trimmed);
        }, SEARCH_DEBOUNCE_MS);
    }, [executeSearch]);

    // ── clearSearch ──

    const clearSearch = useCallback(() => {
        if (debounceRef.current !== null) {
            clearTimeout(debounceRef.current);
            debounceRef.current = null;
        }
        searchRequestRef.current += 1;
        setQueryState("");
        setResults([]);
        setIsSearching(false);
        setSearchError(null);
    }, []);

    return {query, setQuery, results, isSearching, searchError, clearSearch};
}
