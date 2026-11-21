// Command ecoff2elf writes an ELF with DWARF for a Net Yaroze program built
// as ECOFF, such as by GCC 2.95.3 for mipsel-ecoff, so that current
// debuggers read its symbols: functions with their sizes, the resident
// library's entry points, source lines and frames (see package
// internal/ecoffelf). The program itself is unchanged; load the ELF in the
// debugger and run the original.
package main

import (
	"log"
	"os"

	"github.com/spf13/cobra"

	"go.chrisrx.dev/psx/format/ecoff"
	"go.chrisrx.dev/psx/internal/ecoffelf"
)

func main() {
	cmd := &cobra.Command{
		Use:   "ecoff2elf <input-ecoff> <output-elf>",
		Short: "write an ELF with DWARF for an ECOFF Net Yaroze program, for a debugger",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			f, err := ecoff.Open(args[0])
			if err != nil {
				return err
			}
			defer f.Close()
			data, err := ecoffelf.Convert(f)
			if err != nil {
				return err
			}
			return os.WriteFile(args[1], data, 0o644)
		},
	}
	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
