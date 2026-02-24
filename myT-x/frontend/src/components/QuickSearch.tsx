import {useEffect, useId, useMemo, useRef, useState} from "react";
import {api} from "../api";
import {useNotificationStore} from "../stores/notificationStore";
import {useTmuxStore} from "../stores/tmuxStore";
import type {SessionSnapshot} from "../types/tmux";

interface QuickSearchProps {
    open: boolean;
    onClose: () => void;
}

interface SearchCandidate {
    sessionName: string;
    repoName: string;
    branchName: string;
    score: number;
}

function fuzzyScore(query: string, target: string): number {
    if (!query || !target) return 0;
    const lq = query.toLowerCase();
    const lt = target.toLowerCase();
    if (lt === lq) return 100;
    if (lt.startsWith(lq)) return 80;
    if (lt.includes(lq)) return 60;
    let qi = 0;
    for (let ti = 0; ti < lt.length && qi < lq.length; ti++) {
        if (lt[ti] === lq[qi]) qi++;
    }
    if (qi === lq.length) return 40;
    return 0;
}

function searchSessions(sessions: SessionSnapshot[], query: string): SearchCandidate[] {
    if (!query.trim()) {
        return sessions.map((s) => ({
            sessionName: s.name,
            repoName: s.worktree?.repo_path?.split(/[\\/]/).filter(Boolean).pop() ?? "",
            branchName: s.worktree?.branch_name ?? "",
            score: 1,
        }));
    }
    return sessions
        .map((s) => {
            const repoName = s.worktree?.repo_path?.split(/[\\/]/).filter(Boolean).pop() ?? "";
            const branchName = s.worktree?.branch_name ?? "";
            const nameScore = fuzzyScore(query, s.name);
            const repoScore = fuzzyScore(query, repoName);
            const branchScore = fuzzyScore(query, branchName);
            const score = Math.max(nameScore, repoScore, branchScore);
            return {sessionName: s.name, repoName, branchName, score};
        })
        .filter((c) => c.score > 0)
        .sort((a, b) => b.score - a.score);
}

export function QuickSearch({open, onClose}: QuickSearchProps) {
    const sessions = useTmuxStore((s) => s.sessions);
    const setActiveSession = useTmuxStore((s) => s.setActiveSession);
    const addNotification = useNotificationStore((s) => s.addNotification);
    const [query, setQuery] = useState("");
    const [selectedIndex, setSelectedIndex] = useState(0);
    const [switching, setSwitching] = useState(false);
    const switchingRef = useRef(false);
    const mountedRef = useRef(true);
    const inputRef = useRef<HTMLInputElement>(null);
    const listboxId = useId();

    const results = useMemo(() => searchSessions(sessions, query), [sessions, query]);

    useEffect(() => {
        if (!open) {
            return undefined;
        }
        setQuery("");
        setSelectedIndex(0);
        setSwitching(false);
        switchingRef.current = false;
        const frameID = requestAnimationFrame(() => inputRef.current?.focus());
        return () => cancelAnimationFrame(frameID);
    }, [open]);

    useEffect(() => {
        mountedRef.current = true;
        return () => {
            mountedRef.current = false;
        };
    }, []);

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

    const selectResult = async (sessionName: string) => {
        if (switchingRef.current) return;
        switchingRef.current = true;
        setSwitching(true);
        try {
            await api.SetActiveSession(sessionName);
            if (!mountedRef.current) return;
            setActiveSession(sessionName);
            onClose();
        } catch (err) {
            if (!mountedRef.current) return;
            console.warn("[DEBUG-QUICKSEARCH] SetActiveSession failed:", err);
            addNotification(`Failed to activate session "${sessionName}".`, "warn");
        } finally {
            switchingRef.current = false;
            if (mountedRef.current) {
                setSwitching(false);
            }
        }
    };

    const handleKeyDown = (e: React.KeyboardEvent) => {
        if (e.key === "Escape") {
            onClose();
            return;
        }
        if (e.key === "ArrowDown") {
            e.preventDefault();
            setSelectedIndex((i) => Math.min(i + 1, Math.max(0, results.length - 1)));
            return;
        }
        if (e.key === "ArrowUp") {
            e.preventDefault();
            setSelectedIndex((i) => Math.max(i - 1, 0));
            return;
        }
        if (e.key === "Enter" && results[selectedIndex] && !switchingRef.current) {
            void selectResult(results[selectedIndex].sessionName);
        }
    };

    if (!open) return null;

    return (
        <div className="quick-search-overlay" onClick={onClose}>
            <div
                className="quick-search-panel"
                role="dialog"
                aria-modal="true"
                aria-label="セッションクイック検索"
                onClick={(e) => e.stopPropagation()}
            >
                <input
                    ref={inputRef}
                    className="quick-search-input"
                    placeholder="セッション名・リポ名・ブランチ名で検索..."
                    value={query}
                    disabled={switching}
                    role="combobox"
                    aria-expanded="true"
                    aria-controls={listboxId}
                    aria-activedescendant={results[selectedIndex] ? `${listboxId}-option-${selectedIndex}` : undefined}
                    onChange={(e) => {
                        setQuery(e.target.value);
                        setSelectedIndex(0);
                    }}
                    onKeyDown={handleKeyDown}
                />
                {switching && <div className="quick-search-loading">切り替え中...</div>}
                <div className="quick-search-results" id={listboxId} role="listbox" aria-label="検索結果">
                    {results.map((r, i) => (
                        <div
                            key={r.sessionName}
                            id={`${listboxId}-option-${i}`}
                            className={`quick-search-item ${i === selectedIndex ? "selected" : ""}`}
                            role="option"
                            aria-selected={i === selectedIndex}
                            onClick={() => {
                                if (!switchingRef.current) void selectResult(r.sessionName);
                            }}
                            onMouseEnter={() => setSelectedIndex(i)}
                        >
                            <span className="quick-search-name">{r.sessionName}</span>
                            {r.repoName && <span className="quick-search-meta">{r.repoName}</span>}
                            {r.branchName && <span className="quick-search-meta">{r.branchName}</span>}
                        </div>
                    ))}
                    {results.length === 0 && query && (
                        <div className="quick-search-empty">該当なし</div>
                    )}
                </div>
            </div>
        </div>
    );
}
