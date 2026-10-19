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
// nops (binutils.MIPS1Traps), the ELF becomes a PSX-EXE with the Net
// Yaroze library combined in (yaroze.Executable), and for a disc mkpsxiso
// writes the image with the files the program registers with package disc.
// Only the first step is outside nyc.
//
// TinyGo is the tinygo command when there is one, given the PlayStation
// targets from the fork when it has none, or else TinyGo compiled to
// WebAssembly, the playground's compiler, run in-process under wazero
// (--compiler, NYC_COMPILER). Package go.chrisrx.dev/psx/internal/toolchain
// says how each is found.
//
// nyc generate is discgen, for go:generate, and nyc iso is mkpsxiso with
// all of its flags.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"go.chrisrx.dev/psx/internal/discgen"
	"go.chrisrx.dev/psx/internal/mkpsxiso"
)

func main() {
	root := &cobra.Command{
		Use:   "nyc",
		Short: "build Go programs for the Net Yaroze",
		Long: `nyc builds Go programs for the Net Yaroze: a PSX-EXE, or a disc image
that boots one, from a main package, in one step. It compiles with the
tinygo command when there is one, and otherwise with TinyGo compiled to
WebAssembly, which needs nothing else installed.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(
		buildCommand(),
		discCommand(),
		runCommand(),
		envCommand(),
		discgen.Command("generate"),
		mkpsxiso.Command("iso"),
	)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "nyc:", err)
		os.Exit(1)
	}
}
