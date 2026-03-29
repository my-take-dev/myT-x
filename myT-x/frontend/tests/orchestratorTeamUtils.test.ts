import {describe, expect, it} from "vitest";
import {
    bootstrapDelaySecToMs,
    buildStartRequest,
    buildTeamPayload,
    createEmptyTeamDraft,
    formatBootstrapDelaySec,
    getBootstrapDelayValidationError,
    isBootstrapDelayMsValid,
    isSystemTeam,
    isTeamDraftValid,
    MAX_BOOTSTRAP_DELAY_MS,
    MIN_BOOTSTRAP_DELAY_MS,
    normalizeBootstrapDelaySecInput,
    parseMemberArgsText,
    removeDraftMember,
    UNAFFILIATED_TEAM_ID,
    upsertDraftMember,
} from "../src/components/viewer/views/orchestrator-teams/orchestratorTeamUtils";
import type {OrchestratorMemberDraft, OrchestratorTeamDefinition} from "../src/components/viewer/views/orchestrator-teams/types";

describe("orchestrator team helpers", () => {
    it("returns true for the unaffiliated team", () => {
        expect(isSystemTeam(UNAFFILIATED_TEAM_ID)).toBe(true);

        const team: OrchestratorTeamDefinition = {
            id: UNAFFILIATED_TEAM_ID,
            name: "無所属",
            order: 0,
            members: [],
        };

        expect(isSystemTeam(team)).toBe(true);
    });

    it("returns false for regular teams", () => {
        expect(isSystemTeam("550e8400-e29b-41d4-a716-446655440000")).toBe(false);
        expect(isSystemTeam("")).toBe(false);

        const team: OrchestratorTeamDefinition = {
            id: "some-uuid",
            name: "Regular Team",
            order: 0,
            members: [],
        };

        expect(isSystemTeam(team)).toBe(false);
    });

    it("parses one argument per line and removes blanks", () => {
        expect(parseMemberArgsText(" --sandbox \n\n workspace-write \n")).toEqual(["--sandbox", "workspace-write"]);
    });

    it("builds a payload with normalized fields and member order", () => {
        const draft = {
            id: "team-1",
            name: " Delivery ",
            description: "",
            bootstrapDelayMs: 0,
            storageLocation: "project" as const,
            members: [
                {
                    id: "member-1",
                    paneTitle: " Lead ",
                    role: " Lead engineer ",
                    command: " codex ",
                    argsText: " --sandbox \n workspace-write ",
                    customMessage: " Coordinate the team ",
                    skills: [],
                },
            ],
        };

        expect(buildTeamPayload(draft)).toEqual({
            id: "team-1",
            name: "Delivery",
            description: "",
            order: 0,
            bootstrap_delay_ms: 0,
            storage_location: "project",
            members: [
                {
                    id: "member-1",
                    team_id: "team-1",
                    order: 0,
                    pane_title: "Lead",
                    role: "Lead engineer",
                    command: "codex",
                    args: ["--sandbox", "workspace-write"],
                    custom_message: "Coordinate the team",
                    skills: [],
                },
            ],
        });
    });

    it("builds a start request for active and new session launches", () => {
        expect(buildStartRequest("team-1", "active-a", "active_session", "ignored")).toEqual({
            team_id: "team-1",
            launch_mode: "active_session",
            source_session_name: "active-a",
            new_session_name: "",
        });

        expect(buildStartRequest("team-1", "active-a", "new_session", " release-team ")).toEqual({
            team_id: "team-1",
            launch_mode: "new_session",
            source_session_name: "active-a",
            new_session_name: "release-team",
        });
    });

    it("upserts and removes draft members", () => {
        const empty = createEmptyTeamDraft();
        const memberA: OrchestratorMemberDraft = {
            id: "a",
            paneTitle: "Lead",
            role: "Lead",
            command: "codex",
            argsText: "",
            customMessage: "",
        };
        const memberB: OrchestratorMemberDraft = {
            ...memberA,
            id: "b",
            paneTitle: "Reviewer",
        };

        const withMembers = upsertDraftMember(upsertDraftMember(empty, memberA, null), memberB, null);
        expect(withMembers.members.map((member) => member.id)).toEqual(["a", "b"]);

        const updated = upsertDraftMember(withMembers, {...memberB, role: "Review"}, 1);
        expect(updated.members[1].role).toBe("Review");

        const removed = removeDraftMember(updated, "a");
        expect(removed.members.map((member) => member.id)).toEqual(["b"]);
    });

    it("normalizes and parses bootstrap delay input", () => {
        expect(normalizeBootstrapDelaySecInput("007")).toBe("7");
        expect(normalizeBootstrapDelaySecInput("")).toBe("");
        expect(normalizeBootstrapDelaySecInput("7a")).toBeNull();
        expect(bootstrapDelaySecToMs("30")).toBe(30000);
        expect(bootstrapDelaySecToMs("")).toBe(0);
    });

    it("formats bootstrap delay values as integer seconds", () => {
        expect(formatBootstrapDelaySec(1500)).toBe("2");
        expect(formatBootstrapDelaySec(3000)).toBe("3");
    });

    it("validates bootstrap delay bounds", () => {
        expect(getBootstrapDelayValidationError(MIN_BOOTSTRAP_DELAY_MS)).toBeNull();
        expect(getBootstrapDelayValidationError(MIN_BOOTSTRAP_DELAY_MS - 1)).toBe("min");
        expect(getBootstrapDelayValidationError(MAX_BOOTSTRAP_DELAY_MS + 1)).toBe("max");
        expect(isBootstrapDelayMsValid(0, false)).toBe(true);
    });

    it("rejects team drafts outside the bootstrap delay bounds", () => {
        const baseDraft = {
            ...createEmptyTeamDraft(),
            name: "Release",
        };

        expect(isTeamDraftValid({...baseDraft, bootstrapDelayMs: MIN_BOOTSTRAP_DELAY_MS})).toBe(true);
        expect(isTeamDraftValid({...baseDraft, bootstrapDelayMs: MAX_BOOTSTRAP_DELAY_MS})).toBe(true);
        expect(isTeamDraftValid({...baseDraft, bootstrapDelayMs: MIN_BOOTSTRAP_DELAY_MS - 1})).toBe(false);
        expect(isTeamDraftValid({...baseDraft, bootstrapDelayMs: MAX_BOOTSTRAP_DELAY_MS + 1})).toBe(false);
    });
});
