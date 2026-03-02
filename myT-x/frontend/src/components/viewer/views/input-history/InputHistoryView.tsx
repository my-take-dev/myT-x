import {useViewerStore} from "../../viewerStore";
import {LogEntryView} from "../shared/LogEntryView";
import {formatTimestamp} from "../../../../utils/timestampUtils";
import {formatInputForDisplay, useInputHistory} from "./useInputHistory";
import type {InputHistoryEntry} from "../../../../stores/inputHistoryStore";

function renderInputEntry(entry: InputHistoryEntry, _isCopied: boolean) {
    return (
        <>
            <span className="input-history-ts">{formatTimestamp(entry.ts)}</span>
            <span className="input-history-pane">{entry.pane_id}</span>
            <span className="input-history-msg">{formatInputForDisplay(entry.input)}</span>
            {entry.source && (
                <span className="input-history-source">[{entry.source}]</span>
            )}
        </>
    );
}

export function InputHistoryView() {
    const closeView = useViewerStore((s) => s.closeView);
    const {entries, markAllRead, copyAll, copyEntry, registerBodyElement} = useInputHistory();

    return (
        <LogEntryView<InputHistoryEntry>
            className="input-history-view"
            title="Input History"
            entries={entries}
            renderEntry={renderInputEntry}
            copyAll={copyAll}
            copyEntry={copyEntry}
            markAllRead={markAllRead}
            registerBodyElement={registerBodyElement}
            onClose={closeView}
            emptyMessage="No input history"
            bodyClassName="input-history-body"
            emptyClassName="input-history-empty"
            entryClassName="input-history-entry"
            logPrefix="[input-history]"
        />
    );
}
