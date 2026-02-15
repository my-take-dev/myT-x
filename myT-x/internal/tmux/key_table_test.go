package tmux

import "testing"

func TestTranslateSendKeys(t *testing.T) {
	got := TranslateSendKeys([]string{"echo", "Space", "ok", "Enter"})
	want := []byte("echo ok\r")
	if string(got) != string(want) {
		t.Fatalf("TranslateSendKeys() = %q, want %q", string(got), string(want))
	}
}
