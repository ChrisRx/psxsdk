// Package nyc is the nyc command, which cmd/nyc builds with the compiler
// embedded.
package nyc

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"go.chrisrx.dev/psx/internal/discgen"
	"go.chrisrx.dev/psx/internal/mkpsxiso"
)

// runtimeFlag adds the --runtime flag, defaulting to NYC_RUNTIME.
func runtimeFlag(fl *pflag.FlagSet, p *string) {
	fl.StringVar(p, "runtime", os.Getenv("NYC_RUNTIME"), "wazero, wasmtime or auto (NYC_RUNTIME): what runs the compiler; auto is the wasmtime command if there is one, which is quicker, else wazero, built in")
}

// Main runs the command, as name, and exits.
func Main(name string) {
	root := &cobra.Command{
		Use:   name,
		Short: "build Go programs for the Net Yaroze",
		Long: name + ` builds Go programs for the Net Yaroze: a PSX-EXE, or a disc image
that boots one, from a main package, in one step. It compiles with TinyGo
compiled to WebAssembly, built in, which needs nothing else installed, and
runs it under the wasmtime command when there is one, which is quicker.`,
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
