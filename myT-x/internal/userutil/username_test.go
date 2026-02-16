package userutil

import "testing"

func TestSanitizeUsername(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "plain", input: "alice", want: "alice"},
		{name: "domain user", input: "DOMAIN\\user", want: "DOMAIN_user"},
		{name: "email", input: "user@domain.com", want: "user_domain.com"},
		{name: "empty", input: "", want: "unknown"},
		{name: "whitespace", input: "  ", want: "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SanitizeUsername(tt.input); got != tt.want {
				t.Fatalf("SanitizeUsername(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
