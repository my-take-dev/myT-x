package tmux

import (
	"strings"

	"myT-x/internal/mcp/agent-orchestrator/domain"
)

// ParseListPanesOutput は list-panes -a の出力をパースする。
// フォーマット: #{pane_id}\t#{pane_title}\t#{session_name}\t#{window_index}
func ParseListPanesOutput(output string) []domain.PaneInfo {
	var panes []domain.PaneInfo
	for line := range strings.SplitSeq(strings.TrimSpace(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.SplitN(line, "\t", 4)
		p := domain.PaneInfo{ID: fields[0]}
		if len(fields) > 1 {
			p.Title = fields[1]
		}
		if len(fields) > 2 {
			p.Session = fields[2]
		}
		if len(fields) > 3 {
			p.Window = fields[3]
		}
		panes = append(panes, p)
	}
	return panes
}
