//go:build !windows

package install

// ShimInstallResult contains shim install details.
type ShimInstallResult struct {
	InstalledPath  string `json:"installed_path"`
	PathUpdated    bool   `json:"path_updated"`
	RestartNeeded  bool   `json:"restart_needed"`
	InstallMessage string `json:"message"`
}

// EnsureShimInstalled is a no-op on non-Windows platforms.
func EnsureShimInstalled(_ string) (ShimInstallResult, error) {
	return ShimInstallResult{
		InstallMessage: "tmux shim install is available only on Windows",
	}, nil
}

// NeedsShimInstall always returns false on non-Windows platforms.
func NeedsShimInstall() (bool, error) {
	return false, nil
}

// EnsureProcessPathContains is a no-op on non-Windows platforms.
func EnsureProcessPathContains(_ string) bool {
	return false
}

// ResolveInstallDir is not supported on non-Windows platforms.
func ResolveInstallDir() (string, error) {
	return "", nil
}
