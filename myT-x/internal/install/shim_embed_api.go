//go:build windows

package install

// HasEmbeddedShim reports whether a shim binary is embedded in this build.
func HasEmbeddedShim() bool {
	return len(embeddedShimBinary) > 0
}

// GetEmbeddedShim returns the embedded shim binary bytes, or nil if not embedded.
func GetEmbeddedShim() []byte {
	return embeddedShimBinary
}
