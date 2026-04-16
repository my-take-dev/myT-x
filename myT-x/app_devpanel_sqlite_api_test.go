package main

import (
	"database/sql"
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestDevPanelSqliteWrappers(t *testing.T) {
	app, rootPath := newDevPanelAppForTest(t)
	dbPath := filepath.Join(rootPath, "sample.db")
	createAppSQLiteFixture(t, dbPath)

	tables, err := app.DevPanelSqliteListTables("session-a", "sample.db")
	if err != nil {
		t.Fatalf("DevPanelSqliteListTables() error = %v", err)
	}
	if len(tables) != 1 || tables[0].Name != "users" {
		t.Fatalf("tables = %+v, want one users table", tables)
	}

	queryResult, err := app.DevPanelSqliteQueryTable("session-a", "sample.db", "users", 0, 25)
	if err != nil {
		t.Fatalf("DevPanelSqliteQueryTable() error = %v", err)
	}
	if len(queryResult.Rows) != 2 || queryResult.TotalRows != 2 {
		t.Fatalf("query result = %+v, want 2 rows", queryResult)
	}

	exportResult, err := app.DevPanelSqliteExportCSV("session-a", "sample.db", "users", "exports/users.csv")
	if err != nil {
		t.Fatalf("DevPanelSqliteExportCSV() error = %v", err)
	}
	if exportResult.Path != "exports/users.csv" || exportResult.RowCount != 2 {
		t.Fatalf("export result = %+v, want exports/users.csv with 2 rows", exportResult)
	}

	exportFile, err := os.Open(filepath.Join(rootPath, "exports", "users.csv"))
	if err != nil {
		t.Fatalf("Open(export csv) error = %v", err)
	}
	defer func() {
		if closeErr := exportFile.Close(); closeErr != nil {
			t.Errorf("Close(export csv) error = %v", closeErr)
		}
	}()

	records, err := csv.NewReader(exportFile).ReadAll()
	if err != nil {
		t.Fatalf("ReadAll(export csv) error = %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("csv record count = %d, want 3", len(records))
	}

	if _, err := app.DevPanelSqliteQueryTable("session-a", "sample.db", "missing_table", 0, 25); err == nil {
		t.Fatal("DevPanelSqliteQueryTable(missing) error = nil, want missing-table error")
	} else if !strings.Contains(err.Error(), `sqlite table or view "missing_table" is no longer available`) {
		t.Fatalf("DevPanelSqliteQueryTable(missing) error = %v, want missing-table message", err)
	}

	if _, err := app.DevPanelSqliteExportCSV("session-a", "sample.db", "missing_table", "exports/missing.csv"); err == nil {
		t.Fatal("DevPanelSqliteExportCSV(missing) error = nil, want missing-table error")
	} else if !strings.Contains(err.Error(), `sqlite table or view "missing_table" is no longer available`) {
		t.Fatalf("DevPanelSqliteExportCSV(missing) error = %v, want missing-table message", err)
	}
}

func createAppSQLiteFixture(t *testing.T, dbPath string) {
	t.Helper()

	db, err := sql.Open("sqlite", "file:"+dbPath+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatalf("sql.Open(%q): %v", dbPath, err)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			t.Errorf("db.Close(%q): %v", dbPath, closeErr)
		}
	}()

	if _, err := db.Exec(`CREATE TABLE users (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		bio TEXT
	)`); err != nil {
		t.Fatalf("Create users table: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO users (name, bio) VALUES (?, ?), (?, ?)`, "alice", "hi", "bob", nil); err != nil {
		t.Fatalf("Insert users rows: %v", err)
	}
}
