//go:build nyc_embed

package toolchain

import _ "embed"

// The playground's compiler, gzipped, and its sysroot, which task nyc
// copies here from the site's play/ directory before building with the
// nyc_embed tag.
var (
	//go:embed wasm/tinygo.wasm.gz
	embeddedWasm []byte
	//go:embed wasm/sysroot.tar.gz
	embeddedSysroot []byte
)
