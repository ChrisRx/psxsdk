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
// TinyGo compiles the package for its playstation target with the Net
// Yaroze's files laid over it and the library linked in (toolchain.Static),
// the trap instructions LLVM leaves in that an R3000A cannot run are
// replaced with nops (link.MIPS1Traps), the ELF becomes a PSX-EXE as it is
// (link.Standalone), and for a disc mkdisc writes the image with the files
// the program registers with package disc. With --libps resident the
// program calls the resident library at its addresses (toolchain.Resident)
// and is combined with that library's image instead (link.Executable).
//
// TinyGo is the playground's compiler, TinyGo compiled to WebAssembly,
// embedded with its sysroot, a copy of the playground's with the cache
// filled at -opt=2 as well, in wasm/, which task yaroze writes. It runs under
// the wasmtime command when there is one, and otherwise under wazero,
// built in (--runtime, YAROZE_RUNTIME). Package
// go.chrisrx.dev/psx/internal/toolchain says how. YAROZE_WASM names another
// compiler to use in place of the embedded one, for working on it.
//
// A directory of C sources with no Go in it is built as a C project from the
// SDK, by the compiler with clang, which is downloaded the first time from
// where wasm/full.json says (package go.chrisrx.dev/psx/internal/cproject).
// yaroze debug debugs a C program with GDB for WebAssembly, from the
// gdb-wasi project, downloaded the first time from where wasm/gdb.json says;
// YAROZE_GDB names a gdb.wasm on disk instead. yaroze debug --port uploads
// the clean monitor in place of Sony's, from wasm/monitor.elf.gz, which
// task yaroze builds from serial/monitor.
//
// yaroze generate, or gen, is discgen, for go:generate, and yaroze iso is
// mkdisc with all of its flags. The converters are here too, each the tool
// of its name: tim is png2tim, tmd obj2tmd, vag wav2vag, vab mkvab, seq
// smf2seq, scene mkscene and archive, or arc, mkarc. sio is siocons, Sony's
// SIOCONS for a Net Yaroze on the serial cable.
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
	//go:embed wasm/full.json
	full []byte
	//go:embed wasm/gdb.json
	gdb []byte
	//go:embed wasm/monitor.elf.gz
	monitor []byte
)

func main() {
	toolchain.Embed(compiler, sysroot)
	toolchain.EmbedFull(full)
	toolchain.EmbedGDB(gdb)
	yarozecmd.EmbedMonitor(monitor)
	yarozecmd.Main("yaroze")
}
