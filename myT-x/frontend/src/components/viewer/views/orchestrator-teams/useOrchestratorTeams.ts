import {useCallback, useEffect, useRef, useState} from "react";
import {
    AddMemberToUnaffiliatedTeam,
    BootstrapMemberToPane,
    DeleteOrchestratorTeam,
    EnsureUnaffiliatedTeam,
    LoadOrchestratorTeams,
    ReorderOrchestratorTeams,
    SaveOrchestratorTeam,
    SaveUnaffiliatedTeamMembers,
    StartOrchestratorTeam,
} from "../../../../../wailsjs/go/main/App";
import {orchestrator} from "../../../../../wailsjs/go/models";
import {translate} from "../../../../i18n";
import {useTmuxStore} from "../../../../stores/tmuxStore";
import {toErrorMessage} from "../../../../utils/errorUtils";
import type {
    BootstrapMemberToPaneRequest,
    BootstrapMemberToPaneResult,
    OrchestratorStorageLocation,
    OrchestratorTeamDefinition,
    OrchestratorTeamDraft,
    OrchestratorTeamMember,
    StartOrchestratorTeamRequest,
    StartOrchestratorTeamResult,
} from "./types";
import {buildTeamPayload} from "./orchestratorTeamUtils";

export function useOrchestratorTeams() {
    const activeSession = useTmuxStore((state) => state.activeSession);
    const [teams, setTeams] = useState<OrchestratorTeamDefinition[]>([]);
    const [error, setError] = useState<string | null>(null);
    const [notice, setNotice] = useState<string | null>(null);
    const [loading, setLoading] = useState(true);
    const isMountedRef = useRef(true);

    useEffect(() => {
        isMountedRef.current = true;
        return () => {
            isMountedRef.current = false;
        };
    }, []);

    const refresh = useCallback(async () => {
        setLoading(true);
        try {
            const result = await LoadOrchestratorTeams(activeSession ?? "");
            if (!isMountedRef.current) return;
            setTeams((result ?? []) as OrchestratorTeamDefinition[]);
        } catch (err) {
            if (!isMountedRef.current) return;
            console.warn("[orchestrator-teams] failed to load teams", err);
            setError(toErrorMessage(err, "Failed to load teams."));
        } finally {
            if (isMountedRef.current) {
                setLoading(false);
            }
        }
    }, [activeSession]);

    useEffect(() => {
        void refresh();
    }, [refresh]);

    const saveTeam = useCallback(async (draft: OrchestratorTeamDraft) => {
        setError(null);
        setNotice(null);
        try {
            await SaveOrchestratorTeam(orchestrator.TeamDefinition.createFrom(buildTeamPayload(draft)), activeSession ?? "");
            await refresh();
        } catch (err) {
            setError(toErrorMessage(err, "Failed to save team."));
            throw err;
        }
    }, [activeSession, refresh]);

    const deleteTeam = useCallback(async (teamID: string, storageLocation: OrchestratorStorageLocation = "global") => {
        setError(null);
        setNotice(null);
        try {
            await DeleteOrchestratorTeam(teamID, storageLocation, activeSession ?? "");
            await refresh();
        } catch (err) {
            setError(toErrorMessage(err, "Failed to delete team."));
            throw err;
        }
    }, [activeSession, refresh]);

    const startTeam = useCallback(async (request: StartOrchestratorTeamRequest): Promise<StartOrchestratorTeamResult> => {
        setError(null);
        setNotice(null);
        try {
            const result = await StartOrchestratorTeam(orchestrator.StartTeamRequest.createFrom(request)) as StartOrchestratorTeamResult;
            if (result.warnings.length > 0) {
                const prefix = translate("viewer.orchestratorTeams.notice.startedWithWarnings", "{name} を起動しました（警告あり）:", {name: result.session_name});
                setNotice(`${prefix}\n${result.warnings.join("\n")}`);
            } else {
                setNotice(translate("viewer.orchestratorTeams.notice.started", "{name} を起動しました", {name: result.session_name}));
            }
            return result;
        } catch (err) {
            setError(toErrorMessage(err, "Failed to start team."));
            throw err;
        }
    }, []);

    const moveTeam = useCallback(async (teamID: string, direction: "up" | "down") => {
        const team = teams.find((t) => t.id === teamID);
        if (!team) return;
        const loc = team.storage_location ?? "global";
        const sameSource = teams.filter((t) => (t.storage_location ?? "global") === loc);
        const index = sameSource.findIndex((t) => t.id === teamID);
        if (direction === "up" && index <= 0) return;
        if (direction === "down" && (index < 0 || index >= sameSource.length - 1)) return;
        const ids = sameSource.map((t) => t.id);
        const swapIndex = direction === "up" ? index - 1 : index + 1;
        [ids[index], ids[swapIndex]] = [ids[swapIndex], ids[index]];
        setError(null);
        try {
            await ReorderOrchestratorTeams(ids, loc, activeSession ?? "");
            await refresh();
        } catch (err) {
            setError(toErrorMessage(err, "Failed to reorder teams."));
            throw err;
        }
    }, [activeSession, teams, refresh, setError]);

    const moveTeamUp = useCallback((teamID: string) => moveTeam(teamID, "up"), [moveTeam]);
    const moveTeamDown = useCallback((teamID: string) => moveTeam(teamID, "down"), [moveTeam]);

    const bootstrapMemberToPane = useCallback(async (request: BootstrapMemberToPaneRequest): Promise<BootstrapMemberToPaneResult> => {
        setError(null);
        setNotice(null);
        try {
            const result = await BootstrapMemberToPane(
                orchestrator.BootstrapMemberToPaneRequest.createFrom(request),
            ) as BootstrapMemberToPaneResult;
            return result;
        } catch (err) {
            setError(toErrorMessage(err, "Failed to bootstrap member."));
            throw err;
        }
    }, []);

    const addMemberToUnaffiliatedTeam = useCallback(async (member: OrchestratorTeamMember, storageLocation: OrchestratorStorageLocation) => {
        setError(null);
        setNotice(null);
        try {
            await AddMemberToUnaffiliatedTeam(
                orchestrator.TeamMember.createFrom(member),
                storageLocation,
                activeSession ?? "",
            );
        } catch (err) {
            setError(toErrorMessage(err, "Failed to add member."));
            throw err;
        }
    }, [activeSession]);

    const saveUnaffiliatedTeamMembers = useCallback(async (draft: OrchestratorTeamDraft) => {
        setError(null);
        setNotice(null);
        try {
            const payload = buildTeamPayload(draft);
            const wireMembers = payload.members.map((m) => orchestrator.TeamMember.createFrom(m));
            await SaveUnaffiliatedTeamMembers(wireMembers, activeSession ?? "");
            await refresh();
        } catch (err) {
            setError(toErrorMessage(err, "Failed to save unaffiliated team members."));
            throw err;
        }
    }, [activeSession, refresh]);

    const ensureUnaffiliatedTeam = useCallback(async (storageLocation: OrchestratorStorageLocation): Promise<OrchestratorTeamDefinition> => {
        const result = await EnsureUnaffiliatedTeam(storageLocation, activeSession ?? "");
        return result as OrchestratorTeamDefinition;
    }, [activeSession]);

    return {
        teams,
        activeSession,
        error,
        notice,
        loading,
        setError,
        setNotice,
        refresh,
        saveTeam,
        deleteTeam,
        startTeam,
        moveTeamUp,
        moveTeamDown,
        bootstrapMemberToPane,
        addMemberToUnaffiliatedTeam,
        saveUnaffiliatedTeamMembers,
        ensureUnaffiliatedTeam,
    };
}
