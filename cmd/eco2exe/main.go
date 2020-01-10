package main

import (
	"crypto/md5"
	"fmt"
	"log"

	"github.com/ChrisRx/psxsdk/pkg/format/ecoff"
	"github.com/ChrisRx/psxsdk/pkg/format/psx"
	"github.com/ChrisRx/psxsdk/pkg/yaroze"
	"github.com/spf13/cobra"
)

var opts struct {
	Patch bool
}

func NewEco2ExeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "eco2exe [flags] <input-file> <output-file>",
		Args: cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			input, err := ecoff.Open(args[0])
			if err != nil {
				log.Fatal(err)
			}

			exe := &psx.File{
				FileHeader: psx.FileHeader{
					Magic:     psx.ExecutableSignature,
					PC0:       input.Entry,
					TextAddr:  input.Entry,
					TextSize:  input.Size(),
					StackAddr: 0x801fff00,
				},
				Sections: []*psx.Section{
					&psx.Section{
						Name: "text",
						Addr: 0x80010000,
						Data: input.Data(),
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

			if err := exe.WriteFile(args[1]); err != nil {
				log.Fatal(err)
			}
			fmt.Printf("created %#v: %x\n", args[1], md5.Sum(exe.Bytes()))
		},
	}

	cmd.PersistentFlags().BoolVarP(&opts.Patch, "patch", "p", true, "patch Net Yaroze executable")
	return cmd
}

func main() {
	if err := NewEco2ExeCommand().Execute(); err != nil {
		log.Fatal(err)
	}
}
