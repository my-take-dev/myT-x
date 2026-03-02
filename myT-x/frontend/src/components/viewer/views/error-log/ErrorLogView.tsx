import {useViewerStore} from "../../viewerStore";
import {LogEntryView} from "../shared/LogEntryView";
import {formatTimestamp} from "../../../../utils/timestampUtils";
import {useErrorLog} from "./useErrorLog";
import type {ErrorLogEntry} from "../../../../stores/errorLogStore";

function renderErrorEntry(entry: ErrorLogEntry, _isCopied: boolean) {
    return (
        <>
            <span className="error-log-ts">{formatTimestamp(entry.ts)}</span>
            <span className={`error-log-level ${entry.level}`}>{entry.level}</span>
            <span className="error-log-msg">{entry.msg}</span>
            {entry.source && (
                <span className="error-log-source">[{entry.source}]</span>
            )}
        </>
    );
}

export function ErrorLogView() {
    const closeView = useViewerStore((s) => s.closeView);
    const {entries, markAllRead, copyAll, copyEntry, registerBodyElement} = useErrorLog();

    return (
        <LogEntryView<ErrorLogEntry>
            className="error-log-view"
            title="Error Log"
            entries={entries}
            renderEntry={renderErrorEntry}
            copyAll={copyAll}
            copyEntry={copyEntry}
            markAllRead={markAllRead}
            registerBodyElement={registerBodyElement}
            onClose={closeView}
            emptyMessage="No errors logged"
            bodyClassName="error-log-body"
            emptyClassName="error-log-empty"
            entryClassName="error-log-entry"
            logPrefix="[error-log]"
        />
    );
}
