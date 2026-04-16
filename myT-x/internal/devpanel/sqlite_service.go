package devpanel

import (
	"database/sql"
	"encoding/csv"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const (
	defaultSqliteQueryLimit = 100
	maxSqliteQueryLimit     = 5000
	sqliteSchemaFieldCount  = 2
	sqlitePragmaFieldCount  = 6
)

type sqliteSchemaEntry struct {
	Name string
	Type string
}

type sqlitePragmaColumn struct {
	CID          int
	Name         string
	Type         string
	NotNull      int
	DefaultValue sql.NullString
	PrimaryKey   int
}

func (c sqlitePragmaColumn) toColumnInfo() SqliteColumnInfo {
	return SqliteColumnInfo{
		Name:       c.Name,
		Type:       c.Type,
		NotNull:    c.NotNull != 0,
		PrimaryKey: c.PrimaryKey != 0,
	}
}

// SqliteListTables returns browsable SQLite tables/views for a session-scoped database path.
func (s *Service) SqliteListTables(sessionName, dbPath string) ([]SqliteTableInfo, error) {
	resolvedDBPath, _, err := s.resolveSqliteDBPath(sessionName, dbPath)
	if err != nil {
		return nil, err
	}

	db, err := openReadOnlySQLite(resolvedDBPath)
	if err != nil {
		return nil, err
	}
	defer closeSQLiteDB(db, resolvedDBPath)

	schemaEntries, err := listSqliteSchemaEntries(db)
	if err != nil {
		return nil, fmt.Errorf("list sqlite tables: %w", err)
	}

	tableInfos := make([]SqliteTableInfo, 0, len(schemaEntries))
	for _, entry := range schemaEntries {
		columns, columnErr := loadSqliteColumns(db, entry.Name)
		if columnErr != nil {
			if isMissingTableErr(columnErr) {
				slog.Warn("[DEVPANEL] sqlite table disappeared during list", "path", resolvedDBPath, "table", entry.Name)
				continue
			}
			return nil, fmt.Errorf("load sqlite columns for %q: %w", entry.Name, columnErr)
		}

		rowCount, countErr := countSqliteRows(db, entry.Name)
		if countErr != nil {
			if isMissingTableErr(countErr) {
				slog.Warn("[DEVPANEL] sqlite table disappeared during row count", "path", resolvedDBPath, "table", entry.Name)
				continue
			}
			slog.Warn("[DEVPANEL] sqlite row count degraded", "path", resolvedDBPath, "table", entry.Name, "error", countErr)
		}

		tableInfos = append(tableInfos, SqliteTableInfo{
			Name:     entry.Name,
			Columns:  columns,
			RowCount: rowCount,
		})
	}

	return tableInfos, nil
}

// SqliteQueryTable returns one page of rows for a validated SQLite table or view.
func (s *Service) SqliteQueryTable(sessionName, dbPath, tableName string, offset int64, limit int) (SqliteQueryResult, error) {
	resolvedDBPath, normalizedTableName, err := s.resolveSqliteQueryInputs(sessionName, dbPath, tableName)
	if err != nil {
		return SqliteQueryResult{}, err
	}

	db, err := openReadOnlySQLite(resolvedDBPath)
	if err != nil {
		return SqliteQueryResult{}, err
	}
	defer closeSQLiteDB(db, resolvedDBPath)

	resolvedTableName, ok, err := findSqliteTable(db, normalizedTableName)
	if err != nil {
		return SqliteQueryResult{}, fmt.Errorf("validate sqlite table %q: %w", normalizedTableName, err)
	}
	if !ok {
		slog.Warn("[DEVPANEL] sqlite table not found", "path", resolvedDBPath, "table", normalizedTableName)
		return SqliteQueryResult{}, sqliteTableUnavailableError(normalizedTableName)
	}

	clampedLimit := clampSqliteLimit(limit)
	clampedOffset := max(offset, 0)

	totalRows, err := countSqliteRows(db, resolvedTableName)
	if err != nil {
		if isMissingTableErr(err) {
			slog.Warn("[DEVPANEL] sqlite table disappeared during count", "path", resolvedDBPath, "table", resolvedTableName)
			return SqliteQueryResult{}, sqliteTableUnavailableError(resolvedTableName)
		}
		return SqliteQueryResult{}, fmt.Errorf("count sqlite rows for %q: %w", resolvedTableName, err)
	}

	columns, rows, nullMasks, err := querySqliteRows(db, resolvedTableName, clampedOffset, clampedLimit)
	if err != nil {
		if isMissingTableErr(err) {
			slog.Warn("[DEVPANEL] sqlite table disappeared during query", "path", resolvedDBPath, "table", resolvedTableName)
			return SqliteQueryResult{}, sqliteTableUnavailableError(resolvedTableName)
		}
		return SqliteQueryResult{}, fmt.Errorf("query sqlite table %q: %w", resolvedTableName, err)
	}

	return SqliteQueryResult{
		Columns:   columns,
		Rows:      rows,
		NullMasks: nullMasks,
		Offset:    clampedOffset,
		TotalRows: totalRows,
		Truncated: clampedOffset+int64(len(rows)) < totalRows,
	}, nil
}

// SqliteExportCSV writes a read-only table/view export to CSV under the session root.
func (s *Service) SqliteExportCSV(sessionName, dbPath, tableName, destRelPath string) (SqliteExportResult, error) {
	resolvedDBPath, normalizedTableName, err := s.resolveSqliteQueryInputs(sessionName, dbPath, tableName)
	if err != nil {
		return SqliteExportResult{}, err
	}

	sessionRoot, err := s.resolveSessionWorkDir(strings.TrimSpace(sessionName))
	if err != nil {
		return SqliteExportResult{}, err
	}

	normalizedDest := normalizePanelPath(destRelPath)
	if normalizedDest == "" {
		return SqliteExportResult{}, errors.New("destination path is required")
	}
	resolvedDestPath, err := s.ResolveAndValidateNewPath(sessionRoot, normalizedDest)
	if err != nil {
		return SqliteExportResult{}, err
	}
	if sameFilePath(resolvedDBPath, resolvedDestPath) {
		return SqliteExportResult{}, errors.New("destination path must differ from database path")
	}

	db, err := openReadOnlySQLite(resolvedDBPath)
	if err != nil {
		return SqliteExportResult{}, err
	}
	defer closeSQLiteDB(db, resolvedDBPath)

	resolvedTableName, ok, err := findSqliteTable(db, normalizedTableName)
	if err != nil {
		return SqliteExportResult{}, fmt.Errorf("validate sqlite table %q: %w", normalizedTableName, err)
	}
	if !ok {
		slog.Warn("[DEVPANEL] sqlite table not found during export", "path", resolvedDBPath, "table", normalizedTableName)
		return SqliteExportResult{}, sqliteTableUnavailableError(normalizedTableName)
	}

	rowCount, err := exportSqliteTableToCSV(db, resolvedTableName, resolvedDestPath)
	if err != nil {
		if isMissingTableErr(err) {
			slog.Warn("[DEVPANEL] sqlite table disappeared during export", "path", resolvedDBPath, "table", resolvedTableName)
			return SqliteExportResult{}, sqliteTableUnavailableError(resolvedTableName)
		}
		return SqliteExportResult{}, fmt.Errorf("export sqlite table %q: %w", resolvedTableName, err)
	}

	return SqliteExportResult{
		Path:     normalizedDest,
		RowCount: rowCount,
	}, nil
}

func (s *Service) resolveSqliteDBPath(sessionName, dbPath string) (resolvedDBPath string, normalizedSessionName string, err error) {
	normalizedSessionName = strings.TrimSpace(sessionName)
	if normalizedSessionName == "" {
		return "", "", errors.New("session name is required")
	}

	normalizedDBPath := normalizePanelPath(dbPath)
	if normalizedDBPath == "" {
		return "", "", errors.New("database path is required")
	}

	rootDir, err := s.resolveSessionWorkDir(normalizedSessionName)
	if err != nil {
		return "", "", err
	}

	resolvedDBPath, err = s.ResolveAndValidatePath(rootDir, normalizedDBPath)
	if err != nil {
		return "", "", err
	}

	return resolvedDBPath, normalizedSessionName, nil
}

func (s *Service) resolveSqliteQueryInputs(sessionName, dbPath, tableName string) (resolvedDBPath string, normalizedTableName string, err error) {
	resolvedDBPath, _, err = s.resolveSqliteDBPath(sessionName, dbPath)
	if err != nil {
		return "", "", err
	}

	normalizedTableName = strings.TrimSpace(tableName)
	if normalizedTableName == "" {
		return "", "", errors.New("table name is required")
	}
	return resolvedDBPath, normalizedTableName, nil
}

func openReadOnlySQLite(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", sqliteReadOnlyDSN(path))
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", path, err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	return db, nil
}

func closeSQLiteDB(db *sql.DB, path string) {
	if db == nil {
		return
	}
	if err := db.Close(); err != nil {
		slog.Warn("[DEVPANEL] failed to close sqlite database", "path", path, "error", err)
	}
}

func sqliteReadOnlyDSN(path string) string {
	return "file:" + path + "?mode=ro&_pragma=busy_timeout(5000)"
}

func listSqliteSchemaEntries(db *sql.DB) ([]sqliteSchemaEntry, error) {
	rows, err := db.Query(`
		SELECT name, type
		FROM sqlite_schema
		WHERE type IN ('table', 'view') AND name NOT LIKE 'sqlite_%'
		ORDER BY name COLLATE NOCASE
	`)
	if err != nil {
		return nil, err
	}
	defer closeSQLiteRows(rows, "schema query")

	entries := make([]sqliteSchemaEntry, 0)
	for rows.Next() {
		var entry sqliteSchemaEntry
		if err := rows.Scan(&entry.Name, &entry.Type); err != nil {
			return nil, fmt.Errorf("scan sqlite schema entry: %w", err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sqlite schema entries: %w", err)
	}
	return entries, nil
}

func findSqliteTable(db *sql.DB, tableName string) (string, bool, error) {
	var resolvedName string
	err := db.QueryRow(`
		SELECT name
		FROM sqlite_schema
		WHERE type IN ('table', 'view') AND name = ?
		LIMIT 1
	`, tableName).Scan(&resolvedName)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return resolvedName, true, nil
}

func loadSqliteColumns(db *sql.DB, tableName string) ([]SqliteColumnInfo, error) {
	query := fmt.Sprintf("PRAGMA table_info(%s)", quoteSqliteIdentifier(tableName))
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer closeSQLiteRows(rows, "pragma table_info")

	columns := make([]SqliteColumnInfo, 0)
	for rows.Next() {
		var column sqlitePragmaColumn
		if err := rows.Scan(
			&column.CID,
			&column.Name,
			&column.Type,
			&column.NotNull,
			&column.DefaultValue,
			&column.PrimaryKey,
		); err != nil {
			return nil, fmt.Errorf("scan sqlite pragma column: %w", err)
		}
		columns = append(columns, column.toColumnInfo())
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sqlite pragma columns: %w", err)
	}
	return columns, nil
}

func countSqliteRows(db *sql.DB, tableName string) (int64, error) {
	var count int64
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", quoteSqliteIdentifier(tableName))
	if err := db.QueryRow(query).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func querySqliteRows(db *sql.DB, tableName string, offset int64, limit int) ([]string, [][]string, [][]bool, error) {
	query := fmt.Sprintf("SELECT * FROM %s LIMIT ? OFFSET ?", quoteSqliteIdentifier(tableName))
	rows, err := db.Query(query, limit, offset)
	if err != nil {
		return nil, nil, nil, err
	}
	defer closeSQLiteRows(rows, "row query")

	columns, err := rows.Columns()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("sqlite row columns: %w", err)
	}

	resultRows := make([][]string, 0, min(limit, 64))
	nullMasks := make([][]bool, 0, min(limit, 64))
	scanValues := make([]any, len(columns))
	scanTargets := make([]any, len(columns))
	for i := range scanTargets {
		scanTargets[i] = &scanValues[i]
	}

	for rows.Next() {
		if err := rows.Scan(scanTargets...); err != nil {
			return nil, nil, nil, fmt.Errorf("scan sqlite row: %w", err)
		}
		resultRow := make([]string, len(columns))
		resultMask := make([]bool, len(columns))
		for i, value := range scanValues {
			if value == nil {
				resultMask[i] = true
				resultRow[i] = ""
				continue
			}
			resultRow[i] = stringifySQLiteValue(value)
		}
		resultRows = append(resultRows, resultRow)
		nullMasks = append(nullMasks, resultMask)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, nil, fmt.Errorf("iterate sqlite rows: %w", err)
	}

	return columns, resultRows, nullMasks, nil
}

func exportSqliteTableToCSV(db *sql.DB, tableName, destPath string) (int64, error) {
	query := fmt.Sprintf("SELECT * FROM %s", quoteSqliteIdentifier(tableName))
	rows, err := db.Query(query)
	if err != nil {
		return 0, err
	}
	defer closeSQLiteRows(rows, "csv export query")

	columns, err := rows.Columns()
	if err != nil {
		return 0, fmt.Errorf("sqlite export columns: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return 0, fmt.Errorf("create export directory: %w", err)
	}

	tempFile, err := os.CreateTemp(filepath.Dir(destPath), ".sqlite-export-*.csv")
	if err != nil {
		return 0, fmt.Errorf("create export temp file: %w", err)
	}
	tempPath := tempFile.Name()
	success := false
	closed := false
	defer func() {
		if !success {
			if !closed {
				if closeErr := tempFile.Close(); closeErr != nil {
					slog.Warn("[DEVPANEL] failed to close sqlite export temp file during cleanup", "path", tempPath, "error", closeErr)
				}
			}
			if removeErr := os.Remove(tempPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				slog.Warn("[DEVPANEL] failed to remove sqlite export temp file during cleanup", "path", tempPath, "error", removeErr)
			}
		}
	}()

	if err := tempFile.Chmod(0o644); err != nil {
		return 0, fmt.Errorf("chmod export temp file: %w", err)
	}

	writer := csv.NewWriter(tempFile)
	if err := writer.Write(columns); err != nil {
		return 0, fmt.Errorf("write csv header: %w", err)
	}

	scanValues := make([]any, len(columns))
	scanTargets := make([]any, len(columns))
	for i := range scanTargets {
		scanTargets[i] = &scanValues[i]
	}

	var rowCount int64
	for rows.Next() {
		if err := rows.Scan(scanTargets...); err != nil {
			return 0, fmt.Errorf("scan sqlite export row: %w", err)
		}
		record := make([]string, len(columns))
		for i, value := range scanValues {
			if value != nil {
				record[i] = stringifySQLiteValue(value)
			}
		}
		if err := writer.Write(record); err != nil {
			return 0, fmt.Errorf("write csv row: %w", err)
		}
		rowCount++
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("iterate sqlite export rows: %w", err)
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return 0, fmt.Errorf("flush csv writer: %w", err)
	}
	if err := tempFile.Sync(); err != nil {
		return 0, fmt.Errorf("sync export temp file: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return 0, fmt.Errorf("close export temp file: %w", err)
	}
	closed = true

	if err := retryFileOperation(func() error {
		return os.Rename(tempPath, destPath)
	}, "rename sqlite export temp file"); err != nil {
		return 0, fmt.Errorf("rename export temp file: %w", err)
	}

	success = true
	return rowCount, nil
}

func closeSQLiteRows(rows *sql.Rows, operation string) {
	if rows == nil {
		return
	}
	if err := rows.Close(); err != nil {
		slog.Warn("[DEVPANEL] failed to close sqlite rows", "operation", operation, "error", err)
	}
}

func clampSqliteLimit(limit int) int {
	switch {
	case limit <= 0:
		return defaultSqliteQueryLimit
	case limit > maxSqliteQueryLimit:
		return maxSqliteQueryLimit
	default:
		return limit
	}
}

func sqliteTableUnavailableError(tableName string) error {
	return fmt.Errorf("sqlite table or view %q is no longer available", tableName)
}

func quoteSqliteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func stringifySQLiteValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case []byte:
		return string(typed)
	case string:
		return typed
	case time.Time:
		return typed.Format(time.RFC3339Nano)
	case bool:
		return strconv.FormatBool(typed)
	case int:
		return strconv.Itoa(typed)
	case int8:
		return strconv.FormatInt(int64(typed), 10)
	case int16:
		return strconv.FormatInt(int64(typed), 10)
	case int32:
		return strconv.FormatInt(int64(typed), 10)
	case int64:
		return strconv.FormatInt(typed, 10)
	case uint:
		return strconv.FormatUint(uint64(typed), 10)
	case uint8:
		return strconv.FormatUint(uint64(typed), 10)
	case uint16:
		return strconv.FormatUint(uint64(typed), 10)
	case uint32:
		return strconv.FormatUint(uint64(typed), 10)
	case uint64:
		return strconv.FormatUint(typed, 10)
	case float32:
		return strconv.FormatFloat(float64(typed), 'f', -1, 32)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	default:
		if stringer, ok := value.(fmt.Stringer); ok {
			return stringer.String()
		}
		return fmt.Sprint(value)
	}
}

func sameFilePath(left, right string) bool {
	cleanLeft := filepath.Clean(left)
	cleanRight := filepath.Clean(right)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(cleanLeft, cleanRight)
	}
	return cleanLeft == cleanRight
}

func isMissingTableErr(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "no such table") || strings.Contains(message, "no such view")
}
