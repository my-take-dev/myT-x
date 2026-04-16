import {act} from "react";
import {createRoot, type Root} from "react-dom/client";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {SqliteRenderer} from "./SqliteRenderer";

const {
    devPanelSqliteExportCSVMock,
    devPanelSqliteListTablesMock,
    devPanelSqliteQueryTableMock,
} = vi.hoisted(() => ({
    devPanelSqliteExportCSVMock: vi.fn(),
    devPanelSqliteListTablesMock: vi.fn(),
    devPanelSqliteQueryTableMock: vi.fn(),
}));

vi.mock("../../../../../api", () => ({
    api: {
        DevPanelSqliteListTables: devPanelSqliteListTablesMock,
        DevPanelSqliteQueryTable: devPanelSqliteQueryTableMock,
        DevPanelSqliteExportCSV: devPanelSqliteExportCSVMock,
    },
}));

vi.mock("../../../../../hooks/useContainerHeight", () => ({
    useContainerHeight: () => 320,
}));

describe("SqliteRenderer", () => {
    let container: HTMLDivElement;
    let root: Root;
    let consoleWarnSpy: ReturnType<typeof vi.spyOn>;

    beforeEach(() => {
        container = document.createElement("div");
        document.body.appendChild(container);
        root = createRoot(container);
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
        devPanelSqliteListTablesMock.mockReset();
        devPanelSqliteQueryTableMock.mockReset();
        devPanelSqliteExportCSVMock.mockReset();
        consoleWarnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    });

    afterEach(() => {
        act(() => {
            root.unmount();
        });
        container.remove();
        consoleWarnSpy.mockRestore();
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = false;
    });

    async function flushRenderer(): Promise<void> {
        await act(async () => {
            await Promise.resolve();
            await new Promise((resolve) => setTimeout(resolve, 0));
        });
    }

    function createDeferred<T>() {
        let resolve: ((value: T | PromiseLike<T>) => void) | null = null;
        let reject: ((reason?: unknown) => void) | null = null;
        const promise = new Promise<T>((nextResolve, nextReject) => {
            resolve = nextResolve;
            reject = nextReject;
        });
        return {
            promise,
            resolve(value: T) {
                resolve?.(value);
            },
            reject(reason?: unknown) {
                reject?.(reason);
            },
        };
    }

    it("loads tables and the first page of rows", async () => {
        devPanelSqliteListTablesMock.mockResolvedValue([
            {
                name: "users",
                columns: [
                    {name: "id", type: "INTEGER", not_null: false, primary_key: true},
                    {name: "name", type: "TEXT", not_null: true, primary_key: false},
                ],
                row_count: 2,
            },
        ]);
        devPanelSqliteQueryTableMock.mockResolvedValue({
            columns: ["id", "name"],
            rows: [["1", "alice"], ["2", "bob"]],
            null_masks: [[false, false], [false, false]],
            offset: 0,
            total_rows: 2,
            truncated: false,
        });

        await act(async () => {
            root.render(
                <SqliteRenderer
                    filePath="data/sample.db"
                    sessionKey="session-1"
                    sessionName="demo-session"
                />,
            );
        });
        await flushRenderer();

        expect(devPanelSqliteListTablesMock).toHaveBeenCalledWith("demo-session", "data/sample.db");
        expect(devPanelSqliteQueryTableMock).toHaveBeenCalledWith("demo-session", "data/sample.db", "users", 0, 100);
        expect(container.textContent).toContain("users");
        expect(container.textContent).toContain("alice");
        expect(container.textContent).toContain("bob");
    });

    it("supports paging and CSV export", async () => {
        devPanelSqliteListTablesMock.mockResolvedValue([
            {
                name: "users",
                columns: [
                    {name: "id", type: "INTEGER", not_null: false, primary_key: true},
                    {name: "name", type: "TEXT", not_null: true, primary_key: false},
                ],
                row_count: 2,
            },
        ]);
        devPanelSqliteQueryTableMock
            .mockResolvedValueOnce({
                columns: ["id", "name"],
                rows: [["1", "alice"]],
                null_masks: [[false, false]],
                offset: 0,
                total_rows: 2,
                truncated: true,
            })
            .mockResolvedValueOnce({
                columns: ["id", "name"],
                rows: [["1", "alice"]],
                null_masks: [[false, false]],
                offset: 0,
                total_rows: 2,
                truncated: true,
            })
            .mockResolvedValueOnce({
                columns: ["id", "name"],
                rows: [["2", "bob"]],
                null_masks: [[false, false]],
                offset: 25,
                total_rows: 2,
                truncated: false,
            });
        devPanelSqliteExportCSVMock.mockResolvedValue({
            path: "exports/sample-users.csv",
            row_count: 2,
        });

        await act(async () => {
            root.render(
                <SqliteRenderer
                    filePath="data/sample.db"
                    sessionKey="session-1"
                    sessionName="demo-session"
                />,
            );
        });
        await flushRenderer();

        const pageSizeSelect = container.querySelector(".file-view-sqlite-page-size select") as HTMLSelectElement | null;
        expect(pageSizeSelect).not.toBeNull();
        await act(async () => {
            if (pageSizeSelect) {
                pageSizeSelect.value = "25";
                pageSizeSelect.dispatchEvent(new Event("change", {bubbles: true}));
            }
        });
        await flushRenderer();

        const nextButton = Array.from(container.querySelectorAll(".file-view-sqlite-paging button"))
            .find((button) => button.textContent === "Next");
        expect(nextButton).not.toBeUndefined();
        await act(async () => {
            nextButton?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });
        await flushRenderer();

        expect(devPanelSqliteQueryTableMock).toHaveBeenLastCalledWith("demo-session", "data/sample.db", "users", 25, 25);

        const exportButton = Array.from(container.querySelectorAll(".file-view-sqlite-export button"))
            .find((button) => button.textContent?.includes("Export"));
        await act(async () => {
            exportButton?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
        });
        await flushRenderer();

        expect(devPanelSqliteExportCSVMock).toHaveBeenCalledWith(
            "demo-session",
            "data/sample.db",
            "users",
            "exports/sample-users.csv",
        );
        expect(container.textContent).toContain("Exported 2 rows to exports/sample-users.csv.");
    });

    it("ignores stale async responses after a session switch", async () => {
        let resolveFirstTables: ((value: unknown) => void) | null = null;
        devPanelSqliteListTablesMock
            .mockImplementationOnce(() => new Promise((resolve) => {
                resolveFirstTables = resolve;
            }))
            .mockImplementationOnce(() => new Promise(() => {}));

        await act(async () => {
            root.render(
                <SqliteRenderer
                    filePath="data/sample.db"
                    sessionKey="session-1"
                    sessionName="demo-session"
                />,
            );
        });

        await act(async () => {
            root.render(
                <SqliteRenderer
                    filePath="data/sample.db"
                    sessionKey="session-2"
                    sessionName="demo-session"
                />,
            );
        });

        await act(async () => {
            resolveFirstTables?.([
                {
                    name: "users",
                    columns: [],
                    row_count: 1,
                },
            ]);
        });
        await flushRenderer();

        expect(container.textContent).not.toContain("users");
    });

    it("keeps the schema loading state while a newer table request is still pending", async () => {
        const firstTables = createDeferred<unknown>();
        const secondTables = createDeferred<unknown>();
        devPanelSqliteListTablesMock
            .mockImplementationOnce(() => firstTables.promise)
            .mockImplementationOnce(() => secondTables.promise);

        await act(async () => {
            root.render(
                <SqliteRenderer
                    filePath="data/sample.db"
                    sessionKey="session-1"
                    sessionName="demo-session"
                />,
            );
        });

        await act(async () => {
            root.render(
                <SqliteRenderer
                    filePath="data/sample.db"
                    sessionKey="session-2"
                    sessionName="demo-session"
                />,
            );
        });

        await act(async () => {
            firstTables.resolve([
                {
                    name: "users",
                    columns: [],
                    row_count: 1,
                },
            ]);
        });
        await flushRenderer();

        expect(container.textContent).toContain("Loading SQLite schema...");
        expect(container.textContent).not.toContain("No SQLite tables or views were found.");
    });

    it("keeps the row loading state while a newer query request is still pending", async () => {
        const firstRows = createDeferred<unknown>();
        const secondRows = createDeferred<unknown>();
        devPanelSqliteListTablesMock.mockResolvedValue([
            {
                name: "users",
                columns: [
                    {name: "id", type: "INTEGER", not_null: false, primary_key: true},
                    {name: "name", type: "TEXT", not_null: true, primary_key: false},
                ],
                row_count: 2,
            },
        ]);
        devPanelSqliteQueryTableMock
            .mockImplementationOnce(() => firstRows.promise)
            .mockImplementationOnce(() => secondRows.promise);

        await act(async () => {
            root.render(
                <SqliteRenderer
                    filePath="data/sample.db"
                    sessionKey="session-1"
                    sessionName="demo-session"
                />,
            );
        });
        await flushRenderer();

        const pageSizeSelect = container.querySelector(".file-view-sqlite-page-size select") as HTMLSelectElement | null;
        expect(pageSizeSelect).not.toBeNull();
        await act(async () => {
            if (pageSizeSelect) {
                pageSizeSelect.value = "25";
                pageSizeSelect.dispatchEvent(new Event("change", {bubbles: true}));
            }
        });

        await act(async () => {
            firstRows.resolve({
                columns: ["id", "name"],
                rows: [["1", "alice"]],
                null_masks: [[false, false]],
                offset: 0,
                total_rows: 2,
                truncated: true,
            });
        });
        await flushRenderer();

        expect(container.textContent).toContain("Loading SQLite rows...");
        expect(container.textContent).not.toContain("No rows to display.");
    });

    it("keeps the export loading state for the latest export request", async () => {
        const firstExport = createDeferred<unknown>();
        const secondExport = createDeferred<unknown>();
        devPanelSqliteListTablesMock.mockResolvedValue([
            {
                name: "users",
                columns: [
                    {name: "id", type: "INTEGER", not_null: false, primary_key: true},
                    {name: "name", type: "TEXT", not_null: true, primary_key: false},
                ],
                row_count: 2,
            },
        ]);
        devPanelSqliteQueryTableMock.mockResolvedValue({
            columns: ["id", "name"],
            rows: [["1", "alice"]],
            null_masks: [[false, false]],
            offset: 0,
            total_rows: 2,
            truncated: false,
        });
        devPanelSqliteExportCSVMock
            .mockImplementationOnce(() => firstExport.promise)
            .mockImplementationOnce(() => secondExport.promise);

        await act(async () => {
            root.render(
                <SqliteRenderer
                    filePath="data/sample.db"
                    sessionKey="session-1"
                    sessionName="demo-session"
                />,
            );
        });
        await flushRenderer();

        const clickExport = async () => {
            const exportButton = Array.from(container.querySelectorAll(".file-view-sqlite-export button"))
                .find((button) => button.textContent?.includes("Export"));
            expect(exportButton).not.toBeUndefined();
            await act(async () => {
                exportButton?.dispatchEvent(new MouseEvent("click", {bubbles: true}));
            });
        };

        await clickExport();
        expect(container.textContent).toContain("Exporting...");

        await act(async () => {
            root.render(
                <SqliteRenderer
                    filePath="data/sample.db"
                    sessionKey="session-2"
                    sessionName="demo-session"
                />,
            );
        });
        await flushRenderer();

        await clickExport();
        expect(container.textContent).toContain("Exporting...");

        await act(async () => {
            firstExport.resolve({
                path: "exports/sample-users.csv",
                row_count: 2,
            });
        });
        await flushRenderer();

        expect(container.textContent).toContain("Exporting...");
        expect(container.textContent).not.toContain("Exported 2 rows to exports/sample-users.csv.");
    });

    it("renders API errors inline", async () => {
        devPanelSqliteListTablesMock.mockRejectedValue(new Error("sqlite read failed"));

        await act(async () => {
            root.render(
                <SqliteRenderer
                    filePath="data/sample.db"
                    sessionKey="session-1"
                    sessionName="demo-session"
                />,
            );
        });
        await flushRenderer();

        expect(container.textContent).toContain("sqlite read failed");
        expect(consoleWarnSpy).toHaveBeenCalledWith("[sqlite] failed to load tables", expect.objectContaining({
            path: "data/sample.db",
            session: "demo-session",
        }));
    });

    it("shows an empty-state message when the database has no tables", async () => {
        devPanelSqliteListTablesMock.mockResolvedValue([]);

        await act(async () => {
            root.render(
                <SqliteRenderer
                    filePath="data/empty.db"
                    sessionKey="session-1"
                    sessionName="demo-session"
                />,
            );
        });
        await flushRenderer();

        expect(container.textContent).toContain("No SQLite tables or views were found.");
    });
});
