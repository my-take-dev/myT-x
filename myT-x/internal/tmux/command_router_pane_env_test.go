package tmux

import (
	"sync"
	"testing"
)

func TestUpdatePaneEnvBasic(t *testing.T) {
	router := NewCommandRouter(nil, nil, RouterOptions{})

	// Initially nil/empty.
	if got := router.getPaneEnv(); got != nil {
		t.Fatalf("getPaneEnv() before update = %v, want nil", got)
	}

	// Update with new values.
	router.UpdatePaneEnv(map[string]string{"KEY1": "val1", "KEY2": "val2"})

	got := router.getPaneEnv()
	if len(got) != 2 {
		t.Fatalf("getPaneEnv() len = %d, want 2", len(got))
	}
	if got["KEY1"] != "val1" {
		t.Errorf("getPaneEnv()[KEY1] = %q, want %q", got["KEY1"], "val1")
	}
	if got["KEY2"] != "val2" {
		t.Errorf("getPaneEnv()[KEY2] = %q, want %q", got["KEY2"], "val2")
	}
}

func TestUpdatePaneEnvOverwrite(t *testing.T) {
	router := NewCommandRouter(nil, nil, RouterOptions{
		PaneEnv: map[string]string{"OLD": "old"},
	})

	router.UpdatePaneEnv(map[string]string{"NEW": "new"})

	got := router.getPaneEnv()
	if _, exists := got["OLD"]; exists {
		t.Error("getPaneEnv() still contains OLD key after overwrite")
	}
	if got["NEW"] != "new" {
		t.Errorf("getPaneEnv()[NEW] = %q, want %q", got["NEW"], "new")
	}
}

func TestUpdatePaneEnvDeepCopy(t *testing.T) {
	router := NewCommandRouter(nil, nil, RouterOptions{})

	input := map[string]string{"A": "1"}
	router.UpdatePaneEnv(input)

	// Mutate the input map after update.
	input["A"] = "mutated"
	input["B"] = "extra"

	got := router.getPaneEnv()
	if got["A"] != "1" {
		t.Errorf("getPaneEnv()[A] = %q, want %q (input mutation leaked)", got["A"], "1")
	}
	if _, exists := got["B"]; exists {
		t.Error("getPaneEnv() contains B key (input mutation leaked)")
	}
}

func TestGetPaneEnvReturnsIndependentCopy(t *testing.T) {
	router := NewCommandRouter(nil, nil, RouterOptions{
		PaneEnv: map[string]string{"X": "1"},
	})

	copy1 := router.getPaneEnv()
	copy1["X"] = "mutated"

	copy2 := router.getPaneEnv()
	if copy2["X"] != "1" {
		t.Errorf("getPaneEnv() returned shared reference; copy2[X] = %q, want %q", copy2["X"], "1")
	}
}

func TestUpdatePaneEnvConcurrent(t *testing.T) {
	router := NewCommandRouter(nil, nil, RouterOptions{})

	const goroutines = 20
	const iterations = 100

	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	// Writers.
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				router.UpdatePaneEnv(map[string]string{
					"KEY": "value",
				})
			}
		}(i)
	}

	// Readers.
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_ = router.getPaneEnv()
			}
		}()
	}

	wg.Wait()
}

func TestUpdatePaneEnvEmpty(t *testing.T) {
	router := NewCommandRouter(nil, nil, RouterOptions{
		PaneEnv: map[string]string{"A": "1"},
	})

	// Update with empty map clears PaneEnv.
	router.UpdatePaneEnv(map[string]string{})

	got := router.getPaneEnv()
	if got != nil {
		t.Errorf("getPaneEnv() after empty update = %v, want nil", got)
	}
}
