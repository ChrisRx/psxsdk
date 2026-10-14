// Command elf2exe writes a linked program out as a PSX-EXE.
//
// This is the plain conversion, for a program that is the whole of the
// machine: the image and the entry point go into the header and nothing is
// added. Compare eco2exe, which is for a Net Yaroze and combines the program
// with the development library the boot disc leaves resident in RAM.
//
// The header is 2048 bytes, so the image begins on the second sector of a
// disc and the console's loader can read it a sector at a time. The BIOS
// reads text to t_addr, sets $sp from s_addr and jumps to pc0; nothing else
// in the header is consulted on a retail console.
package main

import (
	"crypto/md5"
	"fmt"
	"log"

	"github.com/spf13/cobra"

	"go.chrisrx.dev/psx/binutils"
	"go.chrisrx.dev/psx/format/psx"
)

var opts struct {
	StackAddr uint32
	StackSize uint32
	GP0       uint32
}

func NewELF2ExeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "elf2exe [flags] <input-file> <output-file>",
		Short: "write a linked program as a PSX-EXE",
		Long: `elf2exe writes a linked program out as a PSX-EXE, taking the flat
image and the entry point from the input and adding nothing.

The input may be an ELF or an ECOFF; only the loadable segments and the entry
point are needed, and both formats carry those. For a Net Yaroze program,
which has to be combined with the resident development library, use eco2exe
instead.`,
		Args:         cobra.ExactArgs(2),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			image, err := binutils.LoadImage(args[0])
			if err != nil {
				return err
			}

			exe := &psx.File{
				FileHeader: psx.FileHeader{
					Magic:     psx.ExecutableSignature,
					PC0:       image.Entry,
					GP0:       opts.GP0,
					TextAddr:  image.Addr,
					TextSize:  image.Size(),
					StackAddr: opts.StackAddr,
					StackSize: opts.StackSize,
				},
				Sections: []*psx.Section{
					{
						Name: "text",
						Addr: image.Addr,
						Data: image.Data,
					},
				},
			}

			// The loader reads whole sectors, so the image is padded to a
			// multiple of one.
			psx.AlignTextData(exe, 2048)

			if err := exe.WriteFile(args[1]); err != nil {
				return err
			}
			fmt.Printf("created %#v: load=%#08x entry=%#08x %d bytes, %x\n",
				args[1], exe.TextAddr, exe.PC0, exe.TextSize, md5.Sum(exe.Bytes()))
			return nil
		},
	}

	f := cmd.Flags()
	// The BIOS sets $sp from these before jumping to the entry point. A
	// program whose crt0 sets its own stack pointer does not depend on them,
	// but a loader that honours them should be told something sane.
	f.Uint32Var(&opts.StackAddr, "stack-addr", 0x801ffff0, "initial stack pointer")
	f.Uint32Var(&opts.StackSize, "stack-size", 0, "stack size recorded in the header")
	f.Uint32Var(&opts.GP0, "gp", 0, "initial value of $gp")
	return cmd
}

func main() {
	if err := NewELF2ExeCommand().Execute(); err != nil {
		log.Fatal(err)
	}
}
