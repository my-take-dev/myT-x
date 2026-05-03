package orchestratorstorage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"myT-x/internal/sessioninfo"
)

func TestDBPathUsesSessionInfoDirectory(t *testing.T) {
	configDir := filepath.Join(t.TempDir(), "config")
	workDir := filepath.Join(t.TempDir(), "workspace")
	key, err := sessioninfo.FolderKey(workDir)
	if err != nil {
		t.Fatalf("FolderKey(): %v", err)
	}

	got, err := DBPath(configDir, workDir)
	if err != nil {
		t.Fatalf("DBPath(): %v", err)
	}

	want := filepath.Join(configDir, sessioninfo.DirName, key, "orchestrator.db")
	if got != want {
		t.Fatalf("DBPath() = %q, want %q", got, want)
	}
}

func TestDBPathRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name      string
		configDir string
		workDir   string
		wantErr   string
	}{
		{
			name:      "empty config dir",
			configDir: " ",
			workDir:   t.TempDir(),
			wantErr:   "config dir",
		},
		{
			name:      "empty work dir",
			configDir: t.TempDir(),
			workDir:   " ",
			wantErr:   "workDir",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DBPath(tt.configDir, tt.workDir)
			if err == nil {
				t.Fatal("DBPath() expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("DBPath() error = %v, want to contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestDBPathDoesNotCreateProjectStorageDirectory(t *testing.T) {
	configDir := filepath.Join(t.TempDir(), "config")
	workDir := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(workDir): %v", err)
	}

	got, err := DBPath(configDir, workDir)
	if err != nil {
		t.Fatalf("DBPath(): %v", err)
	}
	if wantPrefix := filepath.Join(configDir, sessioninfo.DirName) + string(os.PathSeparator); !strings.HasPrefix(got, wantPrefix) {
		t.Fatalf("DBPath() = %q, want under %q", got, wantPrefix)
	}
	if strings.Contains(got, filepath.Join(workDir, ".myT-x")) {
		t.Fatalf("DBPath() = %q, want outside project .myT-x", got)
	}
	if _, err := os.Stat(filepath.Join(workDir, ".myT-x")); !os.IsNotExist(err) {
		t.Fatalf("project .myT-x stat error = %v, want not exist", err)
	}
}
