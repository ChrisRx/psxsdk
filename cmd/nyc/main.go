// Command nyc builds Go programs for the Net Yaroze: a PSX-EXE, or a disc
// image that boots one, from a main package, in one step.
//
//	nyc build ./mygame        # mygame.exe
//	nyc disc ./mygame         # mygame.bin and mygame.cue
//	nyc run ./mygame          # the disc, played in a window (package psxplay)
//
// A build is the steps that are tools of their own here, run in-process:
// TinyGo compiles the package for the playstation-yaroze target, the trap
// instructions LLVM leaves in that an R3000A cannot run are replaced with
// nops (link.MIPS1Traps), the ELF becomes a PSX-EXE with the Net
// Yaroze library combined in (link.Executable), and for a disc mkpsxiso
// writes the image with the files the program registers with package disc.
// Only the first step is outside nyc.
//
// TinyGo is the tinygo command when there is one, given the PlayStation
// targets nyc carries when it has none, or else TinyGo compiled to
// WebAssembly, the playground's compiler, run in-process under wazero
// (--compiler, NYC_COMPILER). Package go.chrisrx.dev/psx/internal/toolchain
// says how each is found. This nyc has no WebAssembly compiler of its own,
// and NYC_WASM has to name one. cmd/nycw is nyc with one embedded.
//
// nyc generate is discgen, for go:generate, and nyc iso is mkpsxiso with
// all of its flags.
package main

import "go.chrisrx.dev/psx/internal/nyc"

func main() { nyc.Main("nyc", "") }
