package tmux

import (
	"testing"

	"myT-x/internal/mcp/agent-orchestrator/domain"
)

func TestParseListPanesOutput(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []domain.PaneInfo
	}{
		{
			name:  "empty input",
			input: "",
			want:  nil,
		},
		{
			name:  "single pane",
			input: "%0\tbash\tmain\t0\n",
			want: []domain.PaneInfo{
				{ID: "%0", Title: "bash", Session: "main", Window: "0"},
			},
		},
		{
			name:  "multiple panes",
			input: "%0\torchestrator\tmain\t0\n%1\tcodex:バックエンド実装\tmain\t1\n%2\tclaude2:コードレビュー\twork\t0\n",
			want: []domain.PaneInfo{
				{ID: "%0", Title: "orchestrator", Session: "main", Window: "0"},
				{ID: "%1", Title: "codex:バックエンド実装", Session: "main", Window: "1"},
				{ID: "%2", Title: "claude2:コードレビュー", Session: "work", Window: "0"},
			},
		},
		{
			name:  "pane with minimal fields",
			input: "%5\n",
			want: []domain.PaneInfo{
				{ID: "%5"},
			},
		},
		{
			name:  "blank lines ignored",
			input: "\n%0\tbash\tmain\t0\n\n",
			want: []domain.PaneInfo{
				{ID: "%0", Title: "bash", Session: "main", Window: "0"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseListPanesOutput(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d panes, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("pane[%d] = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}
