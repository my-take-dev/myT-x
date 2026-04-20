import {useEffect, useMemo, useState} from "react";
import {useI18n} from "../../i18n";
import type {PaneSnapshot} from "../../types/tmux";
import type {
    EnlistPaneRequest,
    EnlistPaneResult,
    OrchestratorMemberDraft,
    OrchestratorStorageLocation,
    OrchestratorSessionEnlistmentContext,
    OrchestratorTeamDefinition,
    OrchestratorTeamMember,
    PaneState,
} from "../viewer/views/orchestrator-teams/types";
import {
    buildTeamPayload,
    copyMemberToDraft,
    createEmptyMemberDraft,
    DEFAULT_BOOTSTRAP_DELAY_MS,
    isMemberDraftValid,
    isSystemTeam,
    UNAFFILIATED_TEAM_ID,
} from "../viewer/views/orchestrator-teams/orchestratorTeamUtils";
import {MemberEditor} from "../viewer/views/orchestrator-teams/MemberEditor";
import {useBootstrapDelayInput} from "../viewer/views/orchestrator-teams/useBootstrapDelayInput";
import {toErrorMessage} from "../../utils/errorUtils";

type EnlistMode = "new" | "copy" | "quick";

interface EnlistPaneModalProps {
    open: boolean;
    sessionName: string;
    pane: PaneSnapshot | null;
    parentPane: PaneSnapshot | null;
    context: OrchestratorSessionEnlistmentContext | null;
    suggestedTeamID: string | null;
    suggestedStorageLocation: OrchestratorStorageLocation;
    suggestedRole: string;
    onClose: () => void;
    onEnlist: (request: EnlistPaneRequest) => Promise<EnlistPaneResult>;
}

interface SourceMemberOption {
    key: string;
    team: OrchestratorTeamDefinition;
    member: OrchestratorTeamMember;
}

function normalizeTeamStorageLocation(team: OrchestratorTeamDefinition): OrchestratorStorageLocation {
    return team.storage_location === "project" ? "project" : "global";
}

function encodeTeamValue(teamID: string, storageLocation: OrchestratorStorageLocation): string {
    return `${teamID}::${storageLocation}`;
}

function decodeTeamValue(value: string): { teamID: string; storageLocation: OrchestratorStorageLocation } {
    const [teamID = "", storage = "global"] = value.split("::", 2);
    return {
        teamID,
        storageLocation: storage === "project" ? "project" : "global",
    };
}

function findTeamValue(
    teams: OrchestratorTeamDefinition[],
    suggestedTeamID: string | null,
    suggestedStorageLocation: OrchestratorStorageLocation,
): string {
    if (suggestedTeamID) {
        const suggested = teams.find((team) => {
            if (team.id !== suggestedTeamID) {
                return false;
            }
            return normalizeTeamStorageLocation(team) === suggestedStorageLocation;
        });
        if (suggested) {
            return encodeTeamValue(suggested.id, normalizeTeamStorageLocation(suggested));
        }
    }

    const unaffiliated = teams.find((team) => team.id === UNAFFILIATED_TEAM_ID && normalizeTeamStorageLocation(team) === suggestedStorageLocation)
        ?? teams.find((team) => team.id === UNAFFILIATED_TEAM_ID)
        ?? teams[0];
    if (!unaffiliated) {
        return "";
    }
    return encodeTeamValue(unaffiliated.id, normalizeTeamStorageLocation(unaffiliated));
}

export function EnlistPaneModal({
    open,
    sessionName,
    pane,
    parentPane,
    context,
    suggestedTeamID,
    suggestedStorageLocation,
    suggestedRole,
    onClose,
    onEnlist,
}: EnlistPaneModalProps) {
    const {t} = useI18n();
    const [mode, setMode] = useState<EnlistMode>("new");
    const [selectedTeamValue, setSelectedTeamValue] = useState("");
    const [draft, setDraft] = useState<OrchestratorMemberDraft>(createEmptyMemberDraft());
    const [selectedSourceKey, setSelectedSourceKey] = useState<string | null>(null);
    const [paneState, setPaneState] = useState<PaneState>("cli_running");
    const [saving, setSaving] = useState(false);
    const [errorText, setErrorText] = useState<string | null>(null);
    const [warningText, setWarningText] = useState<string | null>(null);

    const bootstrapDelay = useBootstrapDelayInput({enabled: paneState === "cli_not_running"});

    const buildInitialDraft = () => {
        const initialDraft = createEmptyMemberDraft();
        initialDraft.paneTitle = pane?.title?.trim() ?? "";
        initialDraft.role = suggestedRole;
        return initialDraft;
    };

    useEffect(() => {
        if (!open || !pane || !context) {
            return;
        }
        setMode("new");
        setSelectedTeamValue(findTeamValue(context.teams, suggestedTeamID, suggestedStorageLocation));
        setDraft(buildInitialDraft());
        setSelectedSourceKey(null);
        setPaneState("cli_running");
        setSaving(false);
        setErrorText(null);
        setWarningText(null);
    }, [context, open, pane, suggestedRole, suggestedStorageLocation, suggestedTeamID]);

    const teamOptions = useMemo(() => context?.teams ?? [], [context]);

    const copyCandidates = useMemo<SourceMemberOption[]>(() => {
        if (!context) {
            return [];
        }
        return context.teams.flatMap((team) => team.members.map((member) => ({
            key: `${team.id}:${normalizeTeamStorageLocation(team)}:${member.id}`,
            team,
            member,
        })));
    }, [context]);

    const quickCandidates = useMemo<SourceMemberOption[]>(
        () => copyCandidates.filter((candidate) => isSystemTeam(candidate.team)),
        [copyCandidates],
    );

    const selectedSource = useMemo(() => {
        const sourceList = mode === "quick" ? quickCandidates : copyCandidates;
        return sourceList.find((candidate) => candidate.key === selectedSourceKey) ?? null;
    }, [copyCandidates, mode, quickCandidates, selectedSourceKey]);

    useEffect(() => {
        if (!selectedSource) {
            return;
        }
        const nextDraft = copyMemberToDraft(selectedSource.member);
        if ((pane?.title?.trim() ?? "") !== "") {
            nextDraft.paneTitle = pane?.title?.trim() ?? nextDraft.paneTitle;
        }
        setDraft(nextDraft);
        if (mode === "copy") {
            setSelectedTeamValue(encodeTeamValue(selectedSource.team.id, normalizeTeamStorageLocation(selectedSource.team)));
        }
    }, [mode, pane?.title, selectedSource]);

    const selectedTeam = useMemo(() => {
        const {teamID, storageLocation} = decodeTeamValue(selectedTeamValue);
        return teamOptions.find((team) => team.id === teamID && normalizeTeamStorageLocation(team) === storageLocation) ?? null;
    }, [selectedTeamValue, teamOptions]);

    const existingPaneTitles = useMemo(() => {
        if (!selectedTeam) {
            return [];
        }
        return selectedTeam.members.map((member) => member.pane_title.trim()).filter((title) => title !== "");
    }, [selectedTeam]);

    const canSubmit = pane !== null
        && context !== null
        && selectedTeam !== null
        && isMemberDraftValid(draft)
        && bootstrapDelay.isValid
        && (mode !== "quick" || selectedSource !== null);

    const handleSubmit = async () => {
        if (!pane || !selectedTeam) {
            return;
        }
        const payload = buildTeamPayload({
            id: selectedTeam.id,
            name: selectedTeam.name,
            description: selectedTeam.description ?? "",
            bootstrapDelayMs: selectedTeam.bootstrap_delay_ms ?? DEFAULT_BOOTSTRAP_DELAY_MS,
            storageLocation: normalizeTeamStorageLocation(selectedTeam),
            members: [draft],
        });
        const wireMember = payload.members[0];
        setSaving(true);
        setErrorText(null);
        setWarningText(null);
        try {
            const result = await onEnlist({
                session_name: sessionName,
                pane_id: pane.id,
                team_id: selectedTeam.id,
                storage_location: normalizeTeamStorageLocation(selectedTeam),
                pane_state: paneState,
                bootstrap_delay_ms: bootstrapDelay.delayMs,
                member: wireMember,
            });
            if (result.warnings.length > 0) {
                setWarningText(result.warnings.join("\n"));
                return;
            }
            onClose();
        } catch (err: unknown) {
            setErrorText(toErrorMessage(err, "Failed to enlist pane."));
        } finally {
            setSaving(false);
        }
    };

    if (!open || pane === null) {
        return null;
    }

    return (
        <div className="modal-overlay" onClick={onClose}>
            <div className="modal-panel orchestrator-enlist-modal" role="dialog" aria-modal="true" onClick={(event) => event.stopPropagation()}>
                <div className="modal-header">
                    <h2>{t("viewer.orchestratorTeams.enlist.title", "Enlist Pane")}</h2>
                </div>
                <div className="modal-body orchestrator-enlist-modal-body">
                    {errorText && (
                        <div className="orchestrator-teams-banner error">
                            <span>{errorText}</span>
                            <button type="button" onClick={() => setErrorText(null)}>
                                {t("viewer.orchestratorTeams.dismiss", "閉じる")}
                            </button>
                        </div>
                    )}
                    {warningText && (
                        <div className="orchestrator-teams-banner notice">
                            <span>{warningText}</span>
                            <button type="button" onClick={() => setWarningText(null)}>
                                {t("viewer.orchestratorTeams.dismiss", "Dismiss")}
                            </button>
                        </div>
                    )}
                    <section className="orchestrator-enlist-section">
                        <div className="orchestrator-team-card-tag">
                            {t("viewer.orchestratorTeams.enlist.target", "Target Pane")}
                        </div>
                        <div className="orchestrator-enlist-target-grid">
                            <div><strong>ID</strong><span>{pane.id}</span></div>
                            <div><strong>Index</strong><span>{pane.index}</span></div>
                            <div><strong>Title</strong><span>{pane.title?.trim() || "-"}</span></div>
                            <div><strong>Parent</strong><span>{parentPane?.id ?? "-"}</span></div>
                        </div>
                    </section>

                    <section className="orchestrator-enlist-section">
                        <label className="form-label" htmlFor="enlist-team-select">
                            {t("viewer.orchestratorTeams.enlist.team", "Destination Team")}
                        </label>
                        <select
                            id="enlist-team-select"
                            className="form-input"
                            value={selectedTeamValue}
                            onChange={(event) => setSelectedTeamValue(event.target.value)}
                        >
                            {teamOptions.map((team) => {
                                const storageLocation = normalizeTeamStorageLocation(team);
                                return (
                                    <option
                                        key={`${team.id}-${storageLocation}`}
                                        value={encodeTeamValue(team.id, storageLocation)}
                                    >
                                        {team.name} ({storageLocation})
                                    </option>
                                );
                            })}
                        </select>
                    </section>

                    <section className="orchestrator-enlist-section">
                        <div className="form-label">{t("viewer.orchestratorTeams.enlist.mode", "Registration Mode")}</div>
                        <div className="orchestrator-enlist-mode-row">
                            {(["new", "copy", "quick"] as const).map((value) => (
                                <button
                                    key={value}
                                    type="button"
                                    className={`orchestrator-team-card-btn ${mode === value ? "active" : ""}`}
                                    onClick={() => {
                                        setMode(value);
                                        setSelectedSourceKey(null);
                                        if (value === "new") {
                                            setDraft(buildInitialDraft());
                                        }
                                    }}
                                >
                                    {value === "new"
                                        ? t("viewer.orchestratorTeams.addTermMemberSource.newMember", "Create new member")
                                        : value === "copy"
                                            ? t("viewer.orchestratorTeams.addTermMemberSource.copyMember", "Copy from existing member")
                                            : t("viewer.orchestratorTeams.addTermMemberSource.quickStart", "Start with unaffiliated member")}
                                </button>
                            ))}
                        </div>
                    </section>

                    {(mode === "copy" || mode === "quick") && (
                        <section className="orchestrator-enlist-section">
                            <div className="form-label">
                                {mode === "copy"
                                    ? t("viewer.orchestratorTeams.enlist.template", "Member Template")
                                    : t("viewer.orchestratorTeams.enlist.quickTemplate", "Unaffiliated Template")}
                            </div>
                            <div className="orchestrator-teams-cards orchestrator-enlist-source-list">
                                {(mode === "quick" ? quickCandidates : copyCandidates).map((candidate) => (
                                    <button
                                        key={candidate.key}
                                        type="button"
                                        className={`orchestrator-team-card ${selectedSourceKey === candidate.key ? "selected" : ""}`}
                                        onClick={() => setSelectedSourceKey(candidate.key)}
                                    >
                                        <span className="orchestrator-team-card-title">{candidate.member.pane_title}</span>
                                        <span className="orchestrator-team-card-meta">{candidate.member.role}</span>
                                        <span className="orchestrator-team-card-tag">{candidate.team.name}</span>
                                    </button>
                                ))}
                            </div>
                        </section>
                    )}

                    <section className="orchestrator-enlist-section">
                        <MemberEditor
                            draft={draft}
                            existingPaneTitles={existingPaneTitles.filter((title) => title !== draft.paneTitle.trim())}
                            onChange={setDraft}
                            onBack={() => {}}
                            onSave={() => {}}
                            hideNavigation
                            roleSuggestions={context?.role_catalog ?? []}
                            skillSuggestions={context?.skill_catalog ?? []}
                        />
                    </section>

                    <section className="orchestrator-enlist-section">
                        <div className="form-group">
                            <label className="form-label">{t("viewer.orchestratorTeams.addTermMember.paneState", "Pane State")}</label>
                            <div className="orchestrator-teams-start-grid">
                                <div className={`orchestrator-teams-mode-card${paneState === "cli_running" ? " selected" : ""}`}>
                                    <label>
                                        <input
                                            type="radio"
                                            name="enlist-pane-state"
                                            checked={paneState === "cli_running"}
                                            onChange={() => setPaneState("cli_running")}
                                        />
                                        <span>{t("viewer.orchestratorTeams.addTermMember.cliRunning", "CLI already running")}</span>
                                    </label>
                                </div>
                                <div className={`orchestrator-teams-mode-card${paneState === "cli_not_running" ? " selected" : ""}`}>
                                    <label>
                                        <input
                                            type="radio"
                                            name="enlist-pane-state"
                                            checked={paneState === "cli_not_running"}
                                            onChange={() => setPaneState("cli_not_running")}
                                        />
                                        <span>{t("viewer.orchestratorTeams.addTermMember.cliNotRunning", "CLI not running")}</span>
                                    </label>
                                </div>
                            </div>
                        </div>
                        {paneState === "cli_not_running" && (
                            <div className="form-group">
                                <label className="form-label">{t("viewer.orchestratorTeams.addTermMember.bootstrapDelay", "Bootstrap delay (seconds)")}</label>
                                <input
                                    className="form-input"
                                    type="text"
                                    inputMode="numeric"
                                    value={bootstrapDelay.delaySec}
                                    onChange={bootstrapDelay.handleChange}
                                />
                            </div>
                        )}
                    </section>
                </div>
                <div className="modal-footer">
                    <button type="button" className="modal-btn" onClick={onClose}>
                        {t("common.cancel", "Cancel")}
                    </button>
                    <button
                        type="button"
                        className="modal-btn primary"
                        disabled={!canSubmit || saving}
                        onClick={() => {
                            void handleSubmit();
                        }}
                    >
                        {saving
                            ? t("viewer.orchestratorTeams.enlist.saving", "Registering...")
                            : t("viewer.orchestratorTeams.enlist.submit", "Register Pane")}
                    </button>
                </div>
            </div>
        </div>
    );
}
