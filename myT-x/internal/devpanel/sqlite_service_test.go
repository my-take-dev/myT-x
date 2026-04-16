package devpanel

import (
	"database/sql"
	"encoding/csv"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestSqliteSchemaFieldCounts(t *testing.T) {
	t.Helper()

	if got := reflect.TypeFor[sqliteSchemaEntry]().NumField(); got != sqliteSchemaFieldCount {
		t.Fatalf("sqliteSchemaEntry field count = %d, want %d", got, sqliteSchemaFieldCount)
	}
	if got := reflect.TypeFor[sqlitePragmaColumn]().NumField(); got != sqlitePragmaFieldCount {
		t.Fatalf("sqlitePragmaColumn field count = %d, want %d", got, sqlitePragmaFieldCount)
	}
}

func TestSqliteListTables(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "sample.db")
	createSQLiteFixture(t, dbPath)

	svc := newTestService("test-session", tmpDir)

	tables, err := svc.SqliteListTables("test-session", "sample.db")
	if err != nil {
		t.Fatalf("SqliteListTables() error = %v", err)
	}

	gotNames := make([]string, 0, len(tables))
	tableByName := make(map[string]SqliteTableInfo, len(tables))
	for _, table := range tables {
		gotNames = append(gotNames, table.Name)
		tableByName[table.Name] = table
	}

	wantNames := []string{"empty_tbl", "no_rowid", "odd\"name\nx", "user_names", "users"}
	if !slices.Equal(gotNames, wantNames) {
		t.Fatalf("table names = %v, want %v", gotNames, wantNames)
	}

	usersTable := tableByName["users"]
	if usersTable.RowCount != 2 {
		t.Fatalf("users row count = %d, want 2", usersTable.RowCount)
	}
	if len(usersTable.Columns) != 4 {
		t.Fatalf("users column count = %d, want 4", len(usersTable.Columns))
	}
	if usersTable.Columns[0].Name != "id" || !usersTable.Columns[0].PrimaryKey {
		t.Fatalf("users first column = %+v, want id primary key", usersTable.Columns[0])
	}
	if usersTable.Columns[1].Name != "name" || !usersTable.Columns[1].NotNull {
		t.Fatalf("users second column = %+v, want name not-null", usersTable.Columns[1])
	}

	oddTable := tableByName["odd\"name\nx"]
	if oddTable.RowCount != 1 {
		t.Fatalf("odd table row count = %d, want 1", oddTable.RowCount)
	}
}

func TestSqliteListTablesHandlesEmptyDatabase(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "empty.db")
	createEmptySQLiteDB(t, dbPath)

	svc := newTestService("test-session", tmpDir)

	tables, err := svc.SqliteListTables("test-session", "empty.db")
	if err != nil {
		t.Fatalf("SqliteListTables() error = %v", err)
	}
	if len(tables) != 0 {
		t.Fatalf("SqliteListTables() returned %d tables, want 0", len(tables))
	}
}

func TestSqliteListTablesRejectsCorruptFile(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "corrupt.db"), []byte("not a sqlite database"), 0o644); err != nil {
		t.Fatalf("WriteFile(corrupt.db): %v", err)
	}

	svc := newTestService("test-session", tmpDir)

	if _, err := svc.SqliteListTables("test-session", "corrupt.db"); err == nil {
		t.Fatal("SqliteListTables() error = nil, want corruption error")
	}
}

func TestSqliteQueryTable(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "sample.db")
	createSQLiteFixture(t, dbPath)

	svc := newTestService("test-session", tmpDir)

	result, err := svc.SqliteQueryTable("test-session", "sample.db", "users", 0, 1)
	if err != nil {
		t.Fatalf("SqliteQueryTable() error = %v", err)
	}

	if !slices.Equal(result.Columns, []string{"id", "name", "bio", "age"}) {
		t.Fatalf("columns = %v, want [id name bio age]", result.Columns)
	}
	if result.Offset != 0 {
		t.Fatalf("offset = %d, want 0", result.Offset)
	}
	if result.TotalRows != 2 {
		t.Fatalf("total rows = %d, want 2", result.TotalRows)
	}
	if !result.Truncated {
		t.Fatal("truncated = false, want true")
	}
	if len(result.Rows) != 1 || len(result.NullMasks) != 1 {
		t.Fatalf("rows/nullMasks lengths = %d/%d, want 1/1", len(result.Rows), len(result.NullMasks))
	}
	if !slices.Equal(result.Rows[0], []string{"1", "alice", "hello,world", "30"}) {
		t.Fatalf("first row = %v, want [1 alice hello,world 30]", result.Rows[0])
	}
	if !slices.Equal(result.NullMasks[0], []bool{false, false, false, false}) {
		t.Fatalf("first row null mask = %v, want all false", result.NullMasks[0])
	}

	nextPage, err := svc.SqliteQueryTable("test-session", "sample.db", "users", 1, 10)
	if err != nil {
		t.Fatalf("SqliteQueryTable(next page) error = %v", err)
	}
	if nextPage.Truncated {
		t.Fatal("next page truncated = true, want false")
	}
	if len(nextPage.Rows) != 1 {
		t.Fatalf("next page row count = %d, want 1", len(nextPage.Rows))
	}
	if !slices.Equal(nextPage.Rows[0], []string{"2", "bob", "", "41"}) {
		t.Fatalf("second row = %v, want [2 bob  41]", nextPage.Rows[0])
	}
	if !slices.Equal(nextPage.NullMasks[0], []bool{false, false, true, false}) {
		t.Fatalf("second row null mask = %v, want bio null", nextPage.NullMasks[0])
	}
}

func TestSqliteQueryTableSupportsSpecialNamesAndReportsMissingTables(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "sample.db")
	createSQLiteFixture(t, dbPath)

	svc := newTestService("test-session", tmpDir)

	specialResult, err := svc.SqliteQueryTable("test-session", "sample.db", "odd\"name\nx", 0, 25)
	if err != nil {
		t.Fatalf("SqliteQueryTable(special) error = %v", err)
	}
	if len(specialResult.Rows) != 1 || specialResult.Rows[0][1] != "quoted" {
		t.Fatalf("special table rows = %v, want one row with value 'quoted'", specialResult.Rows)
	}

	missingResult, err := svc.SqliteQueryTable("test-session", "sample.db", "missing_table", 0, 25)
	if err == nil {
		t.Fatal("SqliteQueryTable(missing) error = nil, want missing-table error")
	}
	if !strings.Contains(err.Error(), `sqlite table or view "missing_table" is no longer available`) {
		t.Fatalf("SqliteQueryTable(missing) error = %v, want missing-table message", err)
	}
	if !reflect.DeepEqual(missingResult, SqliteQueryResult{}) {
		t.Fatalf("missing table result = %+v, want zero-value result", missingResult)
	}
}

func TestSqliteExportCSV(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "sample.db")
	createSQLiteFixture(t, dbPath)

	svc := newTestService("test-session", tmpDir)

	result, err := svc.SqliteExportCSV("test-session", "sample.db", "users", "exports/users.csv")
	if err != nil {
		t.Fatalf("SqliteExportCSV() error = %v", err)
	}
	if result.Path != "exports/users.csv" {
		t.Fatalf("export path = %q, want %q", result.Path, "exports/users.csv")
	}
	if result.RowCount != 2 {
		t.Fatalf("export row count = %d, want 2", result.RowCount)
	}

	exportFile, err := os.Open(filepath.Join(tmpDir, "exports", "users.csv"))
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

	wantRecords := [][]string{
		{"id", "name", "bio", "age"},
		{"1", "alice", "hello,world", "30"},
		{"2", "bob", "", "41"},
	}
	if !reflect.DeepEqual(records, wantRecords) {
		t.Fatalf("csv records = %v, want %v", records, wantRecords)
	}

	missingResult, err := svc.SqliteExportCSV("test-session", "sample.db", "missing_table", "exports/missing.csv")
	if err == nil {
		t.Fatal("SqliteExportCSV(missing) error = nil, want missing-table error")
	}
	if !strings.Contains(err.Error(), `sqlite table or view "missing_table" is no longer available`) {
		t.Fatalf("SqliteExportCSV(missing) error = %v, want missing-table message", err)
	}
	if !reflect.DeepEqual(missingResult, SqliteExportResult{}) {
		t.Fatalf("missing export result = %+v, want zero-value result", missingResult)
	}
	if _, statErr := os.Stat(filepath.Join(tmpDir, "exports", "missing.csv")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("Stat(missing export) error = %v, want not-exist", statErr)
	}
}

func TestSqlitePathValidation(t *testing.T) {
	parentDir := t.TempDir()
	rootDir := filepath.Join(parentDir, "root")
	if err := os.MkdirAll(rootDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(root): %v", err)
	}

	dbPath := filepath.Join(rootDir, "sample.db")
	createSQLiteFixture(t, dbPath)

	outsideDBPath := filepath.Join(parentDir, "outside.db")
	createSQLiteFixture(t, outsideDBPath)

	svc := newTestService("test-session", rootDir)

	if _, err := svc.SqliteListTables("test-session", filepath.Join("..", "outside.db")); err == nil {
		t.Fatal("SqliteListTables() error = nil, want path traversal rejection")
	}

	if _, err := svc.SqliteExportCSV("test-session", "sample.db", "users", filepath.Join("..", "escape.csv")); err == nil {
		t.Fatal("SqliteExportCSV() error = nil, want export path traversal rejection")
	}

	if _, err := svc.SqliteExportCSV("test-session", "sample.db", "users", "sample.db"); err == nil {
		t.Fatal("SqliteExportCSV() error = nil, want destination/database path collision rejection")
	}
}

func createSQLiteFixture(t *testing.T, dbPath string) {
	t.Helper()

	db := openSQLiteForFixture(t, dbPath)
	mustExecSQLite(t, db, `CREATE TABLE users (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		bio TEXT,
		age INTEGER
	)`)
	mustExecSQLite(t, db, `INSERT INTO users (name, bio, age) VALUES (?, ?, ?)`, "alice", "hello,world", 30)
	mustExecSQLite(t, db, `INSERT INTO users (name, bio, age) VALUES (?, ?, ?)`, "bob", nil, 41)
	mustExecSQLite(t, db, `CREATE TABLE empty_tbl (x TEXT)`)
	mustExecSQLite(t, db, `CREATE TABLE no_rowid (
		id TEXT PRIMARY KEY,
		label TEXT NOT NULL
	) WITHOUT ROWID`)
	mustExecSQLite(t, db, `INSERT INTO no_rowid (id, label) VALUES (?, ?)`, "row-1", "without-rowid")
	specialTableName := "odd\"name\nx"
	mustExecSQLite(t, db, `CREATE TABLE `+quoteSqliteIdentifier(specialTableName)+` (
		id INTEGER PRIMARY KEY,
		value TEXT
	)`)
	mustExecSQLite(t, db, `INSERT INTO `+quoteSqliteIdentifier(specialTableName)+` (value) VALUES (?)`, "quoted")
	mustExecSQLite(t, db, `CREATE VIEW user_names AS SELECT name, bio FROM users`)
	closeFixtureDB(t, db)
}

func createEmptySQLiteDB(t *testing.T, dbPath string) {
	t.Helper()

	db := openSQLiteForFixture(t, dbPath)
	mustExecSQLite(t, db, `CREATE TABLE __temp_empty (id INTEGER PRIMARY KEY)`)
	mustExecSQLite(t, db, `DROP TABLE __temp_empty`)
	closeFixtureDB(t, db)
}

func openSQLiteForFixture(t *testing.T, dbPath string) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", "file:"+dbPath+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatalf("sql.Open(%q): %v", dbPath, err)
	}
	return db
}

func mustExecSQLite(t *testing.T, db *sql.DB, statement string, args ...any) {
	t.Helper()

	if _, err := db.Exec(statement, args...); err != nil {
		t.Fatalf("Exec(%s) error = %v", normalizeSQLForLog(statement), err)
	}
}

func closeFixtureDB(t *testing.T, db *sql.DB) {
	t.Helper()

	if err := db.Close(); err != nil {
		t.Fatalf("Close(fixture db) error = %v", err)
	}
}

func normalizeSQLForLog(statement string) string {
	return strings.Join(strings.Fields(statement), " ")
}
