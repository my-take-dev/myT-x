import type {main} from "../../wailsjs/go/models";

// Re-export Wails-generated types to avoid duplicate definitions.
// Use these types throughout canvas-related code.
export type OrchestratorTask = main.OrchestratorTask;
export type OrchestratorAgent = main.OrchestratorAgent;
export type PaneProcessStatus = main.PaneProcessStatus;

export interface CanvasNodePosition {
    x: number;
    y: number;
}

export interface CanvasNodeSize {
    width: number;
    height: number;
}
