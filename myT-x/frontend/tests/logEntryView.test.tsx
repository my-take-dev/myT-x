import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {LogEntryView} from "../src/components/viewer/views/shared/LogEntryView";

interface TestEntry {
    readonly seq: number;
    readonly message: string;
}

const renderEntry = (entry: TestEntry) => <span>{entry.message}</span>;

describe("LogEntryView", () => {
    let container: HTMLDivElement;
    let root: Root;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        container.remove();
        vi.restoreAllMocks();
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = false;
    });

    it("disables copy-all button when entries are empty", () => {
        const copyAll = vi.fn(async () => true);

        act(() => {
            root.render(
                <LogEntryView<TestEntry>
                    className="test-log-view"
                    title="Test Log"
                    entries={[]}
                    renderEntry={renderEntry}
                    copyAll={copyAll}
                    copyEntry={vi.fn(async () => true)}
                    markAllRead={vi.fn()}
                    registerBodyElement={vi.fn()}
                    onClose={vi.fn()}
                    emptyMessage="No entries"
                    bodyClassName="test-log-body"
                    emptyClassName="test-log-empty"
                    entryClassName="test-log-entry"
                    logPrefix="[test-log]"
                />,
            );
        });

        const copyAllButton = container.querySelector('button[aria-label="Copy all"]') as HTMLButtonElement | null;
        expect(copyAllButton).toBeTruthy();
        expect(copyAllButton?.disabled).toBe(true);
    });

    it("calls copyEntry when pressing Enter on an entry row", async () => {
        const entry: TestEntry = {seq: 1, message: "row-1"};
        const copyEntry = vi.fn(async () => true);

        act(() => {
            root.render(
                <LogEntryView<TestEntry>
                    className="test-log-view"
                    title="Test Log"
                    entries={[entry]}
                    renderEntry={renderEntry}
                    copyAll={vi.fn(async () => true)}
                    copyEntry={copyEntry}
                    markAllRead={vi.fn()}
                    registerBodyElement={vi.fn()}
                    onClose={vi.fn()}
                    emptyMessage="No entries"
                    bodyClassName="test-log-body"
                    emptyClassName="test-log-empty"
                    entryClassName="test-log-entry"
                    logPrefix="[test-log]"
                />,
            );
        });

        const row = container.querySelector(".test-log-entry");
        expect(row).toBeTruthy();
        act(() => {
            row?.dispatchEvent(new KeyboardEvent("keydown", {key: "Enter", bubbles: true, cancelable: true}));
        });
        await act(async () => {
            await Promise.resolve();
        });

        expect(copyEntry).toHaveBeenCalledTimes(1);
        expect(copyEntry).toHaveBeenCalledWith(entry);
    });

    it("does not mark all copied when copyAll returns false", async () => {
        const copyAll = vi.fn(async () => false);

        act(() => {
            root.render(
                <LogEntryView<TestEntry>
                    className="test-log-view"
                    title="Test Log"
                    entries={[{seq: 1, message: "row-1"}]}
                    renderEntry={renderEntry}
                    copyAll={copyAll}
                    copyEntry={vi.fn(async () => true)}
                    markAllRead={vi.fn()}
                    registerBodyElement={vi.fn()}
                    onClose={vi.fn()}
                    emptyMessage="No entries"
                    bodyClassName="test-log-body"
                    emptyClassName="test-log-empty"
                    entryClassName="test-log-entry"
                    logPrefix="[test-log]"
                />,
            );
        });

        const copyAllButton = container.querySelector('button[aria-label="Copy all"]') as HTMLButtonElement | null;
        expect(copyAllButton).toBeTruthy();

        act(() => {
            copyAllButton?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });
        await act(async () => {
            await Promise.resolve();
        });

        expect(copyAll).toHaveBeenCalledTimes(1);
        expect(container.querySelector('button[aria-label="Copied!"]')).toBeNull();
    });
});
