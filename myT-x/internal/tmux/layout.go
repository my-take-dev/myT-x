package tmux

// LayoutNodeType is the node category in pane layout tree.
type LayoutNodeType string

const (
	LayoutLeaf  LayoutNodeType = "leaf"
	LayoutSplit LayoutNodeType = "split"
)

// SplitDirection is the pane split direction.
type SplitDirection string

const (
	SplitHorizontal SplitDirection = "horizontal"
	SplitVertical   SplitDirection = "vertical"
)

// LayoutNode is a binary tree representation of tmux pane layout.
type LayoutNode struct {
	Type      LayoutNodeType `json:"type"`
	Direction SplitDirection `json:"direction,omitempty"`
	Ratio     float64        `json:"ratio,omitempty"`
	PaneID    int            `json:"pane_id"`
	Children  [2]*LayoutNode `json:"children,omitempty"`
}

func newLeafLayout(paneID int) *LayoutNode {
	return &LayoutNode{
		Type:   LayoutLeaf,
		PaneID: paneID,
	}
}

func cloneLayout(node *LayoutNode) *LayoutNode {
	if node == nil {
		return nil
	}
	out := &LayoutNode{
		Type:      node.Type,
		Direction: node.Direction,
		Ratio:     node.Ratio,
		PaneID:    node.PaneID,
	}
	out.Children[0] = cloneLayout(node.Children[0])
	out.Children[1] = cloneLayout(node.Children[1])
	return out
}

func splitLayout(root *LayoutNode, targetPaneID int, direction SplitDirection, newPaneID int) (*LayoutNode, bool) {
	if root == nil {
		return nil, false
	}
	if root.Type == LayoutLeaf && root.PaneID == targetPaneID {
		return &LayoutNode{
			Type:      LayoutSplit,
			Direction: direction,
			Ratio:     0.5,
			Children: [2]*LayoutNode{
				newLeafLayout(targetPaneID),
				newLeafLayout(newPaneID),
			},
		}, true
	}
	if root.Type != LayoutSplit {
		return root, false
	}

	if next, ok := splitLayout(root.Children[0], targetPaneID, direction, newPaneID); ok {
		root.Children[0] = next
		return root, true
	}
	if next, ok := splitLayout(root.Children[1], targetPaneID, direction, newPaneID); ok {
		root.Children[1] = next
		return root, true
	}
	return root, false
}

func swapPaneIDsInLayout(root *LayoutNode, sourcePaneID int, targetPaneID int) *LayoutNode {
	if root == nil {
		return nil
	}
	if root.Type == LayoutLeaf {
		switch root.PaneID {
		case sourcePaneID:
			root.PaneID = targetPaneID
		case targetPaneID:
			root.PaneID = sourcePaneID
		}
		return root
	}
	root.Children[0] = swapPaneIDsInLayout(root.Children[0], sourcePaneID, targetPaneID)
	root.Children[1] = swapPaneIDsInLayout(root.Children[1], sourcePaneID, targetPaneID)
	return root
}

// --- Layout Presets ---

// LayoutPreset identifies a named layout arrangement.
type LayoutPreset string

const (
	PresetEvenHorizontal LayoutPreset = "even-horizontal"
	PresetEvenVertical   LayoutPreset = "even-vertical"
	PresetMainVertical   LayoutPreset = "main-vertical"
	PresetMainHorizontal LayoutPreset = "main-horizontal"
	PresetTiled          LayoutPreset = "tiled"
)

// BuildPresetLayout creates a layout tree from a preset for the given pane IDs.
func BuildPresetLayout(preset LayoutPreset, paneIDs []int) *LayoutNode {
	if len(paneIDs) == 0 {
		return nil
	}
	if len(paneIDs) == 1 {
		return newLeafLayout(paneIDs[0])
	}
	switch preset {
	case PresetEvenHorizontal:
		return buildEvenSplit(paneIDs, SplitHorizontal)
	case PresetEvenVertical:
		return buildEvenSplit(paneIDs, SplitVertical)
	case PresetMainVertical:
		return buildMainSplit(paneIDs, SplitHorizontal, SplitVertical)
	case PresetMainHorizontal:
		return buildMainSplit(paneIDs, SplitVertical, SplitHorizontal)
	case PresetTiled:
		return buildTiledLayout(paneIDs)
	default:
		return buildEvenSplit(paneIDs, SplitHorizontal)
	}
}

// buildEvenSplit creates a balanced binary tree with even ratios.
func buildEvenSplit(paneIDs []int, dir SplitDirection) *LayoutNode {
	if len(paneIDs) == 1 {
		return newLeafLayout(paneIDs[0])
	}
	mid := len(paneIDs) / 2
	return &LayoutNode{
		Type:      LayoutSplit,
		Direction: dir,
		Ratio:     float64(mid) / float64(len(paneIDs)),
		Children: [2]*LayoutNode{
			buildEvenSplit(paneIDs[:mid], dir),
			buildEvenSplit(paneIDs[mid:], dir),
		},
	}
}

// buildMainSplit creates a main pane (60%) + evenly split sub panes.
func buildMainSplit(paneIDs []int, mainDir, subDir SplitDirection) *LayoutNode {
	if len(paneIDs) <= 2 {
		return buildEvenSplit(paneIDs, mainDir)
	}
	return &LayoutNode{
		Type:      LayoutSplit,
		Direction: mainDir,
		Ratio:     0.6,
		Children: [2]*LayoutNode{
			newLeafLayout(paneIDs[0]),
			buildEvenSplit(paneIDs[1:], subDir),
		},
	}
}

// buildTiledLayout creates a grid-like arrangement.
func buildTiledLayout(paneIDs []int) *LayoutNode {
	n := len(paneIDs)
	if n <= 2 {
		return buildEvenSplit(paneIDs, SplitHorizontal)
	}
	cols := 2
	if n > 4 {
		cols = 3
	}
	rows := (n + cols - 1) / cols
	rowNodes := make([]*LayoutNode, 0, rows)
	for r := 0; r < rows; r++ {
		start := r * cols
		end := start + cols
		if end > n {
			end = n
		}
		rowNodes = append(rowNodes, buildEvenSplit(paneIDs[start:end], SplitHorizontal))
	}
	return buildEvenSplitNodes(rowNodes, SplitVertical)
}

// buildEvenSplitNodes builds a balanced binary tree from pre-built nodes.
func buildEvenSplitNodes(nodes []*LayoutNode, dir SplitDirection) *LayoutNode {
	if len(nodes) == 1 {
		return nodes[0]
	}
	mid := len(nodes) / 2
	return &LayoutNode{
		Type:      LayoutSplit,
		Direction: dir,
		Ratio:     float64(mid) / float64(len(nodes)),
		Children: [2]*LayoutNode{
			buildEvenSplitNodes(nodes[:mid], dir),
			buildEvenSplitNodes(nodes[mid:], dir),
		},
	}
}

// removePaneFromLayout removes one pane leaf from layout tree while preserving
// existing split directions/ratios whenever possible.
func removePaneFromLayout(root *LayoutNode, paneID int) (*LayoutNode, bool) {
	if root == nil {
		return nil, false
	}
	if root.Type == LayoutLeaf {
		if root.PaneID == paneID {
			return nil, true
		}
		return root, false
	}
	if root.Type != LayoutSplit {
		return root, false
	}

	left, removedLeft := removePaneFromLayout(root.Children[0], paneID)
	right, removedRight := removePaneFromLayout(root.Children[1], paneID)
	if !removedLeft && !removedRight {
		return root, false
	}

	root.Children[0] = left
	root.Children[1] = right

	switch {
	case left == nil && right == nil:
		return nil, true
	case left == nil:
		return right, true
	case right == nil:
		return left, true
	default:
		return root, true
	}
}
