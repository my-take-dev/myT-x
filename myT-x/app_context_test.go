package main

import (
	"context"
	"sync"
	"testing"
)

func TestRuntimeContextSetAndGet(t *testing.T) {
	app := NewApp()
	if app.runtimeContext() != nil {
		t.Fatal("runtimeContext() should be nil before startup context is set")
	}

	want := context.Background()
	app.setRuntimeContext(want)
	if got := app.runtimeContext(); got != want {
		t.Fatalf("runtimeContext() = %v, want %v", got, want)
	}
}

func TestRuntimeContextConcurrentSetGet(t *testing.T) {
	app := NewApp()

	const goroutines = 8
	const iterations = 200

	var wg sync.WaitGroup
	start := make(chan struct{})

	for i := 0; i < goroutines; i++ {
		wg.Add(2)

		go func(id int) {
			defer wg.Done()
			<-start
			for j := 0; j < iterations; j++ {
				if (id+j)%2 == 0 {
					app.setRuntimeContext(context.Background())
				} else {
					app.setRuntimeContext(nil)
				}
			}
		}(i)

		go func() {
			defer wg.Done()
			<-start
			for j := 0; j < iterations; j++ {
				_ = app.runtimeContext()
			}
		}()
	}

	close(start)
	wg.Wait()

	app.setRuntimeContext(context.Background())
	if app.runtimeContext() == nil {
		t.Fatal("runtimeContext() should return the last set non-nil context")
	}
}
