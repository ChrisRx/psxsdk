package main

import (
	"fmt"
	"log"

	"github.com/ChrisRx/psxsdk/pkg/format/ecoff"
	"github.com/mewmew/mips"
	"github.com/spf13/cobra"
)

var opts struct {
	Disassemble bool
}

func NewObjdumpCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "objdump [flags] <file>",
		Args: cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			f, err := ecoff.Open(args[0])
			if err != nil {
				log.Fatal(err)
			}
			defer f.Close()

			fmt.Printf("%+v\n\nSections:\n", f)
			for i, s := range f.Sections {
				fmt.Printf("%2d %+v\n", i, s)
			}
			fmt.Print("\n")

			fmt.Print("Symbols:\n")
			i := 0
			for _, s := range f.ExternalSymbols {
				fmt.Printf("[%3d] e %+v\n", i, s)
				i++
			}
			for _, s := range f.LocalSymbols {
				fmt.Printf("[%3d] l %+v\n", i, s)
				i++
			}
			fmt.Print("\n")

			if opts.Disassemble {
				data := f.Data()
				procs := f.SymbolsByType(ecoff.ST_PROC)
				for i := 0; i < len(data); i += 4 {
					addr := f.Entry + uint32(i)
					if s, ok := procs[addr]; ok {
						fmt.Printf("%s:\n", s.Name)
					}
					inst, err := mips.Decode(data[i:])
					if err != nil {
						log.Printf("error decoding addr 0x%08X; %v", addr, err)
						continue
					}
					fmt.Printf("\t%s\n", inst)
				}
			}
		},
	}

	cmd.PersistentFlags().BoolVarP(&opts.Disassemble, "disassemble", "d", false, "disassemble executable sections")
	return cmd
}

func main() {
	if err := NewObjdumpCommand().Execute(); err != nil {
		log.Fatal(err)
	}
}
