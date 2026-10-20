// Command nycw is nyc with TinyGo compiled to WebAssembly embedded, so
// that it builds programs with nothing else installed:
//
//	go run go.chrisrx.dev/psx/cmd/nycw@latest disc ./mygame
//
// Its commands and flags are nyc's, less --compiler: it compiles only
// with the WebAssembly compiler, even when there is a tinygo command.
//
// The compiler and its sysroot, a copy of the playground's with the cache
// filled at -opt=2 as well, are in wasm/, which task nyc writes.
package main

import (
	_ "embed"

	"go.chrisrx.dev/psx/internal/nyc"
	"go.chrisrx.dev/psx/internal/toolchain"
)

var (
	//go:embed wasm/tinygo.wasm.gz
	compiler []byte
	//go:embed wasm/sysroot.tar.gz
	sysroot []byte
)

func main() {
	toolchain.Embed(compiler, sysroot)
	nyc.Main("nycw", "wasm")
}
