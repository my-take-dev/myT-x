export interface TaskSchedulerTemplateViewContext {
    readonly kind: "task-scheduler-template";
    readonly key: string;
    readonly name: string;
    readonly message: string;
    readonly targetPaneID: string;
    readonly clearBefore: boolean;
    readonly clearCommand: string;
}

export interface OrchestratorTeamsDefaultViewContext {
    readonly kind: "orchestrator-teams-default";
}

export interface OrchestratorTeamsAddTermMemberViewContext {
    readonly kind: "orchestrator-teams-add-term-member";
    readonly addTermMemberPaneId: string;
}

export type OrchestratorTeamsViewContext =
    | OrchestratorTeamsDefaultViewContext
    | OrchestratorTeamsAddTermMemberViewContext;

export interface ViewerContextMap {
    readonly "task-scheduler": TaskSchedulerTemplateViewContext;
    readonly "orchestrator-teams": OrchestratorTeamsViewContext;
}

export type ViewerContext = ViewerContextMap[keyof ViewerContextMap];

export type OpenViewWithContext = <TViewId extends keyof ViewerContextMap>(
    viewId: TViewId,
    context: ViewerContextMap[TViewId],
) => void;
