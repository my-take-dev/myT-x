//go:build !windows

package hotkeys

import "fmt"

// ParseBinding is currently implemented only for Windows in this project.
func ParseBinding(spec string) (Binding, error) {
	return Binding{}, fmt.Errorf("global hotkeys are currently supported only on Windows")
}
