// Command ecoffcmp compares the .text of ECOFF objects against ELF objects
// assembled from ecoff2asm output, to check that the conversion is faithful.
package main

import (
	"debug/elf"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"go.chrisrx.dev/psx/format/ecoff"
)

// mask returns the bits of a relocated instruction that must still match.
//
// ECOFF keeps relocation addends in the instruction's immediate or target
// field, whereas the generated assembly names a symbol and lets the addend
// live in the ELF relocation. Those fields therefore differ by design. The
// opcode and register fields do not, and neither does any word that carries no
// relocation at all.
func mask(t ecoff.RelocationType) uint32 {
	switch t {
	case ecoff.R_JMPADDR:
		return 0xfc000000 // opcode only; the 26-bit target moves
	case ecoff.R_REFHI, ecoff.R_REFLO, ecoff.R_GPREL, ecoff.R_LITERAL:
		return 0xffff0000 // opcode and registers; the 16-bit immediate moves
	case ecoff.R_REFWORD, ecoff.R_REFHALF:
		return 0x00000000 // the whole datum is the reference
	}
	return 0xffffffff
}

type result struct {
	name       string
	words      int
	masked     int
	mismatches int
	note       string
}

func compare(m *ecoff.ArchiveMember, dir string) (*result, error) {
	base := strings.ToLower(strings.TrimSuffix(strings.TrimSuffix(m.Name, ".o"), ".O"))
	res := &result{name: m.Name}

	var text *ecoff.Section
	for _, s := range m.File.Sections {
		if s.SectionName() == ecoff.S_TEXT && s.Size > 0 {
			text = s
			break
		}
	}
	if text == nil {
		res.note = "no .text"
		return res, nil
	}

	ef, err := elf.Open(filepath.Join(dir, base+".o"))
	if err != nil {
		return nil, err
	}
	defer ef.Close()

	es := ef.Section(".text")
	if es == nil {
		return nil, fmt.Errorf("%s: assembled object has no .text", m.Name)
	}
	got, err := es.Data()
	if err != nil {
		return nil, err
	}
	want, err := text.Data()
	if err != nil {
		return nil, err
	}
	if len(got) != len(want) {
		return nil, fmt.Errorf("%s: .text is %d bytes, want %d", m.Name, len(got), len(want))
	}

	relocs := make(map[uint32]ecoff.RelocationType, len(text.Relocations))
	for _, r := range text.Relocations {
		relocs[r.Address] = r.Type
	}

	bo := m.File.TargetByteOrder()
	for off := 0; off+4 <= len(want); off += 4 {
		mk := uint32(0xffffffff)
		if t, ok := relocs[uint32(off)]; ok {
			mk = mask(t)
			res.masked++
		}
		res.words++
		a, b := bo.Uint32(want[off:])&mk, bo.Uint32(got[off:])&mk
		if a == b {
			continue
		}
		if res.mismatches < 5 {
			fmt.Printf("  %s+0x%04X: got 0x%08X want 0x%08X (mask 0x%08X)\n",
				m.Name, off, bo.Uint32(got[off:]), bo.Uint32(want[off:]), mk)
		}
		res.mismatches++
	}
	return res, nil
}

func main() {
	cmd := &cobra.Command{
		Use:   "ecoffcmp <archive> <object-dir>",
		Short: "compare ECOFF objects against assembled ecoff2asm output",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := ecoff.OpenArchive(args[0])
			if err != nil {
				return err
			}
			defer a.Close()

			var words, masked, bad, skipped int
			for _, m := range a.Objects() {
				res, err := compare(m, args[1])
				if err != nil {
					return err
				}
				if res.note != "" {
					skipped++
					continue
				}
				words += res.words
				masked += res.masked
				if res.mismatches > 0 {
					bad++
				}
			}

			fmt.Printf("\n%d words compared across %d objects (%d exact, %d relocated), %d skipped\n",
				words, len(a.Objects())-skipped, words-masked, masked, skipped)
			if bad > 0 {
				return fmt.Errorf("%d objects do not match libps.a", bad)
			}
			fmt.Println("all objects match libps.a")
			return nil
		},
	}
	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
	os.Exit(0)
}
