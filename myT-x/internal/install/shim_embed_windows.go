//go:build windows && embed_shim

package install

import _ "embed"

//go:embed embedded/shimbin/tmux-shim.exe
var embeddedShimBinary []byte
