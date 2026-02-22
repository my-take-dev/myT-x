//go:build !windows

package install

// CleanupLegacyShimInstalls is a no-op on non-Windows platforms.
func CleanupLegacyShimInstalls() error {
	return nil
}
