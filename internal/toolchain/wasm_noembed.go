//go:build !nyc_embed

package toolchain

// Without the nyc_embed tag there is no compiler built in, and NYC_WASM
// has to name one.
var embeddedWasm, embeddedSysroot []byte
