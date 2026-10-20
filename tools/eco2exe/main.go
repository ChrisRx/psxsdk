// Command eco2exe makes a PSX-EXE of a Net Yaroze program, ECOFF or ELF,
// with the startup stub and the Net Yaroze library combined in, so that it
// boots without the Net Yaroze boot disc.
package main

import (
	"crypto/md5"
	"fmt"
	"log"

	"github.com/spf13/cobra"

	"go.chrisrx.dev/psx/internal/flags"
	"go.chrisrx.dev/psx/link"
)

var opts struct {
	Patch bool
	Loads flags.Loads
}

func NewEco2ExeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "eco2exe [flags] <input-file> <output-file>",
		Short: "convert a Net Yaroze executable to a PSX-EXE",
		Long: `eco2exe combines a Net Yaroze executable with the Net Yaroze
development library and writes the result as a PSX-EXE.

The input may be either the ECOFF produced by the SDK's own toolchain or the
ELF produced by a current mipsel-none-elf toolchain. Only the flat image and
the entry point are needed, and both formats carry those.

--load file@addr puts a file into the image at a hexadecimal address, as
siocons' "local dload" put a program's data in memory before the program
ran, so that a program written for the serial loader runs from a disc:

  eco2exe --load data1.arc@80090000 --load data2.arc@80098000 game.exe MAIN.EXE`,
		Args: cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			input, err := link.LoadImage(args[0])
			if err != nil {
				log.Fatal(err)
			}
			exe, err := link.Executable(input, opts.Patch, opts.Loads...)
			if err != nil {
				log.Fatal(err)
			}
			if err := exe.WriteFile(args[1]); err != nil {
				log.Fatal(err)
			}
			fmt.Printf("created %#v: %x\n", args[1], md5.Sum(exe.Bytes()))
		},
	}

	cmd.PersistentFlags().BoolVarP(&opts.Patch, "patch", "p", true, "patch Net Yaroze executable")
	cmd.PersistentFlags().Var(&opts.Loads, "load", "file@addr: put a file into the image at a hexadecimal address, as siocons' local dload did")
	return cmd
}

func main() {
	if err := NewEco2ExeCommand().Execute(); err != nil {
		log.Fatal(err)
	}
}
