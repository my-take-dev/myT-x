package codex

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

func TestReadSQLiteMissingFile(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "does-not-exist.sqlite")
	summary, err := ReadSQLite(missing, "cwd")
	if err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
	if summary.Available {
		t.Error("missing file should report Available=false")
	}
}

func TestReadSQLiteEmptyPath(t *testing.T) {
	summary, err := ReadSQLite("", "cwd")
	if err != nil {
		t.Fatalf("empty path should not error: %v", err)
	}
	if summary.Available {
		t.Error("empty path should report Available=false")
	}
}

func TestReadSQLiteReadsThreadsCaseInsensitively(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "state_5.sqlite")

	db, err := sql.Open("sqlite", "file:"+dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			t.Fatalf("close sqlite: %v", closeErr)
		}
	}()

	if _, err := db.Exec(`
		CREATE TABLE threads (
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			model TEXT NOT NULL,
			tokens_used INTEGER NOT NULL,
			cwd TEXT NOT NULL,
			archived INTEGER NOT NULL
		)
	`); err != nil {
		t.Fatalf("create threads: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO threads (created_at, updated_at, model, tokens_used, cwd, archived)
		VALUES
			(1715731200, 1715734800, 'gpt-5', 100, 'D:\myT-x\DEV-myT-x', 0),
			(1715817600, 1715821200, 'gpt-5-mini', 40, 'd:\MYT-X\dev-myT-x', 0),
			(1715904000, 1715907600, 'ignored', 999, 'd:\MYT-X\dev-myT-x', 1)
	`); err != nil {
		t.Fatalf("insert threads: %v", err)
	}

	summary, err := ReadSQLite(dbPath, `d:\myt-x\dev-myT-x`)
	if err != nil {
		t.Fatalf("ReadSQLite: %v", err)
	}
	if !summary.Available {
		t.Fatal("summary should report Available=true")
	}
	if summary.TotalThreads != 2 {
		t.Fatalf("TotalThreads = %d, want 2", summary.TotalThreads)
	}
	if summary.ActiveDays != 2 {
		t.Fatalf("ActiveDays = %d, want 2", summary.ActiveDays)
	}
	if summary.TokensUsed != 140 {
		t.Fatalf("TokensUsed = %d, want 140", summary.TokensUsed)
	}
	if summary.Models["gpt-5"] != 1 || summary.Models["gpt-5-mini"] != 1 {
		t.Fatalf("Models = %+v", summary.Models)
	}
	wantUpdatedAt := time.Unix(1715821200, 0).UTC()
	if !summary.LastUpdatedAt.Equal(wantUpdatedAt) {
		t.Fatalf("LastUpdatedAt = %v, want %v", summary.LastUpdatedAt, wantUpdatedAt)
	}
}

func TestIsMissingTableErr(t *testing.T) {
	tests := []struct {
		name string
		msg  string
		want bool
	}{
		{"match lowercase", "SQL logic error: no such table: foo (1)", true},
		{"match mixed", "No Such Table: threads", true},
		{"different error", "database is locked", false},
		{"empty", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isMissingTableErr(stringError(tc.msg))
			if got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

type stringError string

func (e stringError) Error() string { return string(e) }
