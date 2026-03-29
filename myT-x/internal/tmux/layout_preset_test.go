package tmux

import (
	"maps"
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
	// main-vertical with 3 panes: left main (uniform 1/3) + right sub (vertical split)
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
	wantRatio := 1.0 / 3.0
	if layout.Ratio != wantRatio {
		t.Errorf("root ratio = %f, want %f (uniform 1/N)", layout.Ratio, wantRatio)
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

// computeLeafArea recursively computes the fraction of total area each leaf pane receives.
func computeLeafArea(node *LayoutNode, area float64) map[int]float64 {
	if node == nil {
		return nil
	}
	if node.Type == LayoutLeaf {
		return map[int]float64{node.PaneID: area}
	}
	result := make(map[int]float64)
	leftArea := area * node.Ratio
	rightArea := area * (1.0 - node.Ratio)
	maps.Copy(result, computeLeafArea(node.Children[0], leftArea))
	maps.Copy(result, computeLeafArea(node.Children[1], rightArea))
	return result
}

func TestBuildPresetLayout_UniformRatios(t *testing.T) {
	const tolerance = 1e-9

	tests := []struct {
		name    string
		preset  LayoutPreset
		paneIDs []int
	}{
		{"main-vertical 3 panes", PresetMainVertical, []int{1, 2, 3}},
		{"main-vertical 4 panes", PresetMainVertical, []int{1, 2, 3, 4}},
		{"main-vertical 5 panes", PresetMainVertical, []int{1, 2, 3, 4, 5}},
		{"main-horizontal 3 panes", PresetMainHorizontal, []int{1, 2, 3}},
		{"main-horizontal 4 panes", PresetMainHorizontal, []int{1, 2, 3, 4}},
		{"even-horizontal 3 panes", PresetEvenHorizontal, []int{1, 2, 3}},
		{"even-vertical 4 panes", PresetEvenVertical, []int{1, 2, 3, 4}},
		{"tiled 4 panes", PresetTiled, []int{1, 2, 3, 4}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layout := BuildPresetLayout(tt.preset, tt.paneIDs)
			if layout == nil {
				t.Fatal("expected non-nil layout")
			}
			areas := computeLeafArea(layout, 1.0)
			wantArea := 1.0 / float64(len(tt.paneIDs))
			for paneID, area := range areas {
				diff := area - wantArea
				if diff < 0 {
					diff = -diff
				}
				if diff > tolerance {
					t.Errorf("pane %d: area = %f, want %f (diff %e)", paneID, area, wantArea, diff)
				}
			}
		})
	}
}

func TestBuildPresetLayout_MainVertical2PaneFallback(t *testing.T) {
	// 2 panes should fallback to buildEvenSplit (ratio 0.5, mainDir=horizontal)
	layout := BuildPresetLayout(PresetMainVertical, []int{1, 2})
	if layout == nil {
		t.Fatal("expected non-nil layout")
	}
	if layout.Ratio != 0.5 {
		t.Errorf("2-pane main-vertical ratio = %f, want 0.5 (even split fallback)", layout.Ratio)
	}
	if layout.Direction != SplitHorizontal {
		t.Errorf("2-pane main-vertical direction = %s, want %s", layout.Direction, SplitHorizontal)
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
