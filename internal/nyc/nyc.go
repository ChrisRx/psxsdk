// Package nyc is the nyc command, which cmd/nyc builds as it is and
// cmd/nycw builds with the WebAssembly compiler embedded.
package nyc

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"go.chrisrx.dev/psx/internal/discgen"
	"go.chrisrx.dev/psx/internal/mkpsxiso"
)

// fixedCompiler is the compiler Main was given, which every build uses
// when it is not empty, with no --compiler flag to choose another.
var fixedCompiler string

// compilerFlag adds the --compiler flag, defaulting to NYC_COMPILER, or,
// when Main was given a compiler, sets p to it.
func compilerFlag(fl *pflag.FlagSet, p *string, usage string) {
	if fixedCompiler != "" {
		*p = fixedCompiler
		return
	}
	fl.StringVar(p, "compiler", os.Getenv("NYC_COMPILER"), usage)
}

// Main runs the command, as name, and exits. An empty compiler lets
// --compiler and NYC_COMPILER choose one. Otherwise every build uses the
// compiler named, as toolchain.Find names them.
func Main(name, compiler string) {
	fixedCompiler = compiler
	long := name + ` builds Go programs for the Net Yaroze: a PSX-EXE, or a disc image
that boots one, from a main package, in one step. It compiles with the
tinygo command when there is one, and otherwise with TinyGo compiled to
WebAssembly, which needs nothing else installed.`
	if compiler == "wasm" {
		long = name + ` builds Go programs for the Net Yaroze: a PSX-EXE, or a disc image
that boots one, from a main package, in one step. It compiles with TinyGo
compiled to WebAssembly, built in, which needs nothing else installed.`
	}
	root := &cobra.Command{
		Use:           name,
		Short:         "build Go programs for the Net Yaroze",
		Long:          long,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(
		buildCommand(),
		discCommand(),
		runCommand(),
		patchCommand(),
		envCommand(),
		discgen.Command("generate"),
		mkpsxiso.Command("iso"),
	)
	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", name, err)
		os.Exit(1)
	}
}
