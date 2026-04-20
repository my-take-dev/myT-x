import {useCallback, useEffect, useMemo, useRef, useState} from "react";
import {EventsOn} from "../../wailsjs/runtime/runtime";
import {GetSessionEnlistmentContext} from "../../wailsjs/go/main/App";
import type {PaneSnapshot} from "../types/tmux";
import type {
    OrchestratorSessionEnlistmentContext,
    OrchestratorStorageLocation,
    OrchestratorTeamDefinition,
} from "../components/viewer/views/orchestrator-teams/types";
import {UNAFFILIATED_TEAM_ID} from "../components/viewer/views/orchestrator-teams/orchestratorTeamUtils";
import {toErrorMessage} from "../utils/errorUtils";
import {notifyAndLog} from "../utils/notifyUtils";

export interface UnregisteredPaneEntry {
    pane: PaneSnapshot;
    parentPaneId: string | null;
    parentPaneTitle: string;
    suggestedTeamID: string | null;
    suggestedStorageLocation: OrchestratorStorageLocation;
    suggestedRole: string;
}

interface PaneCreatedPayload {
    sessionName?: unknown;
    paneId?: unknown;
    parentPaneId?: unknown;
}

function payloadTargetsSession(payload: unknown, sessionName: string): boolean {
    if (!payload || typeof payload !== "object") {
        return false;
    }
    const candidate = (payload as {sessionName?: unknown}).sessionName;
    return typeof candidate === "string" && candidate === sessionName;
}

function normalizeStorageLocation(team: OrchestratorTeamDefinition): OrchestratorStorageLocation {
    return team.storage_location === "project" ? "project" : "global";
}

function findParentSuggestion(
    teams: OrchestratorTeamDefinition[],
    parentPaneTitle: string,
): Pick<UnregisteredPaneEntry, "suggestedTeamID" | "suggestedStorageLocation" | "suggestedRole"> {
    const trimmedTitle = parentPaneTitle.trim();
    if (trimmedTitle === "") {
        return {
            suggestedTeamID: null,
            suggestedStorageLocation: "global",
            suggestedRole: "",
        };
    }

    const matchingTeams = teams.flatMap((team) => {
        const matchedMember = team.members.find((member) => member.pane_title.trim() === trimmedTitle);
        if (!matchedMember) {
            return [];
        }
        return [{
            team,
            role: matchedMember.role,
        }];
    });

    if (matchingTeams.length === 0) {
        return {
            suggestedTeamID: null,
            suggestedStorageLocation: "global",
            suggestedRole: "",
        };
    }

    const preferred = matchingTeams.find((entry) => entry.team.id !== UNAFFILIATED_TEAM_ID) ?? matchingTeams[0];
    return {
        suggestedTeamID: preferred.team.id,
        suggestedStorageLocation: normalizeStorageLocation(preferred.team),
        suggestedRole: preferred.role,
    };
}

export function useUnregisteredPanes(sessionName: string | null, panes: PaneSnapshot[]) {
    const [context, setContext] = useState<OrchestratorSessionEnlistmentContext | null>(null);
    const [parentPaneMap, setParentPaneMap] = useState<Record<string, string>>({});
    const activeSessionRef = useRef<string | null>(sessionName);
    const mountedRef = useRef(true);
    const requestIdRef = useRef(0);

    useEffect(() => {
        activeSessionRef.current = sessionName;
    }, [sessionName]);

    useEffect(() => {
        mountedRef.current = true;
        return () => {
            mountedRef.current = false;
        };
    }, []);

    const reload = useCallback(async () => {
        const requestId = requestIdRef.current + 1;
        requestIdRef.current = requestId;
        if (!sessionName) {
            setContext(null);
            return;
        }
        const capturedSessionName = sessionName;
        try {
            const result = await GetSessionEnlistmentContext(capturedSessionName) as OrchestratorSessionEnlistmentContext;
            if (
                !mountedRef.current
                || activeSessionRef.current !== capturedSessionName
                || requestIdRef.current !== requestId
            ) {
                return;
            }
            setContext(result);
        } catch (err: unknown) {
            if (
                !mountedRef.current
                || activeSessionRef.current !== capturedSessionName
                || requestIdRef.current !== requestId
            ) {
                return;
            }
            setContext(null);
            notifyAndLog("Load pane enlistment data", "warn", err, "useUnregisteredPanes");
            console.warn("[use-unregistered-panes] enlistment context reload failed", toErrorMessage(err, "Failed to load pane enlistment data."));
        }
    }, [sessionName]);

    useEffect(() => {
        setParentPaneMap({});
        void reload();
    }, [reload]);

    useEffect(() => {
        if (!sessionName) {
            return;
        }

        const disposeAgentsUpdated = EventsOn("orchestrator:agents-updated", (payload: unknown) => {
            if (!payloadTargetsSession(payload, sessionName)) {
                return;
            }
            void reload();
        });

        const disposePaneCreated = EventsOn("tmux:pane-created", (payload: unknown) => {
            if (!payloadTargetsSession(payload, sessionName)) {
                return;
            }
            const {paneId, parentPaneId} = payload as PaneCreatedPayload;
            if (typeof paneId !== "string" || typeof parentPaneId !== "string" || parentPaneId.trim() === "") {
                return;
            }
            setParentPaneMap((current) => ({
                ...current,
                [paneId]: parentPaneId,
            }));
        });

        return () => {
            disposeAgentsUpdated();
            disposePaneCreated();
        };
    }, [reload, sessionName]);

    useEffect(() => {
        const activePaneIDs = new Set(panes.map((pane) => pane.id));
        setParentPaneMap((current) => {
            const next = Object.fromEntries(
                Object.entries(current).filter(([paneID]) => activePaneIDs.has(paneID)),
            );
            return Object.keys(next).length === Object.keys(current).length ? current : next;
        });
    }, [panes]);

    const unregisteredPanes = useMemo<UnregisteredPaneEntry[]>(() => {
        if (!context || panes.length === 0) {
            return [];
        }

        const paneTitleByID = new Map(panes.map((pane) => [pane.id, pane.title?.trim() ?? ""]));

        const sortedPanes = [...panes].sort((a, b) => {
            if (a.index !== b.index) {
                return a.index - b.index;
            }
            return comparePaneIDs(a.id, b.id);
        });
        const initialPaneId = sortedPanes[0]?.id ?? null;
        const registeredPaneIDs = new Set(context.registered_pane_ids ?? []);

        return sortedPanes
            .filter((pane) => pane.id !== initialPaneId)
            .filter((pane) => !registeredPaneIDs.has(pane.id))
            .map((pane) => {
                const parentPaneId = parentPaneMap[pane.id] ?? null;
                const parentPaneTitle = parentPaneId != null
                    ? paneTitleByID.get(parentPaneId) ?? ""
                    : "";
                const suggestion = findParentSuggestion(context.teams, parentPaneTitle);
                return {
                    pane,
                    parentPaneId,
                    parentPaneTitle,
                    ...suggestion,
                };
            });
    }, [context, panes, parentPaneMap]);

    return {
        context,
        unregisteredPanes,
        reload,
    };
}

function comparePaneIDs(left: string, right: string): number {
    return parsePaneID(left) - parsePaneID(right);
}

function parsePaneID(paneID: string): number {
    const parsed = Number.parseInt(paneID.replace(/^%/, ""), 10);
    return Number.isFinite(parsed) ? parsed : Number.MAX_SAFE_INTEGER;
}
