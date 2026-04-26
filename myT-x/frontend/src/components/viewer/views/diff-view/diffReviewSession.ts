import {useMemo} from "react";
import {useTmuxStore} from "../../../../stores/tmuxStore";
import {buildDiffReviewSessionKey} from "./diffReviewKeys";

export function useDiffReviewSessionKey(): string {
    const sessions = useTmuxStore((state) => state.sessions);
    const activeSession = useTmuxStore((state) => state.activeSession);
    const activeSessionSnapshot = useMemo(
        () => (activeSession ? sessions.find((entry) => entry.name === activeSession) ?? null : null),
        [sessions, activeSession],
    );

    return activeSessionSnapshot ? buildDiffReviewSessionKey(activeSessionSnapshot.id) : "";
}
