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
} from "./types";

export const UNAFFILIATED_TEAM_ID = "__unaffiliated__";
export const UNAFFILIATED_TEAM_NAME = "無所属";
export const DEFAULT_BOOTSTRAP_DELAY_MS = 3000;
export const MIN_BOOTSTRAP_DELAY_MS = 1000;
export const MAX_BOOTSTRAP_DELAY_MS = 30000;

export type BootstrapDelayValidationError = "min" | "max" | null;

export function isSystemTeam(teamOrId: OrchestratorTeamDefinition | string): boolean {
    const id = typeof teamOrId === "string" ? teamOrId : teamOrId.id;
    return id === UNAFFILIATED_TEAM_ID;
}

export function normalizeBootstrapDelaySecInput(value: string): string | null {
    if (value === "") {
        return "";
    }
    if (!/^\d+$/.test(value)) {
        return null;
    }
    return value.replace(/^0+(?=\d)/, "");
}

export function bootstrapDelaySecToMs(value: string): number {
    return value === "" ? 0 : Number(value) * 1000;
}

export function formatBootstrapDelaySec(delayMs: number): string {
    if (!Number.isFinite(delayMs)) {
        return String(DEFAULT_BOOTSTRAP_DELAY_MS / 1000);
    }
    return String(Math.max(0, Math.round(delayMs / 1000)));
}

export function getBootstrapDelayValidationError(
    delayMs: number,
    enabled = true,
): BootstrapDelayValidationError {
    if (!enabled) {
        return null;
    }
    if (delayMs < MIN_BOOTSTRAP_DELAY_MS) {
        return "min";
    }
    if (delayMs > MAX_BOOTSTRAP_DELAY_MS) {
        return "max";
    }
    return null;
}

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

export function createEmptyTeamDraft(storageLocation: OrchestratorStorageLocation = "global"): OrchestratorTeamDraft {
    return {
        id: createDraftID(),
        name: "",
        description: "",
        bootstrapDelayMs: DEFAULT_BOOTSTRAP_DELAY_MS,
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
        bootstrapDelayMs: definition.bootstrap_delay_ms ?? DEFAULT_BOOTSTRAP_DELAY_MS,
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
        bootstrapDelayMs: definition.bootstrap_delay_ms ?? DEFAULT_BOOTSTRAP_DELAY_MS,
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

export function isBootstrapDelayMsValid(delayMs: number, enabled = true): boolean {
    return getBootstrapDelayValidationError(delayMs, enabled) === null;
}

export function isTeamDraftValid(draft: OrchestratorTeamDraft): boolean {
    if (draft.name.trim() === "") {
        return false;
    }
    if (!isBootstrapDelayMsValid(draft.bootstrapDelayMs)) {
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
