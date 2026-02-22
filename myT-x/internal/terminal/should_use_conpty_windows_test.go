//go:build windows

package terminal

import "testing"

func TestShouldUseConPty(t *testing.T) {
	tests := []struct {
		name    string
		disable string
		enable  string
		want    bool
	}{
		// DISABLE takes precedence
		{name: "disable=1", disable: "1", enable: "", want: false},
		{name: "disable=true", disable: "true", enable: "", want: false},
		{name: "disable=yes", disable: "yes", enable: "", want: false},
		{name: "disable=on", disable: "on", enable: "", want: false},
		{name: "disable=TRUE (case insensitive)", disable: "TRUE", enable: "", want: false},
		{name: "disable with whitespace", disable: "  1  ", enable: "", want: false},
		{name: "disable overrides enable", disable: "1", enable: "1", want: false},

		// ENABLE controls when DISABLE is not set
		{name: "enable=1", disable: "", enable: "1", want: true},
		{name: "enable=true", disable: "", enable: "true", want: true},
		{name: "enable=yes", disable: "", enable: "yes", want: true},
		{name: "enable=on", disable: "", enable: "on", want: true},
		{name: "enable=0 disables", disable: "", enable: "0", want: false},
		{name: "enable=false disables", disable: "", enable: "false", want: false},
		{name: "enable=no disables", disable: "", enable: "no", want: false},
		{name: "enable=off disables", disable: "", enable: "off", want: false},
		{name: "enable=FALSE (case insensitive)", disable: "", enable: "FALSE", want: false},
		{name: "enable with whitespace off", disable: "", enable: "  off  ", want: false},

		// Default (both empty) returns true
		{name: "both empty defaults true", disable: "", enable: "", want: true},

		// Unknown value defaults to true
		{name: "enable=unknown defaults true", disable: "", enable: "unknown", want: true},

		// DISABLE with non-truthy value does not disable
		{name: "disable=0 does not disable", disable: "0", enable: "", want: true},
		{name: "disable=false does not disable", disable: "false", enable: "", want: true},
		{name: "disable=random_string does not disable", disable: "random_string", enable: "", want: true},

		// Whitespace-only ENABLE defaults to enabled (TrimSpace -> empty -> default)
		{name: "enable=whitespace defaults true", disable: "", enable: "   ", want: true},

		// Both non-standard values default to enabled
		{name: "both non-standard defaults true", disable: "random", enable: "random", want: true},
		{name: "disable=random + enable=off", disable: "random", enable: "off", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("GO_TMUX_DISABLE_CONPTY", tc.disable)
			t.Setenv("GO_TMUX_ENABLE_CONPTY", tc.enable)

			got := shouldUseConPty()
			if got != tc.want {
				t.Errorf("shouldUseConPty() = %v, want %v (DISABLE=%q, ENABLE=%q)",
					got, tc.want, tc.disable, tc.enable)
			}
		})
	}
}
