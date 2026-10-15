// Command psxtrace prints and compares instruction traces.
package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/mewmew/mips"
	"github.com/spf13/cobra"

	"go.chrisrx.dev/psx/format/psxtrace"
)

func main() {
	cmd := &cobra.Command{
		Use:   "psxtrace",
		Short: "print and compare instruction traces",
		Long: `psxtrace prints and compares instruction traces, which record what an
emulated R3000A did one instruction at a time.

psxe writes one when PSXE_TRACE names a file, with PSXE_TRACE_SKIP and
PSXE_TRACE_COUNT choosing which instructions:

  PSXE_TRACE=boot.trace PSXE_TRACE_COUNT=1000000 psxe -a -b bios.bin game.cue`,
		SilenceUsage: true,
	}
	cmd.AddCommand(newDumpCommand(), newDiffCommand(), newVRAMCommand())
	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func newDumpCommand() *cobra.Command {
	var skip, count uint64
	cmd := &cobra.Command{
		Use:   "dump [flags] <trace>",
		Short: "print a trace as text",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, closer, err := open(args[0])
			if err != nil {
				return err
			}
			defer closer.Close()

			w := bufio.NewWriter(os.Stdout)
			defer w.Flush()
			for i := uint64(0); count == 0 || i < skip+count; i++ {
				rec, err := r.Next()
				if err == io.EOF {
					return nil
				}
				if err != nil {
					return fmt.Errorf("record %d: %w", i, err)
				}
				if i >= skip {
					fmt.Fprintf(w, "%10d  %s\n", i, format(rec))
				}
			}
			return nil
		},
	}
	cmd.Flags().Uint64Var(&skip, "skip", 0, "records to skip before printing")
	cmd.Flags().Uint64Var(&count, "count", 0, "records to print; 0 for all")
	return cmd
}

func newDiffCommand() *cobra.Command {
	var opts psxtrace.CompareOptions
	cmd := &cobra.Command{
		Use:   "diff [flags] <trace-a> <trace-b>",
		Short: "report the first record two traces disagree on",
		Long: `diff reads two traces in step and reports the first record they disagree
on, along with the matching records before it. It exits 1 if they diverge.

Comparison stops at the end of the shorter trace, which is normally just where
one of them was cut off.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			a, ca, err := open(args[0])
			if err != nil {
				return err
			}
			defer ca.Close()
			b, cb, err := open(args[1])
			if err != nil {
				return err
			}
			defer cb.Close()

			n, d, err := psxtrace.Compare(a, b, opts)
			if err != nil {
				return err
			}
			if d == nil {
				fmt.Printf("traces match for %d records\n", n)
				return nil
			}

			fmt.Printf("%s\n\n", d)
			for i, rec := range d.Context {
				fmt.Printf("  %10d  %s\n", d.Index-uint64(len(d.Context)-i), format(rec))
			}
			fmt.Printf("a %10d  %s\n", d.Index, format(d.A))
			fmt.Printf("b %10d  %s\n", d.Index, format(d.B))
			os.Exit(1)
			return nil
		},
	}
	cmd.Flags().BoolVar(&opts.IgnoreCycles, "ignore-cycles", false, "leave cycle counts out of the comparison")
	cmd.Flags().IntVarP(&opts.Context, "context", "C", 10, "matching records to show before the divergence")
	return cmd
}

func open(path string) (*psxtrace.Reader, io.Closer, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	r, err := psxtrace.NewReader(f)
	if err != nil {
		f.Close()
		return nil, nil, fmt.Errorf("%s: %w", path, err)
	}
	return r, f, nil
}

// excodes names the R3000A exception codes.
var excodes = map[uint8]string{
	0x00: "interrupt",
	0x04: "address error on load",
	0x05: "address error on store",
	0x06: "bus error on fetch",
	0x07: "bus error on data",
	0x08: "syscall",
	0x09: "break",
	0x0a: "reserved instruction",
	0x0b: "coprocessor unusable",
	0x0c: "overflow",
}

// format prints a record on one line: where, what, how long, what changed.
func format(r psxtrace.Record) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%08x: %08x  %-28s %3d", r.PC, r.Insn, disassemble(r.Insn), r.Cycles)
	for _, w := range r.Writes {
		fmt.Fprintf(&sb, "  %s=%08x", w.Reg, w.Value)
	}
	if r.Excode != psxtrace.NoException {
		name, ok := excodes[r.Excode]
		if !ok {
			name = "unknown"
		}
		fmt.Fprintf(&sb, "  !exception %#02x (%s)", r.Excode, name)
	}
	return sb.String()
}

func disassemble(insn uint32) string {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], insn)
	inst, err := mips.Decode(b[:])
	if err != nil {
		return "?"
	}
	return inst.String()
}
