import {useCallback, useEffect, useState} from "react";
import {
    DeleteOrchestratorTeam,
    LoadOrchestratorTeams,
    ReorderOrchestratorTeams,
    SaveOrchestratorTeam,
    StartOrchestratorTeam,
} from "../../../../../wailsjs/go/main/App";
import {translate} from "../../../../i18n";
import {useTmuxStore} from "../../../../stores/tmuxStore";
import type {
    OrchestratorLaunchMode,
    OrchestratorMemberDraft,
    OrchestratorMemberDraftSkill,
    OrchestratorStorageLocation,
    OrchestratorTeamDefinition,
    OrchestratorTeamDraft,
    OrchestratorTeamMember,
    OrchestratorTeamMemberSkill,
    StartOrchestratorTeamRequest,
    StartOrchestratorTeamResult,
} from "./types";

function createDraftID(): string {
    return globalThis.crypto?.randomUUID?.() ?? `draft-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
}

export function parseMemberArgsText(value: string): string[] {
    return value
        .split(/\r?\n/)
        .map((entry) => entry.trim())
        .filter((entry) => entry !== "");
}

export function memberArgsToText(args: string[]): string {
    return args.join("\n");
}

export function createEmptyMemberDraft(): OrchestratorMemberDraft {
    return {
        id: createDraftID(),
        paneTitle: "",
        role: "",
        command: "",
        argsText: "",
        customMessage: "",
        skills: [],
    };
}

const defaultBootstrapDelayMs = 3000;

export function createEmptyTeamDraft(storageLocation: OrchestratorStorageLocation = "global"): OrchestratorTeamDraft {
    return {
        id: createDraftID(),
        name: "",
        description: "",
        bootstrapDelayMs: defaultBootstrapDelayMs,
        storageLocation,
        members: [],
    };
}

function skillsToDraftSkills(skills?: OrchestratorTeamMemberSkill[]): OrchestratorMemberDraftSkill[] {
    if (!skills) {
        return [];
    }
    return skills.map((s) => ({
        id: createDraftID(),
        name: s.name,
        description: s.description ?? "",
    }));
}

export function createTeamDraft(definition: OrchestratorTeamDefinition): OrchestratorTeamDraft {
    return {
        id: definition.id,
        name: definition.name,
        description: definition.description ?? "",
        bootstrapDelayMs: definition.bootstrap_delay_ms ?? defaultBootstrapDelayMs,
        storageLocation: definition.storage_location ?? "global",
        members: definition.members.map((member) => ({
            id: member.id,
            paneTitle: member.pane_title,
            role: member.role,
            command: member.command,
            argsText: memberArgsToText(member.args),
            customMessage: member.custom_message,
            skills: skillsToDraftSkills(member.skills),
        })),
    };
}

export function createCopiedTeamDraft(definition: OrchestratorTeamDefinition): OrchestratorTeamDraft {
    return {
        id: createDraftID(),
        name: `${definition.name} (コピー)`,
        description: definition.description ?? "",
        bootstrapDelayMs: definition.bootstrap_delay_ms ?? defaultBootstrapDelayMs,
        storageLocation: definition.storage_location ?? "global",
        members: definition.members.map((member) => ({
            id: createDraftID(),
            paneTitle: member.pane_title,
            role: member.role,
            command: member.command,
            argsText: memberArgsToText(member.args),
            customMessage: member.custom_message,
            skills: skillsToDraftSkills(member.skills),
        })),
    };
}

export function copyMemberToDraft(member: OrchestratorTeamMember): OrchestratorMemberDraft {
    return {
        id: createDraftID(),
        paneTitle: member.pane_title,
        role: member.role,
        command: member.command,
        argsText: memberArgsToText(member.args),
        customMessage: member.custom_message,
        skills: skillsToDraftSkills(member.skills),
    };
}

export function isTeamNameDuplicate(
    name: string,
    currentTeamID: string,
    teams: OrchestratorTeamDefinition[],
    storageLocation: OrchestratorStorageLocation = "global",
): boolean {
    const trimmed = name.trim();
    if (trimmed === "") return false;
    return teams.some(
        (t) => t.id !== currentTeamID && t.name.trim() === trimmed && (t.storage_location ?? "global") === storageLocation,
    );
}

export function isMemberDraftValid(draft: OrchestratorMemberDraft): boolean {
    return draft.paneTitle.trim() !== "" && draft.role.trim() !== "" && draft.command.trim() !== "";
}

export function isTeamDraftValid(draft: OrchestratorTeamDraft): boolean {
    if (draft.name.trim() === "") {
        return false;
    }
    return draft.members.every(isMemberDraftValid);
}

function draftSkillsToSkills(draftSkills: OrchestratorMemberDraftSkill[]): OrchestratorTeamMemberSkill[] {
    return draftSkills
        .filter((s) => s.name.trim() !== "")
        .map((s) => ({
            name: s.name.trim(),
            description: s.description.trim(),
        }));
}

export function buildTeamPayload(draft: OrchestratorTeamDraft): OrchestratorTeamDefinition {
    const members: OrchestratorTeamMember[] = draft.members.map((member, index) => ({
        id: member.id,
        team_id: draft.id,
        order: index,
        pane_title: member.paneTitle.trim(),
        role: member.role.trim(),
        command: member.command.trim(),
        args: parseMemberArgsText(member.argsText),
        custom_message: member.customMessage.trim(),
        skills: draftSkillsToSkills(member.skills),
    }));

    return {
        id: draft.id,
        name: draft.name.trim(),
        description: draft.description.trim(),
        order: 0,
        bootstrap_delay_ms: draft.bootstrapDelayMs,
        storage_location: draft.storageLocation,
        members,
    };
}

export function buildStartRequest(
    teamID: string,
    activeSession: string | null,
    launchMode: OrchestratorLaunchMode,
    newSessionName: string,
): StartOrchestratorTeamRequest {
    return {
        team_id: teamID,
        launch_mode: launchMode,
        source_session_name: activeSession?.trim() ?? "",
        new_session_name: launchMode === "new_session" ? newSessionName.trim() : "",
    };
}

export function upsertDraftMember(
    draft: OrchestratorTeamDraft,
    memberDraft: OrchestratorMemberDraft,
    index: number | null,
): OrchestratorTeamDraft {
    const members = [...draft.members];
    if (index === null || index < 0 || index >= members.length) {
        members.push(memberDraft);
    } else {
        members[index] = memberDraft;
    }
    return {
        ...draft,
        members,
    };
}

export function removeDraftMember(draft: OrchestratorTeamDraft, memberID: string): OrchestratorTeamDraft {
    return {
        ...draft,
        members: draft.members.filter((member) => member.id !== memberID),
    };
}

export function useOrchestratorTeams() {
    const activeSession = useTmuxStore((state) => state.activeSession);
    const [teams, setTeams] = useState<OrchestratorTeamDefinition[]>([]);
    const [error, setError] = useState<string | null>(null);
    const [notice, setNotice] = useState<string | null>(null);
    const [loading, setLoading] = useState(true);

    const refresh = useCallback(async () => {
        setLoading(true);
        try {
            const result = await LoadOrchestratorTeams(activeSession ?? "");
            setTeams((result ?? []) as OrchestratorTeamDefinition[]);
        } catch (err) {
            console.warn("[orchestrator-teams] failed to load teams", err);
            setError(String(err));
        } finally {
            setLoading(false);
        }
    }, [activeSession]);

    useEffect(() => {
        void refresh();
    }, [refresh]);

    const saveTeam = useCallback(async (draft: OrchestratorTeamDraft) => {
        setError(null);
        setNotice(null);
        try {
            await SaveOrchestratorTeam(buildTeamPayload(draft) as never, activeSession ?? "");
            await refresh();
        } catch (err) {
            setError(String(err));
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
            setError(String(err));
            throw err;
        }
    }, [activeSession, refresh]);

    const startTeam = useCallback(async (request: StartOrchestratorTeamRequest): Promise<StartOrchestratorTeamResult> => {
        setError(null);
        setNotice(null);
        try {
            const result = await StartOrchestratorTeam(request as never) as StartOrchestratorTeamResult;
            if (result.warnings.length > 0) {
                const prefix = translate("viewer.orchestratorTeams.notice.startedWithWarnings", "{name} を起動しました（警告あり）:", {name: result.session_name});
                setNotice(`${prefix}\n${result.warnings.join("\n")}`);
            } else {
                setNotice(translate("viewer.orchestratorTeams.notice.started", "{name} を起動しました", {name: result.session_name}));
            }
            return result;
        } catch (err) {
            setError(String(err));
            throw err;
        }
    }, []);

    const moveTeamUp = useCallback(async (teamID: string) => {
        const team = teams.find((t) => t.id === teamID);
        if (!team) return;
        const loc = team.storage_location ?? "global";
        const sameSource = teams.filter((t) => (t.storage_location ?? "global") === loc);
        const index = sameSource.findIndex((t) => t.id === teamID);
        if (index <= 0) return;
        const ids = sameSource.map((t) => t.id);
        [ids[index - 1], ids[index]] = [ids[index], ids[index - 1]];
        setError(null);
        try {
            await ReorderOrchestratorTeams(ids, loc, activeSession ?? "");
            await refresh();
        } catch (err) {
            setError(String(err));
        }
    }, [activeSession, teams, refresh, setError]);

    const moveTeamDown = useCallback(async (teamID: string) => {
        const team = teams.find((t) => t.id === teamID);
        if (!team) return;
        const loc = team.storage_location ?? "global";
        const sameSource = teams.filter((t) => (t.storage_location ?? "global") === loc);
        const index = sameSource.findIndex((t) => t.id === teamID);
        if (index < 0 || index >= sameSource.length - 1) return;
        const ids = sameSource.map((t) => t.id);
        [ids[index], ids[index + 1]] = [ids[index + 1], ids[index]];
        setError(null);
        try {
            await ReorderOrchestratorTeams(ids, loc, activeSession ?? "");
            await refresh();
        } catch (err) {
            setError(String(err));
        }
    }, [activeSession, teams, refresh, setError]);

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
    };
}
