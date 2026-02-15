//go:build !windows

package install

// HasEmbeddedShim always returns false on non-Windows platforms.
func HasEmbeddedShim() bool { return false }

// GetEmbeddedShim always returns nil on non-Windows platforms.
func GetEmbeddedShim() []byte { return nil }
