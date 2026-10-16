// Command psxemu runs the emulator headless, for comparing it against psxe.
package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"image/png"
	"io/fs"
	"log"
	"os"
	"runtime/pprof"

	"github.com/spf13/cobra"

	"go.chrisrx.dev/psx/emu"
	"go.chrisrx.dev/psx/format/psx"
	"go.chrisrx.dev/psx/format/psxtrace"
)

var opts struct {
	BIOS       string
	Disc       string
	EXE        string
	Trace      string
	Skip       uint64
	Count      uint64
	CPUProfile string
	VRAM       string
	PNG        string
	Card       string
}

func main() {
	cmd := &cobra.Command{
		Use:   "psxemu [flags]",
		Short: "run the emulator headless",
		Long: `psxemu boots the BIOS headless, printing what it writes to the TTY, with
--disc in the drive if given.

--trace writes an instruction trace in the format psxe writes with
PSXE_TRACE, so the two can be compared with psxtrace diff. --skip and --count
choose which instructions, as PSXE_TRACE_SKIP and PSXE_TRACE_COUNT do:

  PSXE_TRACE=psxe.trace PSXE_TRACE_COUNT=1000000 psxe -a -b bios.bin
  psxemu --bios bios.bin --trace emu.trace --count 1000000
  psxtrace diff psxe.trace emu.trace`,
		Args:         cobra.NoArgs,
		RunE:         run,
		SilenceUsage: true,
	}
	f := cmd.Flags()
	f.StringVar(&opts.BIOS, "bios", "", "BIOS image to boot")
	f.StringVar(&opts.Disc, "disc", "", "cue sheet of a disc to put in the drive")
	f.StringVar(&opts.EXE, "exe", "", "PSX-EXE to load when the BIOS reaches its shell, as psxe -x does")
	f.StringVar(&opts.Trace, "trace", "", "write an instruction trace here")
	f.Uint64Var(&opts.Skip, "skip", 0, "instructions to run before tracing")
	f.Uint64Var(&opts.Count, "count", 0, "instructions to run, after --skip; 0 runs until interrupted")
	f.StringVar(&opts.CPUProfile, "cpuprofile", "", "write a CPU profile of the run here, for go tool pprof")
	f.StringVar(&opts.VRAM, "vram", "", "at the end, write VRAM here, as psxe does with PSXE_TRACE_VRAM")
	f.StringVar(&opts.PNG, "png", "", "at the end, write what the display shows here")
	f.StringVar(&opts.Card, "card", "", "memory card image for slot 1, made blank if it does not exist, and written back at the end")
	cmd.MarkFlagRequired("bios")

	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func run(cmd *cobra.Command, args []string) error {
	if opts.CPUProfile != "" {
		f, err := os.Create(opts.CPUProfile)
		if err != nil {
			return err
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			return err
		}
		defer pprof.StopCPUProfile()
	}

	bios, err := os.ReadFile(opts.BIOS)
	if err != nil {
		return err
	}
	m, err := emu.New(bios)
	if err != nil {
		return err
	}
	m.TTY = os.Stdout
	// psxe leaves CD audio out while tracing, so a trace to compare with
	// its has to as well.
	if opts.Trace != "" {
		m.SetCDAudio(false)
	}
	if opts.Disc != "" {
		d, err := emu.OpenDisc(opts.Disc)
		if err != nil {
			return err
		}
		defer d.Close()
		m.InsertDisc(d)
	}

	var card *emu.MemoryCard
	if opts.Card != "" {
		data, err := os.ReadFile(opts.Card)
		if errors.Is(err, fs.ErrNotExist) {
			data, err = nil, nil
		}
		if err != nil {
			return err
		}
		if card, err = emu.NewMemoryCard(data); err != nil {
			return err
		}
		m.InsertCard(1, card)
	}

	// An executable is loaded in the middle of the run, when the BIOS
	// reaches its shell, as psxe does it, so the steps before it are traced
	// the same way.
	var exe *psx.File
	if opts.EXE != "" {
		if exe, err = psx.Open(opts.EXE); err != nil {
			return err
		}
	}
	// run runs n instructions, one at a time while the executable is still
	// to be loaded, and then as many at once as it can. It returns how many
	// it ran, counting one a device could not emulate.
	run := func(n uint64) (uint64, error) {
		var ran uint64
		for ; exe != nil && ran < n; ran++ {
			if m.AtShell() {
				if err := m.LoadEXE(exe); err != nil {
					return ran, err
				}
				exe = nil
				break
			}
			if err := m.Step(); err != nil {
				return ran + 1, err
			}
		}
		more, err := m.Run(n - ran)
		return ran + more, err
	}

	if _, err := run(opts.Skip); err != nil {
		return err
	}

	var tw *psxtrace.Writer
	if opts.Trace != "" {
		f, err := os.Create(opts.Trace)
		if err != nil {
			return err
		}
		defer f.Close()
		if tw, err = psxtrace.NewWriter(f); err != nil {
			return err
		}
		m.Trace(tw)
	}

	count := opts.Count
	if count == 0 {
		count = ^uint64(0)
	}
	if ran, err := run(count); err != nil {
		return fmt.Errorf("instruction %d: %w", opts.Skip+ran-1, err)
	}

	if opts.VRAM != "" {
		vram := m.VRAM()
		buf := make([]byte, 2*len(vram))
		for i, p := range vram {
			binary.LittleEndian.PutUint16(buf[2*i:], p)
		}
		if err := os.WriteFile(opts.VRAM, buf, 0o644); err != nil {
			return err
		}
	}
	if opts.PNG != "" {
		f, err := os.Create(opts.PNG)
		if err != nil {
			return err
		}
		defer f.Close()
		if err := png.Encode(f, m.Display()); err != nil {
			return err
		}
	}
	if card != nil {
		if err := os.WriteFile(opts.Card, card.Data(), 0o644); err != nil {
			return err
		}
	}
	if tw != nil {
		return tw.Flush()
	}
	return nil
}
