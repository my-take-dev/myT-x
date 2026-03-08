package tmux

import (
	"bytes"
	"sync"
	"testing"
)

func TestPaneOutputHistory_Write_Capture(t *testing.T) {
	tests := []struct {
		name     string
		capacity int
		writes   [][]byte
		want     []byte
	}{
		{
			name:     "empty buffer capture returns nil",
			capacity: 100,
			writes:   nil,
			want:     nil,
		},
		{
			name:     "single write under capacity",
			capacity: 100,
			writes:   [][]byte{[]byte("hello")},
			want:     []byte("hello"),
		},
		{
			name:     "multiple small writes concatenated",
			capacity: 100,
			writes:   [][]byte{[]byte("hello"), []byte(" "), []byte("world")},
			want:     []byte("hello world"),
		},
		{
			name:     "write exactly at capacity",
			capacity: 5,
			writes:   [][]byte{[]byte("hello")},
			want:     []byte("hello"),
		},
		{
			name:     "write exceeding capacity single wrap",
			capacity: 10,
			writes:   [][]byte{[]byte("hello"), []byte("world123")},
			want:     []byte("loworld123"),
		},
		{
			name:     "write data larger than capacity",
			capacity: 5,
			writes:   [][]byte{[]byte("hello world!")},
			want:     []byte("orld!"),
		},
		{
			name:     "multiple writes with buffer wrap",
			capacity: 10,
			writes: [][]byte{
				[]byte("12345"),
				[]byte("67890"),
				[]byte("ABCDE"),
			},
			want: []byte("67890ABCDE"),
		},
		{
			name:     "empty write has no effect",
			capacity: 100,
			writes:   [][]byte{[]byte("data"), []byte{}, []byte("more")},
			want:     []byte("datamore"),
		},
		{
			name:     "write after capacity filled multiple times",
			capacity: 5,
			writes: [][]byte{
				[]byte("AAAAA"),
				[]byte("BBBBB"),
				[]byte("CCCCC"),
			},
			want: []byte("CCCCC"),
		},
		{
			name:     "complex wrap-around scenario",
			capacity: 8,
			writes: [][]byte{
				[]byte("1234"),
				[]byte("5678"),
				[]byte("9ABC"),
			},
			want: []byte("56789ABC"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewPaneOutputHistory(tt.capacity)

			for _, data := range tt.writes {
				h.Write(data)
			}

			got := h.Capture()
			if !bytes.Equal(got, tt.want) {
				t.Errorf("Capture() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPaneOutputHistory_Reset(t *testing.T) {
	tests := []struct {
		name     string
		capacity int
		writes   [][]byte
	}{
		{
			name:     "reset after single write",
			capacity: 100,
			writes:   [][]byte{[]byte("hello")},
		},
		{
			name:     "reset after multiple writes",
			capacity: 50,
			writes:   [][]byte{[]byte("a"), []byte("b"), []byte("c")},
		},
		{
			name:     "reset on empty buffer",
			capacity: 100,
			writes:   nil,
		},
		{
			name:     "reset after capacity exceeded",
			capacity: 5,
			writes:   [][]byte{[]byte("hello world")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewPaneOutputHistory(tt.capacity)

			for _, data := range tt.writes {
				h.Write(data)
			}

			h.Reset()
			got := h.Capture()

			if got != nil {
				t.Errorf("Capture() after Reset() = %v, want nil", got)
			}
		})
	}
}

func TestPaneOutputHistory_ResetAndReuse(t *testing.T) {
	h := NewPaneOutputHistory(10)

	h.Write([]byte("first"))
	if !bytes.Equal(h.Capture(), []byte("first")) {
		t.Fatal("first write failed")
	}

	h.Reset()
	if h.Capture() != nil {
		t.Fatal("reset failed")
	}

	h.Write([]byte("second"))
	if !bytes.Equal(h.Capture(), []byte("second")) {
		t.Fatal("second write after reset failed")
	}
}

func TestPaneOutputHistory_Release(t *testing.T) {
	h := NewPaneOutputHistory(10)
	h.Write([]byte("hello"))

	h.Release()

	if got := h.Capture(); got != nil {
		t.Fatalf("Capture() after Release() = %v, want nil", got)
	}
	if h.buf != nil {
		t.Fatal("buf should be nil after Release()")
	}
	if h.capacity != 0 {
		t.Fatalf("capacity after Release() = %d, want 0", h.capacity)
	}

	h.Write([]byte("world"))
	if got := h.Capture(); got != nil {
		t.Fatalf("Capture() after Write() on released history = %v, want nil", got)
	}
}

func TestPaneOutputHistory_WriteEmptyData(t *testing.T) {
	h := NewPaneOutputHistory(100)

	h.Write([]byte("initial"))
	h.Write([]byte{})
	h.Write([]byte{})

	got := h.Capture()
	if !bytes.Equal(got, []byte("initial")) {
		t.Errorf("Capture() after empty writes = %v, want %v", got, []byte("initial"))
	}
}

func TestPaneOutputHistory_SmallCapacity(t *testing.T) {
	h := NewPaneOutputHistory(1)

	h.Write([]byte("A"))
	if !bytes.Equal(h.Capture(), []byte("A")) {
		t.Error("capacity=1: first write failed")
	}

	h.Write([]byte("B"))
	if !bytes.Equal(h.Capture(), []byte("B")) {
		t.Error("capacity=1: second write should overwrite")
	}
}

func TestPaneOutputHistory_LargeWrite(t *testing.T) {
	capacity := 256
	h := NewPaneOutputHistory(capacity)

	// Create data larger than capacity
	largeData := make([]byte, capacity*3)
	for i := range largeData {
		largeData[i] = byte('A' + (i % 26))
	}

	h.Write(largeData)
	got := h.Capture()

	// Should contain the last `capacity` bytes
	want := largeData[len(largeData)-capacity:]
	if !bytes.Equal(got, want) {
		t.Errorf("large write: got %d bytes, want %d bytes", len(got), len(want))
	}
}

func TestPaneOutputHistory_Concurrent(t *testing.T) {
	h := NewPaneOutputHistory(1024)
	var wg sync.WaitGroup
	numGoroutines := 10
	writesPerGoroutine := 100

	// Launch multiple goroutines writing concurrently
	for i := range numGoroutines {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			data := make([]byte, 10)
			for range writesPerGoroutine {
				for k := range data {
					data[k] = byte('0' + (id % 10))
				}
				h.Write(data)
			}
		}(i)
	}

	// Launch goroutines reading concurrently
	for range numGoroutines {
		wg.Go(func() {
			for range writesPerGoroutine {
				_ = h.Capture()
			}
		})
	}

	wg.Wait()

	// Verify final state is consistent
	got := h.Capture()
	if got == nil || len(got) == 0 {
		t.Error("concurrent operations resulted in nil or empty capture")
	}
}

func TestPaneOutputHistory_ConcurrentReset(t *testing.T) {
	h := NewPaneOutputHistory(512)
	var wg sync.WaitGroup
	numGoroutines := 5

	// Launch goroutines writing, reading, and resetting
	for i := range numGoroutines {
		wg.Add(3)

		// Writer
		go func(id int) {
			defer wg.Done()
			data := []byte("data")
			for range 50 {
				h.Write(data)
			}
		}(i)

		// Reader
		go func() {
			defer wg.Done()
			for range 50 {
				_ = h.Capture()
			}
		}()

		// Resetter
		go func() {
			defer wg.Done()
			for range 10 {
				h.Reset()
			}
		}()
	}

	wg.Wait()

	// No panic or race condition indicates success
}

func TestPaneOutputHistory_NewWithDefaultCapacity(t *testing.T) {
	h := NewPaneOutputHistory(defaultPaneOutputHistoryCapacity)

	data := make([]byte, 1000)
	for i := range data {
		data[i] = 'X'
	}

	h.Write(data)
	got := h.Capture()

	if !bytes.Equal(got, data) {
		t.Error("default capacity history failed to store data")
	}
}

func TestPaneOutputHistory_WrapAroundAtBoundary(t *testing.T) {
	h := NewPaneOutputHistory(10)

	// Fill buffer to exact capacity
	h.Write([]byte("0123456789"))
	if !bytes.Equal(h.Capture(), []byte("0123456789")) {
		t.Fatal("initial fill failed")
	}

	// Write just one byte (will wrap)
	h.Write([]byte("A"))
	if !bytes.Equal(h.Capture(), []byte("123456789A")) {
		t.Error("single byte wrap failed")
	}

	// Write exactly capacity bytes (should overwrite everything)
	h.Write([]byte("BCDEFGHIJ"))
	// After writing BCDEFGHIJ (9 bytes) starting at position 1, we get "ABCDEFGHIJ"
	// with writePos=0, giving us "ABCDEFGHIJ"
	if !bytes.Equal(h.Capture(), []byte("ABCDEFGHIJ")) {
		t.Error("capacity-sized write after wrap failed")
	}
}

func TestPaneOutputHistory_SequentialWrites(t *testing.T) {
	h := NewPaneOutputHistory(20)

	h.Write([]byte("12345"))
	if !bytes.Equal(h.Capture(), []byte("12345")) {
		t.Error("step 1 failed")
	}

	h.Write([]byte("67890"))
	if !bytes.Equal(h.Capture(), []byte("1234567890")) {
		t.Error("step 2 failed")
	}

	h.Write([]byte("ABCDE"))
	if !bytes.Equal(h.Capture(), []byte("1234567890ABCDE")) {
		t.Error("step 3 failed")
	}

	h.Write([]byte("FGHIJ"))
	if !bytes.Equal(h.Capture(), []byte("1234567890ABCDEFGHIJ")) {
		t.Error("step 4 failed")
	}

	h.Write([]byte("KLMNO"))
	// Should now wrap: oldest 5 bytes removed, newest 5 added
	if !bytes.Equal(h.Capture(), []byte("67890ABCDEFGHIJKLMNO")) {
		t.Error("step 5 (wrap) failed")
	}
}
