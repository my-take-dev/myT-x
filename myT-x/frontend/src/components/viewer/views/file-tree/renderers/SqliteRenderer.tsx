import {memo, useCallback, useEffect, useMemo, useRef, useState, type ChangeEvent} from "react";
import {FixedSizeList, type ListChildComponentProps} from "react-window";
import {api} from "../../../../../api";
import {useContainerHeight} from "../../../../../hooks/useContainerHeight";
import {toErrorMessage} from "../../../../../utils/errorUtils";
import {matchesCapturedSessionKey} from "../../../../../utils/sessionGuard";
import type {devpanel} from "../../../../../../wailsjs/go/models";

interface SqliteRendererProps {
    readonly filePath: string;
    readonly sessionKey?: string;
    readonly sessionName?: string | null;
}

interface SqliteRowData {
    readonly columns: readonly string[];
    readonly rows: readonly string[][];
    readonly nullMasks: readonly boolean[][];
    readonly gridTemplateColumns: string;
}

const SQLITE_PAGE_SIZES = [25, 50, 100, 250, 500] as const;
const SQLITE_DEFAULT_PAGE_SIZE = 100;
const SQLITE_ROW_HEIGHT = 32;

const SqliteRow = memo(function SqliteRow({index, style, data}: ListChildComponentProps<SqliteRowData>) {
    const row = data.rows[index];
    const nullMask = data.nullMasks[index];
    if (!row || !nullMask) {
        return null;
    }

    return (
        <div
            className="file-view-sqlite-row"
            style={{...style, gridTemplateColumns: data.gridTemplateColumns}}
            role="row"
        >
            {data.columns.map((column, cellIndex) => {
                const isNull = Boolean(nullMask[cellIndex]);
                const cellValue = row[cellIndex] ?? "";
                return (
                    <div
                        key={`${index}-${column}-${cellIndex}`}
                        className={`file-view-sqlite-cell${isNull ? " is-null" : ""}`}
                        title={isNull ? "NULL" : cellValue}
                        role="gridcell"
                    >
                        {isNull ? "NULL" : (cellValue === "" ? " " : cellValue)}
                    </div>
                );
            })}
        </div>
    );
});

function sanitizeFileSegment(value: string): string {
    const sanitized = value.trim()
        .replace(/[^a-zA-Z0-9._-]+/g, "_")
        .replace(/^_+|_+$/g, "");
    return sanitized === "" ? "sqlite" : sanitized;
}

function buildDefaultExportPath(filePath: string, tableName: string | null): string {
    const baseName = filePath.split("/").pop() ?? "database.db";
    const stem = sanitizeFileSegment(baseName.replace(/\.(?:sqlite3?|db)$/i, ""));
    const tableStem = sanitizeFileSegment(tableName ?? "table");
    return `exports/${stem}-${tableStem}.csv`;
}

export const SqliteRenderer = memo(function SqliteRenderer({
    filePath,
    sessionKey,
    sessionName,
}: SqliteRendererProps) {
    const latestSessionKeyRef = useRef(sessionKey ?? "");
    const isMountedRef = useRef(true);
    const rowsViewportRef = useRef<HTMLDivElement>(null);
    const pendingTableLoadsRef = useRef(0);
    const pendingRowLoadsRef = useRef(0);
    const pendingExportsRef = useRef(0);
    const rowsHeight = useContainerHeight(rowsViewportRef, SQLITE_ROW_HEIGHT * 8, {noiseThresholdPx: 1});

    const [tables, setTables] = useState<readonly devpanel.SqliteTableInfo[]>([]);
    const [selectedTableName, setSelectedTableName] = useState<string | null>(null);
    const [queryResult, setQueryResult] = useState<devpanel.SqliteQueryResult | null>(null);
    const [isLoadingTables, setIsLoadingTables] = useState(false);
    const [isLoadingRows, setIsLoadingRows] = useState(false);
    const [isExporting, setIsExporting] = useState(false);
    const [loadError, setLoadError] = useState<string | null>(null);
    const [queryError, setQueryError] = useState<string | null>(null);
    const [exportError, setExportError] = useState<string | null>(null);
    const [exportNotice, setExportNotice] = useState<string | null>(null);
    const [pageSize, setPageSize] = useState<number>(SQLITE_DEFAULT_PAGE_SIZE);
    const [offset, setOffset] = useState(0);
    const [exportPath, setExportPath] = useState(() => buildDefaultExportPath(filePath, null));

    latestSessionKeyRef.current = sessionKey ?? "";

    useEffect(() => {
        return () => {
            isMountedRef.current = false;
        };
    }, []);

    const normalizedFilePath = filePath.trim();
    const normalizedSessionName = sessionName?.trim() ?? "";
    const normalizedSessionKey = sessionKey?.trim() ?? "";
    const hasActiveSession = normalizedFilePath !== "" && normalizedSessionName !== "" && normalizedSessionKey !== "";

    useEffect(() => {
        setTables([]);
        setSelectedTableName(null);
        setQueryResult(null);
        setIsExporting(false);
        setLoadError(null);
        setQueryError(null);
        setExportError(null);
        setExportNotice(null);
        setPageSize(SQLITE_DEFAULT_PAGE_SIZE);
        setOffset(0);
    }, [normalizedFilePath, normalizedSessionKey]);

    useEffect(() => {
        setExportPath(buildDefaultExportPath(normalizedFilePath, selectedTableName));
    }, [normalizedFilePath, selectedTableName]);

    useEffect(() => {
        if (!hasActiveSession) {
            setLoadError("SQLite preview requires an active session and file path.");
            return;
        }

        const capturedSessionKey = normalizedSessionKey;
        let disposed = false;

        const loadTables = async () => {
            pendingTableLoadsRef.current += 1;
            setIsLoadingTables(true);
            setLoadError(null);

            try {
                const nextTables = await api.DevPanelSqliteListTables(normalizedSessionName, normalizedFilePath);
                if (disposed || !matchesCapturedSessionKey(capturedSessionKey, latestSessionKeyRef.current)) {
                    return;
                }

                setTables(nextTables);
                setSelectedTableName((previousSelectedTableName) => {
                    if (previousSelectedTableName && nextTables.some((table) => table.name === previousSelectedTableName)) {
                        return previousSelectedTableName;
                    }
                    return nextTables[0]?.name ?? null;
                });
            } catch (err: unknown) {
                if (disposed || !matchesCapturedSessionKey(capturedSessionKey, latestSessionKeyRef.current)) {
                    return;
                }

                console.warn("[sqlite] failed to load tables", {
                    path: normalizedFilePath,
                    session: normalizedSessionName,
                    err,
                });
                setTables([]);
                setSelectedTableName(null);
                setLoadError(toErrorMessage(err, "Failed to load SQLite tables."));
            } finally {
                if (isMountedRef.current) {
                    pendingTableLoadsRef.current = Math.max(0, pendingTableLoadsRef.current - 1);
                    setIsLoadingTables(pendingTableLoadsRef.current > 0);
                }
            }
        };

        void loadTables();

        return () => {
            disposed = true;
        };
    }, [hasActiveSession, normalizedFilePath, normalizedSessionKey, normalizedSessionName]);

    const selectedTable = useMemo(
        () => tables.find((table) => table.name === selectedTableName) ?? null,
        [selectedTableName, tables],
    );

    useEffect(() => {
        if (!hasActiveSession || selectedTableName === null) {
            setQueryResult(null);
            setQueryError(null);
            return;
        }

        const capturedSessionKey = normalizedSessionKey;
        let disposed = false;

        const loadPage = async () => {
            pendingRowLoadsRef.current += 1;
            setIsLoadingRows(true);
            setQueryError(null);
            setExportError(null);
            setExportNotice(null);

            try {
                const nextResult = await api.DevPanelSqliteQueryTable(
                    normalizedSessionName,
                    normalizedFilePath,
                    selectedTableName,
                    offset,
                    pageSize,
                );
                if (disposed || !matchesCapturedSessionKey(capturedSessionKey, latestSessionKeyRef.current)) {
                    return;
                }
                setQueryResult(nextResult);
            } catch (err: unknown) {
                if (disposed || !matchesCapturedSessionKey(capturedSessionKey, latestSessionKeyRef.current)) {
                    return;
                }

                console.warn("[sqlite] failed to load rows", {
                    path: normalizedFilePath,
                    session: normalizedSessionName,
                    table: selectedTableName,
                    err,
                });
                setQueryResult(null);
                setQueryError(toErrorMessage(err, "Failed to load SQLite rows."));
            } finally {
                if (isMountedRef.current) {
                    pendingRowLoadsRef.current = Math.max(0, pendingRowLoadsRef.current - 1);
                    setIsLoadingRows(pendingRowLoadsRef.current > 0);
                }
            }
        };

        void loadPage();

        return () => {
            disposed = true;
        };
    }, [hasActiveSession, normalizedFilePath, normalizedSessionKey, normalizedSessionName, offset, pageSize, selectedTableName]);

    const handleSelectTable = useCallback((tableName: string) => {
        setSelectedTableName(tableName);
        setOffset(0);
        setQueryResult(null);
        setQueryError(null);
        setExportError(null);
        setExportNotice(null);
    }, []);

    const handlePreviousPage = useCallback(() => {
        setOffset((currentOffset) => Math.max(0, currentOffset - pageSize));
    }, [pageSize]);

    const handleNextPage = useCallback(() => {
        setOffset((currentOffset) => currentOffset + pageSize);
    }, [pageSize]);

    const handlePageSizeChange = useCallback((event: ChangeEvent<HTMLSelectElement>) => {
        const nextPageSize = Number.parseInt(event.target.value, 10);
        setPageSize(Number.isFinite(nextPageSize) ? nextPageSize : SQLITE_DEFAULT_PAGE_SIZE);
        setOffset(0);
    }, []);

    const handleExportPathChange = useCallback((event: ChangeEvent<HTMLInputElement>) => {
        setExportPath(event.target.value);
    }, []);

    const handleExportCSV = useCallback(async () => {
        if (!hasActiveSession || selectedTableName === null) {
            return;
        }

        const trimmedExportPath = exportPath.trim();
        if (trimmedExportPath === "") {
            setExportNotice(null);
            setExportError("Export path is required.");
            return;
        }

        const capturedSessionKey = normalizedSessionKey;
        pendingExportsRef.current += 1;
        setIsExporting(true);
        setExportError(null);
        setExportNotice(null);

        try {
            const result = await api.DevPanelSqliteExportCSV(
                normalizedSessionName,
                normalizedFilePath,
                selectedTableName,
                trimmedExportPath,
            );
            if (!matchesCapturedSessionKey(capturedSessionKey, latestSessionKeyRef.current)) {
                return;
            }

            setExportPath(result.path);
            setExportNotice(`Exported ${result.row_count} rows to ${result.path}.`);
        } catch (err: unknown) {
            if (!matchesCapturedSessionKey(capturedSessionKey, latestSessionKeyRef.current)) {
                return;
            }

            console.warn("[sqlite] failed to export csv", {
                path: normalizedFilePath,
                session: normalizedSessionName,
                table: selectedTableName,
                err,
            });
            setExportError(toErrorMessage(err, "Failed to export SQLite table to CSV."));
        } finally {
            if (isMountedRef.current) {
                pendingExportsRef.current = Math.max(0, pendingExportsRef.current - 1);
                setIsExporting(pendingExportsRef.current > 0);
            }
        }
    }, [exportPath, hasActiveSession, normalizedFilePath, normalizedSessionKey, normalizedSessionName, selectedTableName]);

    const gridTemplateColumns = useMemo(() => {
        const columnCount = queryResult?.columns.length ?? 0;
        return columnCount > 0 ? `repeat(${columnCount}, minmax(160px, 1fr))` : "minmax(0, 1fr)";
    }, [queryResult?.columns.length]);

    const rowData = useMemo<SqliteRowData>(() => ({
        columns: queryResult?.columns ?? [],
        rows: queryResult?.rows ?? [],
        nullMasks: queryResult?.null_masks ?? [],
        gridTemplateColumns,
    }), [gridTemplateColumns, queryResult?.columns, queryResult?.null_masks, queryResult?.rows]);

    const hasNextPage = Boolean(queryResult && queryResult.truncated);
    const pageSummary = queryResult
        ? `${queryResult.rows.length === 0 ? 0 : queryResult.offset + 1}-${queryResult.offset + queryResult.rows.length} / ${queryResult.total_rows}`
        : "0 / 0";

    if (!hasActiveSession) {
        return <div className="file-content-empty">SQLite preview requires an active session and file path.</div>;
    }

    return (
        <div className="file-view-sqlite">
            <aside className="file-view-sqlite-tables">
                <div className="file-view-sqlite-tables-header">Tables</div>
                {isLoadingTables ? (
                    <div className="file-content-empty">Loading SQLite schema...</div>
                ) : loadError ? (
                    <div className="file-content-empty">{loadError}</div>
                ) : tables.length === 0 ? (
                    <div className="file-content-empty">No SQLite tables or views were found.</div>
                ) : (
                    <div className="file-view-sqlite-tables-list" role="listbox" aria-label="SQLite tables">
                        {tables.map((table) => (
                            <button
                                key={table.name}
                                type="button"
                                className={`file-view-sqlite-tables-item${table.name === selectedTableName ? " selected" : ""}`}
                                onClick={() => handleSelectTable(table.name)}
                                aria-selected={table.name === selectedTableName}
                            >
                                <span className="file-view-sqlite-tables-name">{table.name}</span>
                                <span className="file-view-sqlite-tables-count">{table.row_count}</span>
                            </button>
                        ))}
                    </div>
                )}
            </aside>
            <section className="file-view-sqlite-main">
                {selectedTable === null ? (
                    <div className="file-content-empty">Select a table to browse rows.</div>
                ) : (
                    <>
                        <div className="file-view-sqlite-columns">
                            {selectedTable.columns.map((column) => (
                                <div key={`${selectedTable.name}-${column.name}`} className="file-view-sqlite-column-pill">
                                    <span className="file-view-sqlite-column-name">{column.name}</span>
                                    {column.type ? (
                                        <span className="file-view-sqlite-column-type">{column.type}</span>
                                    ) : null}
                                    {column.primary_key ? (
                                        <span className="file-view-sqlite-column-flag">PK</span>
                                    ) : null}
                                    {column.not_null ? (
                                        <span className="file-view-sqlite-column-flag">NOT NULL</span>
                                    ) : null}
                                </div>
                            ))}
                        </div>

                        {queryError ? (
                            <div className="file-content-empty">{queryError}</div>
                        ) : (
                            <div className="file-view-sqlite-rows-panel">
                                <div className="file-view-sqlite-grid file-view-sqlite-grid-header" style={{gridTemplateColumns}}>
                                    {(queryResult?.columns ?? selectedTable.columns.map((column) => column.name)).map((columnName) => (
                                        <div key={`${selectedTable.name}-${columnName}`} className="file-view-sqlite-header-cell" role="columnheader">
                                            {columnName}
                                        </div>
                                    ))}
                                </div>
                                <div className="file-view-sqlite-rows" ref={rowsViewportRef}>
                                    {isLoadingRows ? (
                                        <div className="file-content-empty">Loading SQLite rows...</div>
                                    ) : queryResult && queryResult.rows.length > 0 && rowsHeight > 0 ? (
                                        <FixedSizeList
                                            height={rowsHeight}
                                            itemCount={queryResult.rows.length}
                                            itemSize={SQLITE_ROW_HEIGHT}
                                            width="100%"
                                            itemData={rowData}
                                            overscanCount={8}
                                        >
                                            {SqliteRow}
                                        </FixedSizeList>
                                    ) : (
                                        <div className="file-content-empty">No rows to display.</div>
                                    )}
                                </div>
                            </div>
                        )}

                        <div className="file-view-sqlite-footer">
                            <div className="file-view-sqlite-paging">
                                <button type="button" onClick={handlePreviousPage} disabled={offset <= 0 || isLoadingRows}>
                                    Prev
                                </button>
                                <span className="file-view-sqlite-paging-status">{pageSummary}</span>
                                <button type="button" onClick={handleNextPage} disabled={!hasNextPage || isLoadingRows}>
                                    Next
                                </button>
                                <label className="file-view-sqlite-page-size">
                                    <span>Page size</span>
                                    <select value={pageSize} onChange={handlePageSizeChange}>
                                        {SQLITE_PAGE_SIZES.map((size) => (
                                            <option key={size} value={size}>
                                                {size}
                                            </option>
                                        ))}
                                    </select>
                                </label>
                            </div>
                            <div className="file-view-sqlite-export">
                                <input
                                    type="text"
                                    value={exportPath}
                                    onChange={handleExportPathChange}
                                    placeholder="exports/table.csv"
                                    spellCheck={false}
                                    aria-label="CSV export path"
                                />
                                <button type="button" onClick={handleExportCSV} disabled={isExporting}>
                                    {isExporting ? "Exporting..." : "Export CSV"}
                                </button>
                            </div>
                            {exportError ? (
                                <div className="file-view-sqlite-status error">{exportError}</div>
                            ) : exportNotice ? (
                                <div className="file-view-sqlite-status">{exportNotice}</div>
                            ) : null}
                        </div>
                    </>
                )}
            </section>
        </div>
    );
});
