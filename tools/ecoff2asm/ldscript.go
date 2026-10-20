package main

import (
	"cmp"
	"fmt"
	"io"
	"slices"

	"go.chrisrx.dev/psx/format/ecoff"
)

// An AbsoluteSymbol is a symbol defined at a fixed address rather than in any
// section.
type AbsoluteSymbol struct {
	Name    string
	Address uint32
}

// AbsoluteSymbols returns every absolute symbol defined by an ECOFF object,
// sorted by address.
//
// The Net Yaroze libps.a declares its entire API this way. Its symusr.o member
// has zero bytes of .text, .data and .bss and consists only of absolute symbol
// definitions naming entry points in the libps.exe that the boot disk leaves
// resident in RAM. There is no code in it to port, only addresses to restate.
func AbsoluteSymbols(f *ecoff.File) []AbsoluteSymbol {
	seen := make(map[string]bool)
	var syms []AbsoluteSymbol
	for _, s := range f.ExternalSymbols {
		if ecoff.StorageClass(s.StorageClass) != ecoff.SC_ABS || s.Name == "" || seen[s.Name] {
			continue
		}
		seen[s.Name] = true
		syms = append(syms, AbsoluteSymbol{Name: s.Name, Address: s.Value})
	}
	slices.SortFunc(syms, func(a, b AbsoluteSymbol) int {
		if n := cmp.Compare(a.Address, b.Address); n != 0 {
			return n
		}
		return cmp.Compare(a.Name, b.Name)
	})
	return syms
}

// WriteSymbolDefinitions writes absolute symbol definitions as a fragment of a
// GNU ld script.
//
// PROVIDE is used so that a program redefining one of these names wins instead
// of failing the link with a duplicate definition.
func WriteSymbolDefinitions(w io.Writer, syms []AbsoluteSymbol) error {
	for _, s := range syms {
		if _, err := fmt.Fprintf(w, "  PROVIDE(%-24s = 0x%08X);\n", s.Name, s.Address); err != nil {
			return err
		}
	}
	return nil
}
