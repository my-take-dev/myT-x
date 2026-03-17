import {describe, expect, it} from "vitest";
import {
    buildStartRequest,
    buildTeamPayload,
    createEmptyTeamDraft,
    parseMemberArgsText,
    removeDraftMember,
    upsertDraftMember,
} from "../src/components/viewer/views/orchestrator-teams/useOrchestratorTeams";
import type {OrchestratorMemberDraft} from "../src/components/viewer/views/orchestrator-teams/types";

describe("orchestrator team helpers", () => {
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
});
