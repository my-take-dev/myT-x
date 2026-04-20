import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {useCanvasStore} from "../../stores/canvasStore";
import {TaskTimelinePanel} from "./TaskTimelinePanel";

const getOrchestratorTaskDetailMock = vi.fn();

vi.mock("../../api", () => ({
    api: {
        GetOrchestratorTaskDetail: (...args: unknown[]) => getOrchestratorTaskDetailMock(...args),
    },
}));

vi.mock("../../utils/notifyUtils", () => ({
    notifyAndLog: vi.fn(),
}));

describe("TaskTimelinePanel", () => {
    let container: HTMLDivElement;
    let root: Root;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
        useCanvasStore.setState({
            taskEdgeMap: {
                "t-001": {
                    task_id: "t-001",
                    agent_name: "worker",
                    assignee_pane_id: "%1",
                    sender_pane_id: "%0",
                    sender_name: "orchestrator",
                    status: "completed",
                    sent_at: "2026-04-18T01:02:03Z",
                    completed_at: "2026-04-18T01:05:03Z",
                    message_preview: "request preview",
                    response_preview: "response preview",
                },
            },
        });
        getOrchestratorTaskDetailMock.mockReset();
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        container.remove();
        useCanvasStore.setState({taskEdgeMap: {}});
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = false;
    });

    it("renders stored payload metadata for message and response details", async () => {
        getOrchestratorTaskDetailMock.mockResolvedValue({
            task_id: "t-001",
            agent_name: "worker",
            sender_name: "orchestrator",
            status: "completed",
            sent_at: "2026-04-18T01:02:03Z",
            completed_at: "2026-04-18T01:05:03Z",
            message_content: "",
            message_preview: "request preview",
            message_storage_mode: "multipart_file",
            message_artifact_paths: [
                "C:\\project\\.myT-x\\orchestrator\\payloads\\t-001__m-001__manifest.json",
                "C:\\project\\.myT-x\\orchestrator\\payloads\\t-001__m-001__p001-of002.md",
            ],
            message_part_count: 2,
            message_content_chars: 32001,
            message_sha256: "message-sha",
            response_content: "",
            response_preview: "response preview",
            response_storage_mode: "file",
            response_artifact_paths: [
                "C:\\project\\.myT-x\\orchestrator\\payloads\\t-001__r-001.md",
            ],
            response_part_count: 1,
            response_content_chars: 16001,
            response_sha256: "response-sha",
        });

        act(() => {
            root.render(<TaskTimelinePanel sessionName="demo" onClose={vi.fn()} />);
        });

        const ticket = container.querySelector(".canvas-timeline-ticket");
        expect(ticket).not.toBeNull();

        await act(async () => {
            ticket?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
            await Promise.resolve();
        });

        const text = container.textContent ?? "";
        expect(getOrchestratorTaskDetailMock).toHaveBeenCalledWith("demo", "t-001");
        expect(text).toContain("request preview");
        expect(text).toContain("storage: multipart_file");
        expect(text).toContain("chars: 32001");
        expect(text).toContain("parts: 2");
        expect(text).toContain("message-sha");
        expect(text).toContain("t-001__m-001__manifest.json");
        expect(text).toContain("response preview");
        expect(text).toContain("storage: file");
        expect(text).toContain("response-sha");
        expect(text).toContain("t-001__r-001.md");
    });
});
