package tmux

import (
	"testing"
)

func collectLeafPaneIDs(node *LayoutNode) []int {
	if node == nil {
		return nil
	}
	if node.Type == LayoutLeaf {
		return []int{node.PaneID}
	}
	var ids []int
	ids = append(ids, collectLeafPaneIDs(node.Children[0])...)
	ids = append(ids, collectLeafPaneIDs(node.Children[1])...)
	return ids
}

func countLeaves(node *LayoutNode) int {
	return len(collectLeafPaneIDs(node))
}

func TestBuildPresetLayout(t *testing.T) {
	tests := []struct {
		name      string
		preset    LayoutPreset
		paneIDs   []int
		wantLeafs int
		wantNil   bool
	}{
		{"empty panes", PresetEvenHorizontal, nil, 0, true},
		{"single pane even-h", PresetEvenHorizontal, []int{1}, 1, false},
		{"2 panes even-h", PresetEvenHorizontal, []int{1, 2}, 2, false},
		{"3 panes even-h", PresetEvenHorizontal, []int{1, 2, 3}, 3, false},
		{"5 panes even-h", PresetEvenHorizontal, []int{1, 2, 3, 4, 5}, 5, false},
		{"2 panes even-v", PresetEvenVertical, []int{1, 2}, 2, false},
		{"4 panes even-v", PresetEvenVertical, []int{1, 2, 3, 4}, 4, false},
		{"3 panes main-v", PresetMainVertical, []int{1, 2, 3}, 3, false},
		{"4 panes main-v", PresetMainVertical, []int{1, 2, 3, 4}, 4, false},
		{"2 panes main-v fallback", PresetMainVertical, []int{1, 2}, 2, false},
		{"3 panes main-h", PresetMainHorizontal, []int{1, 2, 3}, 3, false},
		{"4 panes main-h", PresetMainHorizontal, []int{1, 2, 3, 4}, 4, false},
		{"4 panes tiled", PresetTiled, []int{1, 2, 3, 4}, 4, false},
		{"5 panes tiled", PresetTiled, []int{1, 2, 3, 4, 5}, 5, false},
		{"6 panes tiled", PresetTiled, []int{1, 2, 3, 4, 5, 6}, 6, false},
		{"2 panes tiled", PresetTiled, []int{1, 2}, 2, false},
		{"unknown preset fallback", LayoutPreset("unknown"), []int{1, 2, 3}, 3, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildPresetLayout(tt.preset, tt.paneIDs)
			if tt.wantNil {
				if got != nil {
					t.Fatalf("expected nil, got %v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected non-nil layout")
			}
			leaves := countLeaves(got)
			if leaves != tt.wantLeafs {
				t.Errorf("leaf count = %d, want %d", leaves, tt.wantLeafs)
			}
			// All original pane IDs should be present
			gotIDs := collectLeafPaneIDs(got)
			idSet := make(map[int]bool)
			for _, id := range gotIDs {
				idSet[id] = true
			}
			for _, id := range tt.paneIDs {
				if !idSet[id] {
					t.Errorf("pane ID %d missing from layout", id)
				}
			}
		})
	}
}

func TestBuildPresetLayout_MainVerticalStructure(t *testing.T) {
	// main-vertical with 3 panes: left main (60%) + right sub (vertical split)
	layout := BuildPresetLayout(PresetMainVertical, []int{10, 20, 30})
	if layout == nil {
		t.Fatal("expected non-nil layout")
	}
	if layout.Type != LayoutSplit {
		t.Fatalf("root type = %s, want split", layout.Type)
	}
	if layout.Direction != SplitHorizontal {
		t.Errorf("root direction = %s, want horizontal", layout.Direction)
	}
	if layout.Ratio != 0.6 {
		t.Errorf("root ratio = %f, want 0.6", layout.Ratio)
	}
	// Left child should be leaf with pane 10
	left := layout.Children[0]
	if left == nil || left.Type != LayoutLeaf || left.PaneID != 10 {
		t.Errorf("left child: want leaf pane 10, got %+v", left)
	}
	// Right child should be a split with panes 20 and 30
	right := layout.Children[1]
	if right == nil || right.Type != LayoutSplit {
		t.Fatalf("right child: want split, got %+v", right)
	}
	if right.Direction != SplitVertical {
		t.Errorf("right direction = %s, want vertical", right.Direction)
	}
}

func TestBuildPresetLayout_TiledGrid(t *testing.T) {
	// 4 panes should create a 2x2 grid
	layout := BuildPresetLayout(PresetTiled, []int{1, 2, 3, 4})
	if layout == nil {
		t.Fatal("expected non-nil layout")
	}
	leaves := countLeaves(layout)
	if leaves != 4 {
		t.Errorf("leaf count = %d, want 4", leaves)
	}
}
