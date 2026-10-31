// Command yaroze builds Go programs for the Net Yaroze: a PSX-EXE, or a disc
// image that boots one, from a main package, in one step, with nothing
// else installed:
//
//	go run go.chrisrx.dev/psx/cmd/yaroze@latest disc ./mygame
//
//	yaroze build ./mygame        # mygame.exe
//	yaroze disc ./mygame         # mygame.bin and mygame.cue
//	yaroze run ./mygame          # the disc, played in a window (package psxplay)
//
// A build is the steps that are tools of their own here, run in-process:
// TinyGo compiles the package for the playstation-yaroze target, the trap
// instructions LLVM leaves in that an R3000A cannot run are replaced with
// nops (link.MIPS1Traps), the ELF becomes a PSX-EXE with the Net
// Yaroze library combined in (link.Executable), and for a disc mkdisc
// writes the image with the files the program registers with package disc.
//
// TinyGo is the playground's compiler, TinyGo compiled to WebAssembly,
// embedded with its sysroot, a copy of the playground's with the cache
// filled at -opt=2 as well, in wasm/, which task yaroze writes. It runs under
// the wasmtime command when there is one, and otherwise under wazero,
// built in (--runtime, YAROZE_RUNTIME). Package
// go.chrisrx.dev/psx/internal/toolchain says how. YAROZE_WASM names another
// compiler to use in place of the embedded one, for working on it.
//
// yaroze generate is discgen, for go:generate, and yaroze iso is mkdisc with
// all of its flags. The converters are here too, each the tool of its name:
// tim is png2tim, tmd obj2tmd, vag wav2vag, vab mkvab, seq smf2seq, scene
// mkscene and arc mkarc. sio is siocons, Sony's SIOCONS for a Net Yaroze
// on the serial cable.
package main

import (
	_ "embed"

	yarozecmd "go.chrisrx.dev/psx/internal/cmd/yaroze"
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
	yarozecmd.Main("yaroze")
}
