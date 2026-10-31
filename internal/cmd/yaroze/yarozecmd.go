// Package yaroze is the yaroze command, which cmd/yaroze builds with the
// compiler embedded. It is not package yaroze, the library a program is
// written against, which it builds programs with.
package yaroze

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.chrisrx.dev/x/context"

	"go.chrisrx.dev/psx/internal/cmd/discgen"
	"go.chrisrx.dev/psx/internal/cmd/mkarc"
	"go.chrisrx.dev/psx/internal/cmd/mkdisc"
	"go.chrisrx.dev/psx/internal/cmd/mkscene"
	"go.chrisrx.dev/psx/internal/cmd/mkvab"
	"go.chrisrx.dev/psx/internal/cmd/obj2tmd"
	"go.chrisrx.dev/psx/internal/cmd/png2tim"
	"go.chrisrx.dev/psx/internal/cmd/siocons"
	"go.chrisrx.dev/psx/internal/cmd/smf2seq"
	"go.chrisrx.dev/psx/internal/cmd/wav2vag"
)

// runtimeFlag adds the --runtime flag, defaulting to YAROZE_RUNTIME.
func runtimeFlag(fl *pflag.FlagSet, p *string) {
	fl.StringVar(p, "runtime", os.Getenv("YAROZE_RUNTIME"), "wazero, wasmtime or auto (YAROZE_RUNTIME): what runs the compiler; auto is the wasmtime command if there is one, which is quicker, else wazero, built in")
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
		mkdisc.Command("iso"),
		png2tim.Command("tim"),
		obj2tmd.Command("tmd"),
		wav2vag.Command("vag"),
		mkvab.Command("vab"),
		smf2seq.Command("seq"),
		mkscene.Command("scene"),
		mkarc.Command("arc"),
		siocons.Command("sio"),
	)
	if err := root.ExecuteContext(context.Shutdown()); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", name, err)
		os.Exit(1)
	}
}
