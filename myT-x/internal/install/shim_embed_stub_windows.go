//go:build windows && !embed_shim

package install

// embeddedShimBinary is nil when the embed_shim build tag is not set.
// In dev mode, the shim is resolved from the file system instead.
var embeddedShimBinary []byte
