export interface OrchestratorTask {
    task_id: string;
    agent_name: string;
    sender_pane_id: string;
    assignee_pane_id: string;
    sender_name: string;
    status: string;
    sent_at: string;
    completed_at: string;
}

export interface OrchestratorAgent {
    name: string;
    pane_id: string;
    role: string;
}

export interface PaneProcessStatus {
    pane_id: string;
    has_child_process: boolean;
}

export interface CanvasNodePosition {
    x: number;
    y: number;
}

export interface CanvasNodeSize {
    width: number;
    height: number;
}
