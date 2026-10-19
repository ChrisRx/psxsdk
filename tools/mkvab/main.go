// Command mkvab builds a VAB sound bank from a definition file and VAG
// files, or reads a bank back into a definition file: what Sony's mkvab
// did, with vabsplit's work as an option.
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"go.chrisrx.dev/psx/format/vab"
	"go.chrisrx.dev/psx/format/vag"
)

var opts struct {
	Def     string
	Output  string
	Reverse string
	Split   bool
}

func main() {
	cmd := &cobra.Command{
		Use:   "mkvab -f DEF -o OUT.VAB VAG...\n  mkvab -r BANK [-o DEF]",
		Short: "build a VAB sound bank from a definition file and VAGs",
		Long: `mkvab builds a sound bank from a definition file, which gives the bank's
programs and their tones, and VAG files, which tone definitions name as vag
1, 2 and so on in the order given. -o names the bank, a .VAB of the VH and
the VB together; --split writes OUT.VH and OUT.VB instead, as vabsplit
would, which is what a program loads.

The definition file is Sony's: a VabHdr section, a ProgAtr P section for
each program, and a VagAtr P T (or ToneAtr P T) section for each tone, of
"label = value" lines. The bank's counts and sizes are worked out from what
is there. A tone's envelope is given as Sony's rates, ar 0-127, dr 0-15, sr
-127-127, rr 0-31 and sl 0-15, a bigger number faster; sr holds at 0.

  VabHdr
          ver = 0x7
  ProgAtr 0
          tones = 1
          mvol = 127
          mpan = 64
  VagAtr 0 0
          vol = 127
          pan = 64
          center = 60
          min = 0
          max = 127
          ar = 127
          dr = 0
          sr = 0
          rr = 20
          sl = 15
          prog = 0
          vag = 1

-r reads a bank, a .VAB or a .VH (with its .VB beside it, for the sizes),
and prints its definition file, or writes it to -o.`,
		Args:         cobra.ArbitraryArgs,
		RunE:         run,
		SilenceUsage: true,
	}
	f := cmd.Flags()
	f.StringVarP(&opts.Def, "def", "f", "", "the definition file")
	f.StringVarP(&opts.Output, "output", "o", "", "the bank to write, or with -r the definition file")
	f.StringVarP(&opts.Reverse, "reverse", "r", "", "read this bank and write its definition file")
	f.BoolVar(&opts.Split, "split", false, "write OUT.VH and OUT.VB rather than OUT.VAB")

	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func run(cmd *cobra.Command, args []string) error {
	if opts.Reverse != "" {
		return reverse()
	}
	if opts.Def == "" || opts.Output == "" || len(args) == 0 {
		return fmt.Errorf("give a definition file with -f, the bank with -o and the VAG files")
	}
	text, err := os.ReadFile(opts.Def)
	if err != nil {
		return err
	}
	v, err := vab.ParseDef(string(text))
	if err != nil {
		return fmt.Errorf("%s: %w", opts.Def, err)
	}
	if n := v.MaxVAG(); n > len(args) {
		return fmt.Errorf("%s names vag %d, but %d VAG files are given", opts.Def, n, len(args))
	}
	if len(args) > vab.MaxVAGs {
		return fmt.Errorf("%d VAG files, more than a bank holds, %d", len(args), vab.MaxVAGs)
	}
	for _, path := range args {
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		g, err := vag.Decode(b)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		v.VAGs = append(v.VAGs, g.Data)
	}
	if opts.Split {
		stem := strings.TrimSuffix(opts.Output, filepath.Ext(opts.Output))
		if err := os.WriteFile(stem+".VH", v.VH(), 0o644); err != nil {
			return err
		}
		if err := os.WriteFile(stem+".VB", v.VB(), 0o644); err != nil {
			return err
		}
		fmt.Printf("created %q and %q: %s\n", stem+".VH", stem+".VB", summary(v))
		return nil
	}
	if err := os.WriteFile(opts.Output, v.Bytes(), 0o644); err != nil {
		return err
	}
	fmt.Printf("created %q: %s\n", opts.Output, summary(v))
	return nil
}

func summary(v *vab.VAB) string {
	programs := 0
	for _, p := range v.Programs {
		if p.Tones > 0 {
			programs++
		}
	}
	return fmt.Sprintf("%d programs, %d VAGs, %d bytes", programs, len(v.VAGs), v.Size())
}

func reverse() error {
	b, err := os.ReadFile(opts.Reverse)
	if err != nil {
		return err
	}
	n, err := vab.HeaderLen(b)
	if err != nil {
		return fmt.Errorf("%s: %w", opts.Reverse, err)
	}
	vh, vb := b[:min(n, len(b))], b[min(n, len(b)):]
	if len(vb) == 0 {
		// A VH alone: the VB beside it gives the VAGs' sizes.
		stem := strings.TrimSuffix(opts.Reverse, filepath.Ext(opts.Reverse))
		for _, ext := range []string{".VB", ".vb", ".Vb"} {
			if body, err := os.ReadFile(stem + ext); err == nil {
				vb = body
				break
			}
		}
	}
	if len(vb) == 0 {
		vb = nil
	}
	v, err := vab.Decode(vh, vb)
	if err != nil {
		return fmt.Errorf("%s: %w", opts.Reverse, err)
	}
	if opts.Output == "" {
		fmt.Print(v.Def())
		return nil
	}
	return os.WriteFile(opts.Output, []byte(v.Def()), 0o644)
}
