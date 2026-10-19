package main

import (
	"crypto/md5"
	"fmt"
	"log"

	"github.com/spf13/cobra"

	"go.chrisrx.dev/psx/binutils"
	"go.chrisrx.dev/psx/format/psx"
	"go.chrisrx.dev/psx/yaroze"
)

var opts struct {
	Patch bool
	Loads []string
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
			input, err := binutils.LoadImage(args[0])
			if err != nil {
				log.Fatal(err)
			}

			exe := &psx.File{
				FileHeader: psx.FileHeader{
					Magic:     psx.ExecutableSignature,
					PC0:       input.Entry,
					TextAddr:  input.Addr,
					TextSize:  input.Size(),
					StackAddr: 0x801fff00,
				},
				Sections: []*psx.Section{
					&psx.Section{
						Name: "text",
						Addr: input.Addr,
						Data: input.Data,
					},
				},
			}

			if opts.Patch {
				if err := yaroze.PatchExecutable(exe); err != nil {
					log.Fatal(err)
				}
			}

			psx.AlignTextData(exe, 2048)

			exe, err = yaroze.Combine(exe)
			if err != nil {
				log.Fatal(err)
			}
			for _, spec := range opts.Loads {
				l, err := binutils.ParseLoad(spec)
				if err != nil {
					log.Fatal(err)
				}
				if err := binutils.Embed(exe, l.Addr, l.Data); err != nil {
					log.Fatal(err)
				}
			}

			if err := exe.WriteFile(args[1]); err != nil {
				log.Fatal(err)
			}
			fmt.Printf("created %#v: %x\n", args[1], md5.Sum(exe.Bytes()))
		},
	}

	cmd.PersistentFlags().BoolVarP(&opts.Patch, "patch", "p", true, "patch Net Yaroze executable")
	cmd.PersistentFlags().StringArrayVar(&opts.Loads, "load", nil, "file@addr: put a file into the image at a hexadecimal address, as siocons' local dload did")
	return cmd
}

func main() {
	if err := NewEco2ExeCommand().Execute(); err != nil {
		log.Fatal(err)
	}
}
