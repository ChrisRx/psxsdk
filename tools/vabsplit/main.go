// Command vabsplit splits a VAB sound bank into its header, the VH, which a
// program keeps in main memory, and its body, the VB, which it sends to the
// SPU: what Sony's vabsplit did.
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"go.chrisrx.dev/psx/format/vab"
)

func main() {
	cmd := &cobra.Command{
		Use:   "vabsplit <bank.vab>...",
		Short: "split VAB sound banks into VH and VB files",
		Long: `vabsplit writes each bank's header and body beside it, with the
extensions .VH and .VB.`,
		Args:         cobra.MinimumNArgs(1),
		RunE:         run,
		SilenceUsage: true,
	}
	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func run(cmd *cobra.Command, args []string) error {
	for _, path := range args {
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		n, err := vab.HeaderLen(b)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if n > len(b) {
			return fmt.Errorf("%s: the header should be %d bytes, but the file is %d", path, n, len(b))
		}
		v, err := vab.Decode(b[:n], b[n:])
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		stem := strings.TrimSuffix(path, filepath.Ext(path))
		if err := os.WriteFile(stem+".VH", b[:n], 0o644); err != nil {
			return err
		}
		if err := os.WriteFile(stem+".VB", b[n:], 0o644); err != nil {
			return err
		}
		programs := 0
		for _, p := range v.Programs {
			if p.Tones > 0 {
				programs++
			}
		}
		fmt.Printf("%s -> %s.VH, %s.VB (%d programs, %d vags)\n", path, stem, stem, programs, len(v.VAGs))
	}
	return nil
}
