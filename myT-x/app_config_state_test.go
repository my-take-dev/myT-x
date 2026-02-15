package main

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"myT-x/internal/config"
)

func TestGetConfigSnapshotReturnsIndependentCopy(t *testing.T) {
	app := &App{}
	base := config.DefaultConfig()
	app.setConfigSnapshot(base)

	snapshot := app.getConfigSnapshot()
	snapshot.Keys["snapshot-only"] = "value"
	snapshot.Worktree.SetupScripts = append(snapshot.Worktree.SetupScripts, "snapshot-script")

	latest := app.getConfigSnapshot()
	if _, exists := latest.Keys["snapshot-only"]; exists {
		t.Fatal("getConfigSnapshot returned shared map reference")
	}
	if len(latest.Worktree.SetupScripts) != len(base.Worktree.SetupScripts) {
		t.Fatal("getConfigSnapshot returned shared slice reference")
	}
}

func TestConfigSnapshotConcurrency(t *testing.T) {
	app := &App{}
	app.setConfigSnapshot(config.DefaultConfig())

	const goroutines = 12
	const iterations = 200

	var wg sync.WaitGroup
	start := make(chan struct{})

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			<-start

			for j := 0; j < iterations; j++ {
				cfg := app.getConfigSnapshot()
				cfg.Keys[fmt.Sprintf("goroutine-%d", id)] = fmt.Sprintf("%d", j)
				cfg.Worktree.SetupScripts = append(cfg.Worktree.SetupScripts, fmt.Sprintf("script-%d-%d", id, j))
				if id%2 == 0 {
					app.setConfigSnapshot(cfg)
					continue
				}
				_ = app.getConfigSnapshot()
			}
		}(i)
	}

	close(start)
	wg.Wait()

	final := app.getConfigSnapshot()
	if final.Shell == "" {
		t.Fatal("config corruption detected: shell should not be empty")
	}
	if final.Keys == nil {
		t.Fatal("config corruption detected: keys should not be nil")
	}
	foundWriterKey := false
	for key := range final.Keys {
		if strings.HasPrefix(key, "goroutine-") {
			foundWriterKey = true
			break
		}
	}
	if !foundWriterKey {
		t.Fatal("config snapshot should include at least one writer key")
	}
	foundScript := false
	for _, script := range final.Worktree.SetupScripts {
		if script != "" {
			foundScript = true
			break
		}
	}
	if !foundScript {
		t.Fatal("config snapshot should include at least one writer-generated setup script")
	}
}
