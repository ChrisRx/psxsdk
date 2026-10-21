package nyc

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"go.chrisrx.dev/psx/internal/psxplay"
	"go.chrisrx.dev/psx/internal/toolchain"
)

func runCommand() *cobra.Command {
	var f discFlags
	var play psxplay.Options
	cmd := &cobra.Command{
		Use:   "run [flags] [package]",
		Short: "build a program onto a disc and run it in a window",
		Long: `run builds the main package in the directory given, the current one if
none is, onto a disc as disc does, in a temporary directory, and runs it in
a window, as psxplay does, booting OpenBIOS, or the BIOS given with --bios.
What the program prints goes to standard output.

` + psxplay.Keys,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pkg := "."
			if len(args) == 1 {
				pkg = args[0]
			}
			name, err := pkgName(pkg)
			if err != nil {
				return err
			}
			tmp, err := os.MkdirTemp("", "nyc-run-")
			if err != nil {
				return err
			}
			defer os.RemoveAll(tmp)
			if err := f.disc(cmd.Context(), pkg, filepath.Join(tmp, name+".bin"), io.Discard); err != nil {
				return err
			}
			play.Disc = filepath.Join(tmp, name+".cue")
			play.Title = "nyc - " + name
			return psxplay.Run(play)
		},
	}
	f.register(cmd)
	play.AddFlags(cmd.Flags(), false)
	return cmd
}

func envCommand() *cobra.Command {
	var compiler string
	cmd := &cobra.Command{
		Use:   "env",
		Short: "say which compiler nyc would use, and where it is",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			w := cmd.OutOrStdout()
			c, err := toolchain.Find(compiler)
			if err != nil {
				return err
			}
			fmt.Fprintf(w, "compiler: %s\n", c.Name())
			switch c := c.(type) {
			case *toolchain.Native:
				fmt.Fprintf(w, "tinygo: %s\n", c.Path)
				fmt.Fprintf(w, "root: %s\n", c.Root)
				if c.Overlaid {
					fmt.Fprintln(w, "targets: nyc's own, laid over the tinygo command's own root")
				}
			case *toolchain.Wasm:
				fmt.Fprintf(w, "unpacked: %s\n", c.Dir())
			}
			return nil
		},
	}
	compilerFlag(cmd.Flags(), &compiler, "tinygo, wasm or auto")
	return cmd
}
