export type OrchestratorLaunchMode = "active_session" | "new_session";
export type OrchestratorStorageLocation = "global" | "project";

export interface OrchestratorTeamMemberSkill {
    name: string;
    description?: string;
}

export interface OrchestratorTeamMember {
    id: string;
    team_id: string;
    order: number;
    pane_title: string;
    role: string;
    command: string;
    args: string[];
    custom_message: string;
    skills?: OrchestratorTeamMemberSkill[];
}

export interface OrchestratorTeamDefinition {
    id: string;
    name: string;
    description?: string;
    order: number;
    bootstrap_delay_ms?: number;
    storage_location?: OrchestratorStorageLocation;
    members: OrchestratorTeamMember[];
}

export interface StartOrchestratorTeamRequest {
    team_id: string;
    launch_mode: OrchestratorLaunchMode;
    source_session_name: string;
    new_session_name: string;
}

export interface StartOrchestratorTeamResult {
    session_name: string;
    launch_mode: OrchestratorLaunchMode;
    member_pane_ids: Record<string, string>;
    warnings: string[];
}

export interface OrchestratorMemberDraftSkill {
    id: string;
    name: string;
    description: string;
}

export interface OrchestratorMemberDraft {
    id: string;
    paneTitle: string;
    role: string;
    command: string;
    argsText: string;
    customMessage: string;
    skills: OrchestratorMemberDraftSkill[];
}

export interface OrchestratorTeamDraft {
    id: string;
    name: string;
    description: string;
    bootstrapDelayMs: number;
    storageLocation: OrchestratorStorageLocation;
    members: OrchestratorMemberDraft[];
}

export type PaneState = "cli_running" | "cli_not_running";

export interface BootstrapMemberToPaneRequest {
    pane_id: string;
    pane_state: PaneState;
    team_name: string;
    member: OrchestratorTeamMember;
    bootstrap_delay_ms: number;
    session_name: string;
}

export interface BootstrapMemberToPaneResult {
    warnings: string[];
}
