package tmux

import (
	"testing"
)

func TestBufferStore_SetAndGet(t *testing.T) {
	tests := []struct {
		name       string
		bufName    string
		data       string
		appendMode bool
		wantData   string
		wantOK     bool
	}{
		{
			name:     "set named buffer",
			bufName:  "buf0",
			data:     "hello",
			wantData: "hello",
			wantOK:   true,
		},
		{
			name:     "set auto-named buffer",
			bufName:  "",
			data:     "auto",
			wantData: "auto",
			wantOK:   true,
		},
		{
			name:     "get nonexistent buffer",
			bufName:  "nonexistent",
			wantData: "",
			wantOK:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bs := NewBufferStore()
			if tt.name == "get nonexistent buffer" {
				_, ok := bs.Get(tt.bufName)
				if ok != tt.wantOK {
					t.Fatalf("Get(%q) ok = %v, want %v", tt.bufName, ok, tt.wantOK)
				}
				return
			}
			bs.Set(tt.bufName, []byte(tt.data), tt.appendMode)

			lookupName := tt.bufName
			if lookupName == "" {
				lookupName = "buffer0000"
			}
			buf, ok := bs.Get(lookupName)
			if ok != tt.wantOK {
				t.Fatalf("Get(%q) ok = %v, want %v", lookupName, ok, tt.wantOK)
			}
			if ok && string(buf.Data) != tt.wantData {
				t.Fatalf("Get(%q).Data = %q, want %q", lookupName, buf.Data, tt.wantData)
			}
		})
	}
}

func TestBufferStore_Append(t *testing.T) {
	bs := NewBufferStore()
	bs.Set("buf", []byte("hello"), false)
	bs.Set("buf", []byte(" world"), true)

	buf, ok := bs.Get("buf")
	if !ok {
		t.Fatal("buffer not found after append")
	}
	if string(buf.Data) != "hello world" {
		t.Fatalf("append: got %q, want %q", buf.Data, "hello world")
	}
}

func TestBufferStore_Overwrite(t *testing.T) {
	bs := NewBufferStore()
	bs.Set("buf", []byte("old"), false)
	bs.Set("buf", []byte("new"), false)

	buf, ok := bs.Get("buf")
	if !ok {
		t.Fatal("buffer not found after overwrite")
	}
	if string(buf.Data) != "new" {
		t.Fatalf("overwrite: got %q, want %q", buf.Data, "new")
	}
}

func TestBufferStore_Latest(t *testing.T) {
	bs := NewBufferStore()

	// Empty store.
	_, ok := bs.Latest()
	if ok {
		t.Fatal("Latest() on empty store should return false")
	}

	bs.Set("first", []byte("1"), false)
	bs.Set("second", []byte("2"), false)

	buf, ok := bs.Latest()
	if !ok {
		t.Fatal("Latest() should return true")
	}
	if buf.Name != "second" {
		t.Fatalf("Latest().Name = %q, want %q", buf.Name, "second")
	}
}

func TestBufferStore_Delete(t *testing.T) {
	bs := NewBufferStore()
	bs.Set("buf", []byte("data"), false)

	if !bs.Delete("buf") {
		t.Fatal("Delete should return true for existing buffer")
	}
	if bs.Delete("buf") {
		t.Fatal("Delete should return false for already-deleted buffer")
	}
	if _, ok := bs.Get("buf"); ok {
		t.Fatal("Get should return false after Delete")
	}
}

func TestBufferStore_List(t *testing.T) {
	bs := NewBufferStore()
	bs.Set("a", []byte("1"), false)
	bs.Set("b", []byte("2"), false)
	bs.Set("c", []byte("3"), false)

	list := bs.List()
	if len(list) != 3 {
		t.Fatalf("List() len = %d, want 3", len(list))
	}
	// Newest first.
	if list[0].Name != "c" {
		t.Fatalf("List()[0].Name = %q, want %q", list[0].Name, "c")
	}
}

func TestBufferStore_Rename(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*BufferStore)
		oldName string
		newName string
		wantErr bool
	}{
		{
			name: "rename existing buffer",
			setup: func(bs *BufferStore) {
				bs.Set("old", []byte("data"), false)
			},
			oldName: "old",
			newName: "new",
			wantErr: false,
		},
		{
			name:    "rename nonexistent buffer",
			setup:   func(bs *BufferStore) {},
			oldName: "missing",
			newName: "new",
			wantErr: true,
		},
		{
			name: "rename to existing name",
			setup: func(bs *BufferStore) {
				bs.Set("a", []byte("1"), false)
				bs.Set("b", []byte("2"), false)
			},
			oldName: "a",
			newName: "b",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bs := NewBufferStore()
			tt.setup(bs)
			err := bs.Rename(tt.oldName, tt.newName)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Rename(%q, %q) error = %v, wantErr %v", tt.oldName, tt.newName, err, tt.wantErr)
			}
			if err == nil {
				if _, ok := bs.Get(tt.oldName); ok {
					t.Fatal("old name should not exist after rename")
				}
				if _, ok := bs.Get(tt.newName); !ok {
					t.Fatal("new name should exist after rename")
				}
			}
		})
	}
}

func TestBufferStore_EvictionAtMaxCount(t *testing.T) {
	bs := NewBufferStore()
	for i := range maxBufferCount + 5 {
		bs.Set("", []byte("data"), false)
		_ = i
	}
	list := bs.List()
	if len(list) != maxBufferCount {
		t.Fatalf("List() len = %d after eviction, want %d", len(list), maxBufferCount)
	}
}

func TestBufferStore_GetReturnsCopy(t *testing.T) {
	bs := NewBufferStore()
	bs.Set("buf", []byte("original"), false)

	buf, _ := bs.Get("buf")
	buf.Data[0] = 'X'

	buf2, _ := bs.Get("buf")
	if string(buf2.Data) != "original" {
		t.Fatalf("Get should return a copy; internal data was mutated to %q", buf2.Data)
	}
}
