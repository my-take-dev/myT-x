package git

import "testing"

func TestIsLockFileConflict(t *testing.T) {
	tests := []struct {
		name   string
		errMsg string
		want   bool
	}{
		{
			name:   "index.lock message",
			errMsg: "fatal: Unable to create '/repo/.git/index.lock': File exists.",
			want:   true,
		},
		{
			name:   "index.lock substring only",
			errMsg: "Another git process seems to be running; index.lock exists",
			want:   true,
		},
		{
			name:   "Unable to create + File exists without index.lock",
			errMsg: "fatal: Unable to create '/repo/.git/shallow.lock': File exists",
			want:   true,
		},
		{
			name:   "unrelated error",
			errMsg: "fatal: not a git repository",
			want:   false,
		},
		{
			name:   "empty string",
			errMsg: "",
			want:   false,
		},
		{
			name:   "Unable to create without File exists",
			errMsg: "Unable to create directory: permission denied",
			want:   false,
		},
		{
			name:   "File exists without Unable to create",
			errMsg: "File exists: /tmp/something",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isLockFileConflict(tt.errMsg); got != tt.want {
				t.Fatalf("isLockFileConflict(%q) = %v, want %v", tt.errMsg, got, tt.want)
			}
		})
	}
}
